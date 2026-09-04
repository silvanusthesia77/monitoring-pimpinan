package main

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"time"
)

const agendaSelect = `
	SELECT a.id, a.title, a.location, a.start_at, a.end_at, a.organizer, a.staff_note,
	       a.status, COALESCE(a.delegate, ''), COALESCE(a.leader_note, ''),
	       a.validated_at, a.pulled_back_at, COALESCE(a.report_note, ''), a.created_at,
	       fi.id, fi.original_name, fi.mime_type, fi.size_bytes, fi.created_at,
	       fd.id, fd.original_name, fd.mime_type, fd.size_bytes, fd.created_at
	FROM agendas a
	LEFT JOIN files fi ON fi.id = a.invitation_file_id
	LEFT JOIN files fd ON fd.id = a.documentation_file_id`

func (app *App) agendas(w http.ResponseWriter, r *http.Request) {
	user, ok := app.currentUser(r)
	if !ok {
		writeError(w, http.StatusUnauthorized, "Belum login")
		return
	}

	switch r.Method {
	case http.MethodGet:
		app.listAgendas(w)
	case http.MethodPost:
		if user.Role != roleStaff {
			forbidden(w)
			return
		}
		app.createAgenda(w, r, user)
	default:
		methodNotAllowed(w)
	}
}

func (app *App) agendaAction(w http.ResponseWriter, r *http.Request) {
	user, ok := app.currentUser(r)
	if !ok {
		writeError(w, http.StatusUnauthorized, "Belum login")
		return
	}

	id, action, ok := parseAgendaAction(w, r.URL.Path)
	if !ok {
		http.NotFound(w, r)
		return
	}

	switch action {
	case "validation":
		if user.Role != roleLeader {
			forbidden(w)
			return
		}
		app.validateAgenda(w, r, id, user)
	case "pullback":
		if user.Role != roleLeader {
			forbidden(w)
			return
		}
		app.pullBackAgenda(w, r, id)
	case "documentation":
		if user.Role != roleStaff {
			forbidden(w)
			return
		}
		app.uploadDocumentation(w, r, id, user)
	default:
		http.NotFound(w, r)
	}
}

func (app *App) listAgendas(w http.ResponseWriter) {
	rows, err := app.db.Query(agendaSelect + " ORDER BY a.start_at ASC")
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Gagal mengambil agenda")
		return
	}
	defer rows.Close()

	agendas := make([]Agenda, 0)
	for rows.Next() {
		agenda, err := scanAgenda(rows)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "Gagal membaca agenda")
			return
		}
		agendas = append(agendas, agenda)
	}
	writeJSON(w, http.StatusOK, agendas)
}

