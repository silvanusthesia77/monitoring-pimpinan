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

	if action == "" {
		if r.Method != http.MethodDelete {
			methodNotAllowed(w)
			return
		}
		if user.Role != roleLeader {
			forbidden(w)
			return
		}
		app.deleteAgenda(w, id)
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
	case "report-pdf":
		app.downloadReportPDF(w, r, id)
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
		if err := app.loadDocumentationFiles(&agenda); err != nil {
			writeError(w, http.StatusInternalServerError, "Gagal membaca lampiran dokumentasi")
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
	fh, ok := firstUploadedFile(r, "invitation")
	if !ok || fh.Size == 0 {
		badRequest(w, "File undangan wajib diunggah")
		return
	}
	fileID, err := app.saveUpload(fh, user.ID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Gagal menyimpan undangan")
		return
	}
	invitationID = sql.NullInt64{Int64: fileID, Valid: true}

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
	attachments := []EmailAttachment{}
	if attachment, err := app.emailAttachment(fileID); err == nil {
		attachments = append(attachments, attachment)
	}
	notificationBody := title + " telah disubmit staf dan membutuhkan validasi pimpinan."
	emailStatus := app.notifyRole(roleLeader, "Agenda baru menunggu validasi", notificationBody, true, EmailOption{
		Body:            newAgendaEmailBody(title, location, organizer, staffNote, startAt, endAt),
		Attachments:     attachments,
		RelatedAgendaID: agendaID,
	})

	agenda, err := app.getAgenda(agendaID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Agenda tersimpan, tetapi gagal dimuat")
		return
	}
	writeJSON(w, http.StatusCreated, AgendaCreateResponse{Agenda: agenda, EmailStatus: emailStatus})
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

	agendaBefore, err := app.getAgenda(id)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	if !agendaBefore.CanValidate {
		writeError(w, http.StatusConflict, "Agenda sudah terkunci atau kegiatan sudah berjalan, validasi tidak bisa diubah")
		return
	}

	if !validDecision(req.Status, req.Delegate) {
		badRequest(w, "Status harus hadir atau diwakili, dan perwakilan wajib diisi jika diwakili")
		return
	}
	if req.Status == statusAttend {
		req.Delegate = ""
	}

	_, err = app.db.Exec(`
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
	if agendaBefore.Status == statusWait {
		app.notifyRole(roleStaff, "Agenda telah divalidasi pimpinan", validationMessage(agenda), false)
	} else {
		app.notifyRole(roleStaff, "Update Validasi Agenda", validationMessage(agenda)+" Keterangan pimpinan diperbarui.", false)
	}
	writeJSON(w, http.StatusOK, agenda)
}

func (app *App) deleteAgenda(w http.ResponseWriter, id int64) {
	agenda, err := app.getAgenda(id)
	if err != nil {
		writeError(w, http.StatusNotFound, "Agenda tidak ditemukan")
		return
	}

	result, err := app.db.Exec("DELETE FROM agendas WHERE id = ?", id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Gagal menghapus agenda")
		return
	}
	rowsAffected, _ := result.RowsAffected()
	if rowsAffected == 0 {
		writeError(w, http.StatusNotFound, "Agenda tidak ditemukan")
		return
	}

	app.notifyRole(roleStaff, "Agenda dihapus pimpinan", agenda.Title+" telah dihapus dari sistem.", false)
	writeJSON(w, http.StatusOK, map[string]string{"message": "Agenda berhasil dihapus"})
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

	app.notifyRole(roleStaff, "Validasi agenda ditarik kembali", agenda.Title+" dikembalikan ke status menunggu untuk perubahan keputusan pimpinan.", false)
	updated, _ := app.getAgenda(id)
	writeJSON(w, http.StatusOK, updated)
}

func (app *App) uploadDocumentation(w http.ResponseWriter, r *http.Request, id int64, user User) {
	if r.Method != http.MethodPost {
		methodNotAllowed(w)
		return
	}
	if err := r.ParseMultipartForm(64 << 20); err != nil {
		badRequest(w, "Form dokumentasi tidak valid")
		return
	}

	agendaBefore, err := app.getAgenda(id)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	if !agendaBefore.CanUploadDoc {
		writeError(w, http.StatusConflict, "Dokumentasi hanya bisa diupload setelah agenda divalidasi dan sebelum laporan tersimpan")
		return
	}

	files := r.MultipartForm.File["documentation"]
	if len(files) == 0 {
		badRequest(w, "Minimal satu file dokumentasi wajib diunggah")
		return
	}
	if len(files) > 5 {
		badRequest(w, "Maksimal upload 5 lampiran dokumentasi")
		return
	}
	for _, fh := range files {
		if fh.Size == 0 {
			badRequest(w, "File dokumentasi tidak boleh kosong")
			return
		}
		if !isPDFImage(contentType(fh)) {
			badRequest(w, "Lampiran dokumentasi harus berupa gambar JPG, PNG, atau GIF")
			return
		}
	}
	values, ok := requiredFormValues(w, r, "report_note")
	if !ok {
		return
	}
	reportNote := values[0]

	tx, err := app.db.Begin()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Gagal memulai penyimpanan laporan")
		return
	}
	defer tx.Rollback()

	fileIDs := make([]int64, 0, len(files))
	for _, fh := range files {
		fileID, err := app.saveUpload(fh, user.ID)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "Gagal menyimpan dokumentasi")
			return
		}
		fileIDs = append(fileIDs, fileID)
	}

	_, err = tx.Exec(
		"UPDATE agendas SET documentation_file_id = ?, report_note = ? WHERE id = ?",
		fileIDs[0], reportNote, id,
	)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Gagal menyimpan laporan")
		return
	}
	for index, fileID := range fileIDs {
		_, err = tx.Exec(
			"INSERT INTO agenda_documentation_files (agenda_id, file_id, sort_order) VALUES (?, ?, ?)",
			id, fileID, index+1,
		)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "Gagal menyimpan lampiran laporan")
			return
		}
	}
	if err := tx.Commit(); err != nil {
		writeError(w, http.StatusInternalServerError, "Gagal menyimpan laporan")
		return
	}

	agenda, err := app.getAgenda(id)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	app.notifyRole(roleLeader, "Dokumentasi kegiatan tersedia", "Laporan dokumentasi untuk "+agenda.Title+" telah diunggah staf.", false)
	writeJSON(w, http.StatusOK, agenda)
}

func (app *App) getAgenda(id int64) (Agenda, error) {
	row := app.db.QueryRow(agendaSelect+" WHERE a.id = ?", id)
	agenda, err := scanAgenda(row)
	if err != nil {
		return Agenda{}, err
	}
	if err := app.loadDocumentationFiles(&agenda); err != nil {
		return Agenda{}, err
	}
	return agenda, nil
}

func (app *App) loadDocumentationFiles(agenda *Agenda) error {
	rows, err := app.db.Query(`
		SELECT f.id, f.original_name, f.mime_type, f.size_bytes, f.created_at
		FROM agenda_documentation_files adf
		JOIN files f ON f.id = adf.file_id
		WHERE adf.agenda_id = ?
		ORDER BY adf.sort_order, f.created_at`, agenda.ID)
	if err != nil {
		return err
	}
	defer rows.Close()

	documents := make([]FileRecord, 0)
	for rows.Next() {
		var doc FileRecord
		var created time.Time
		if err := rows.Scan(&doc.ID, &doc.OriginalName, &doc.MimeType, &doc.SizeBytes, &created); err != nil {
			return err
		}
		doc.CreatedAt = created.Format(time.RFC3339)
		documents = append(documents, doc)
	}
	if err := rows.Err(); err != nil {
		return err
	}
	if len(documents) == 0 && agenda.Documentation != nil {
		documents = append(documents, *agenda.Documentation)
	}
	agenda.Documents = documents
	if len(agenda.Documents) > 0 && agenda.Documentation == nil {
		agenda.Documentation = &agenda.Documents[0]
	}
	return nil
}

func (app *App) getStoredDocumentationFiles(agendaID int64) ([]storedReportFile, error) {
	rows, err := app.db.Query(`
		SELECT f.id, f.original_name, f.stored_name, f.mime_type, f.size_bytes, f.created_at
		FROM agenda_documentation_files adf
		JOIN files f ON f.id = adf.file_id
		WHERE adf.agenda_id = ?
		ORDER BY adf.sort_order, f.created_at`, agendaID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	files := make([]storedReportFile, 0)
	for rows.Next() {
		var file storedReportFile
		var created time.Time
		if err := rows.Scan(&file.ID, &file.OriginalName, &file.StoredName, &file.MimeType, &file.SizeBytes, &created); err != nil {
			return nil, err
		}
		file.CreatedAt = created.Format(time.RFC3339)
		files = append(files, file)
	}
	return files, rows.Err()
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
	agenda.Phase, agenda.DisplayStatus = agendaLifecycle(agenda.Status, startAt, endAt, agenda.Documentation != nil)
	agenda.IsLocked = agenda.Phase == phaseLocked
	agenda.CanValidate = canValidateAgenda(agenda.Status, startAt)
	agenda.CanRevise = agenda.CanValidate
	agenda.CanPullback = agenda.Status != statusWait && agenda.CanValidate
	agenda.CanUploadDoc = agenda.Status != statusWait && agenda.Documentation == nil
	return agenda, nil
}

func parseAgendaAction(w http.ResponseWriter, path string) (int64, string, bool) {
	trimmed := strings.TrimPrefix(path, "/api/agendas/")
	parts := strings.Split(strings.Trim(trimmed, "/"), "/")
	if len(parts) < 1 || len(parts) > 2 || parts[0] == "" {
		return 0, "", false
	}

	id, err := strconv.ParseInt(parts[0], 10, 64)
	if err != nil {
		badRequest(w, "ID agenda tidak valid")
		return 0, "", false
	}
	if len(parts) == 1 {
		return id, "", true
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

func newAgendaEmailBody(title, location, organizer, staffNote string, startAt, endAt time.Time) string {
	return strings.Join([]string{
		"Yth. Pimpinan Kabupaten Sorong Selatan,",
		"",
		"Dengan hormat,",
		"",
		"Melalui sistem Aplikasi Monitor Agenda Pimpinan Kabupaten Sorong Selatan, staf telah mengajukan agenda baru yang memerlukan validasi Bapak/Ibu Pimpinan.",
		"",
		"Detail agenda:",
		"Nama kegiatan: " + title,
		"Lokasi: " + location,
		"Waktu mulai: " + formatIndonesianDateTime(startAt),
		"Waktu selesai: " + formatIndonesianDateTime(endAt),
		"Penyelenggara: " + organizer,
		"Keterangan staf: " + staffNote,
		"",
		"Mohon Bapak/Ibu Pimpinan dapat masuk ke aplikasi untuk meninjau agenda tersebut dan memberikan keputusan kehadiran, apakah hadir langsung atau diwakili.",
		"",
		"Terima kasih.",
		"",
		"Hormat kami,",
		"Sistem Aplikasi Monitor Agenda Pimpinan Kabupaten Sorong Selatan",
	}, "\n")
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