func (app *App) createAgenda(w http.ResponseWriter, r *http.Request, user User) {
	if err := r.ParseMultipartForm(24 << 20); err != nil {
		badRequest(w, "Form agenda tidak valid")
		return
	}

	values, ok := requiredFormValues(w, r, "title", "location", "organizer", "staff_note")
	if !ok {
		return
	}
	title, location, organizer, staffNote := values[0], values[1], values[2], values[3]

	startAt, endAt, ok := parseAgendaTimes(w, r)
	if !ok {
		return
	}

	var invitationID sql.NullInt64
	if fh, ok := firstUploadedFile(r, "invitation"); ok {
		fileID, err := app.saveUpload(fh, user.ID)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "Gagal menyimpan undangan")
			return
		}
		invitationID = sql.NullInt64{Int64: fileID, Valid: true}
	}

	result, err := app.db.Exec(`
		INSERT INTO agendas
			(title, location, start_at, end_at, organizer, staff_note, invitation_file_id, created_by)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		title, location, startAt, endAt, organizer, staffNote, invitationID, user.ID,
	)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Gagal menyimpan agenda")
		return
	}

	agendaID, _ := result.LastInsertId()
	app.addNotification(roleLeader, "Agenda baru menunggu validasi", title+" telah disubmit staf. Email pemberitahuan disimulasikan terkirim ke pimpinan.", true)

	agenda, err := app.getAgenda(agendaID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Agenda tersimpan, tetapi gagal dimuat")
		return
	}
	writeJSON(w, http.StatusCreated, agenda)
}

func (app *App) validateAgenda(w http.ResponseWriter, r *http.Request, id int64, user User) {
	if r.Method != http.MethodPut {
		methodNotAllowed(w)
		return
	}

	var req struct {
		Status     string `json:"status"`
		Delegate   string `json:"delegate"`
		LeaderNote string `json:"leader_note"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		badRequest(w, "Data validasi tidak valid")
		return
	}
	if !validDecision(req.Status, req.Delegate) {
		badRequest(w, "Status harus hadir atau diwakili, dan perwakilan wajib diisi jika diwakili")
		return
	}
	if req.Status == statusAttend {
		req.Delegate = ""
	}

	_, err := app.db.Exec(`
		UPDATE agendas
		SET status = ?, delegate = ?, leader_note = ?, validated_by = ?, validated_at = NOW(), pulled_back_at = NULL
		WHERE id = ?`,
		req.Status, req.Delegate, req.LeaderNote, user.ID, id,
	)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Gagal memvalidasi agenda")
		return
	}

	agenda, err := app.getAgenda(id)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	app.addNotification(roleStaff, "Agenda telah divalidasi pimpinan", validationMessage(agenda), false)
	writeJSON(w, http.StatusOK, agenda)
}

func (app *App) pullBackAgenda(w http.ResponseWriter, r *http.Request, id int64) {
	if r.Method != http.MethodPost {
		methodNotAllowed(w)
		return
	}

	agenda, err := app.getAgenda(id)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	if agenda.Status == statusWait {
		badRequest(w, "Agenda belum divalidasi")
		return
	}
	if !agenda.CanRevise {
		writeError(w, http.StatusConflict, "Validasi tidak bisa diubah karena kegiatan kurang dari 24 jam atau sudah berjalan")
		return
	}

	_, err = app.db.Exec(`
		UPDATE agendas
		SET status = 'menunggu', delegate = NULL, leader_note = NULL, validated_by = NULL,
		    validated_at = NULL, pulled_back_at = NOW()
		WHERE id = ?`,
		id,
	)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Gagal menarik validasi")
		return
	}

	app.addNotification(roleStaff, "Validasi agenda ditarik kembali", agenda.Title+" dikembalikan ke status menunggu untuk perubahan keputusan pimpinan.", false)
	updated, _ := app.getAgenda(id)
	writeJSON(w, http.StatusOK, updated)
}

func (app *App) uploadDocumentation(w http.ResponseWriter, r *http.Request, id int64, user User) {
	if r.Method != http.MethodPost {
		methodNotAllowed(w)
		return
	}
	if err := r.ParseMultipartForm(32 << 20); err != nil {
		badRequest(w, "Form dokumentasi tidak valid")
		return
	}

	fh, ok := firstUploadedFile(r, "documentation")
	if !ok {
		badRequest(w, "File dokumentasi wajib diunggah")
		return
	}

	fileID, err := app.saveUpload(fh, user.ID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Gagal menyimpan dokumentasi")
		return
	}

	_, err = app.db.Exec(
		"UPDATE agendas SET documentation_file_id = ?, report_note = ? WHERE id = ?",
		fileID, r.FormValue("report_note"), id,
	)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Gagal menyimpan laporan")
		return
	}

	agenda, err := app.getAgenda(id)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	app.addNotification(roleLeader, "Dokumentasi kegiatan tersedia", "Laporan dokumentasi untuk "+agenda.Title+" telah diunggah staf.", false)
	writeJSON(w, http.StatusOK, agenda)
}

func (app *App) getAgenda(id int64) (Agenda, error) {
	row := app.db.QueryRow(agendaSelect+" WHERE a.id = ?", id)
	return scanAgenda(row)
}

func scanAgenda(scanner agendaScanner) (Agenda, error) {
	var agenda Agenda
	var startAt, endAt, createdAt time.Time
	var validatedAt, pulledBackAt sql.NullTime
	var invitationID, invitationSize, docID, docSize sql.NullInt64
	var invitationName, invitationMime, docName, docMime sql.NullString
	var invitationCreated, docCreated sql.NullTime

	err := scanner.Scan(
		&agenda.ID, &agenda.Title, &agenda.Location, &startAt, &endAt, &agenda.Organizer,
		&agenda.StaffNote, &agenda.Status, &agenda.Delegate, &agenda.LeaderNote,
		&validatedAt, &pulledBackAt, &agenda.ReportNote, &createdAt,
		&invitationID, &invitationName, &invitationMime, &invitationSize, &invitationCreated,
		&docID, &docName, &docMime, &docSize, &docCreated,
	)
	if err != nil {
		return Agenda{}, err
	}

	agenda.StartAt = startAt.Format(time.RFC3339)
	agenda.EndAt = endAt.Format(time.RFC3339)
	agenda.CreatedAt = createdAt.Format(time.RFC3339)
	agenda.CanRevise = canRevise(startAt)
	if time.Until(startAt) > 0 {
		agenda.HoursUntil = int64(time.Until(startAt).Hours())
	}
	agenda.ValidatedAt = nullableTime(validatedAt)
	agenda.PulledBackAt = nullableTime(pulledBackAt)
	agenda.Invitation = nullableFile(invitationID, invitationName, invitationMime, invitationSize, invitationCreated)
	agenda.Documentation = nullableFile(docID, docName, docMime, docSize, docCreated)
	return agenda, nil
}

func parseAgendaAction(w http.ResponseWriter, path string) (int64, string, bool) {
	trimmed := strings.TrimPrefix(path, "/api/agendas/")
	parts := strings.Split(strings.Trim(trimmed, "/"), "/")
	if len(parts) != 2 {
		return 0, "", false
	}

	id, err := strconv.ParseInt(parts[0], 10, 64)
	if err != nil {
		badRequest(w, "ID agenda tidak valid")
		return 0, "", false
	}
	return id, parts[1], true
}

func parseAgendaTimes(w http.ResponseWriter, r *http.Request) (time.Time, time.Time, bool) {
	startAt, err := parseInputTime(r.FormValue("start_at"))
	if err != nil {
		badRequest(w, "Waktu mulai tidak valid")
		return time.Time{}, time.Time{}, false
	}

	endAt, err := parseInputTime(r.FormValue("end_at"))
	if err != nil || !endAt.After(startAt) {
		badRequest(w, "Waktu selesai harus setelah waktu mulai")
		return time.Time{}, time.Time{}, false
	}
	return startAt, endAt, true
}

func validDecision(status, delegate string) bool {
	if status == statusAttend {
		return true
	}
	return status == statusDelegate && strings.TrimSpace(delegate) != ""
}

func validationMessage(agenda Agenda) string {
	if agenda.Status == statusDelegate {
		return agenda.Title + " diputuskan diwakili oleh " + agenda.Delegate + "."
	}
	return agenda.Title + " diputuskan dihadiri langsung."
}

func nullableTime(value sql.NullTime) *string {
	if !value.Valid {
		return nil
	}
	formatted := value.Time.Format(time.RFC3339)
	return &formatted
}

func nullableFile(id sql.NullInt64, name, mime sql.NullString, size sql.NullInt64, created sql.NullTime) *FileRecord {
	if !id.Valid {
		return nil
	}
	return &FileRecord{
		ID:           id.Int64,
		OriginalName: name.String,
		MimeType:     mime.String,
		SizeBytes:    size.Int64,
		CreatedAt:    created.Time.Format(time.RFC3339),
	}
}
