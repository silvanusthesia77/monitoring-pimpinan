package main

import (
	"database/sql"
	"net/http"
	"strconv"
	"strings"
	"time"
)

func (app *App) users(w http.ResponseWriter, r *http.Request) {
	user, ok := app.currentUser(r)
	if !ok {
		writeError(w, http.StatusUnauthorized, "Belum login")
		return
	}
	if user.Role != roleAdmin {
		forbidden(w)
		return
	}
	if r.Method != http.MethodGet {
		methodNotAllowed(w)
		return
	}

	rows, err := app.db.Query("SELECT id, name, email, role, position, created_at FROM users ORDER BY role, name")
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Gagal mengambil user")
		return
	}
	defer rows.Close()

	users := make([]UserListItem, 0)
	for rows.Next() {
		var item UserListItem
		var createdAt time.Time
		if err := rows.Scan(&item.ID, &item.Name, &item.Email, &item.Role, &item.Position, &createdAt); err != nil {
			writeError(w, http.StatusInternalServerError, "Gagal membaca user")
			return
		}
		item.CreatedAt = createdAt.Format(time.RFC3339)
		item.CanDelete = item.ID != user.ID
		users = append(users, item)
	}
	writeJSON(w, http.StatusOK, users)
}

func (app *App) userAction(w http.ResponseWriter, r *http.Request) {
	user, ok := app.currentUser(r)
	if !ok {
		writeError(w, http.StatusUnauthorized, "Belum login")
		return
	}
	if user.Role != roleAdmin {
		forbidden(w)
		return
	}
	if r.Method != http.MethodDelete {
		methodNotAllowed(w)
		return
	}

	id, ok := parseUserID(w, r.URL.Path)
	if !ok {
		return
	}
	app.deleteUser(w, id, user.ID)
}

func (app *App) deleteUser(w http.ResponseWriter, id, adminID int64) {
	if id == adminID {
		badRequest(w, "Admin tidak bisa menghapus akun yang sedang dipakai")
		return
	}

	tx, err := app.db.Begin()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Gagal memulai hapus user")
		return
	}
	defer tx.Rollback()

	var target User
	err = tx.QueryRow("SELECT id, name, email, role, position FROM users WHERE id = ?", id).
		Scan(&target.ID, &target.Name, &target.Email, &target.Role, &target.Position)
	if err == sql.ErrNoRows {
		writeError(w, http.StatusNotFound, "User tidak ditemukan")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Gagal membaca user")
		return
	}

	if target.Role == roleAdmin {
		var adminCount int
		if err := tx.QueryRow("SELECT COUNT(*) FROM users WHERE role = ?", roleAdmin).Scan(&adminCount); err != nil {
			writeError(w, http.StatusInternalServerError, "Gagal memeriksa admin")
			return
		}
		if adminCount <= 1 {
			badRequest(w, "Minimal harus ada satu admin aktif")
			return
		}
	}

	if _, err := tx.Exec("UPDATE files SET uploaded_by = ? WHERE uploaded_by = ?", adminID, id); err != nil {
		writeError(w, http.StatusInternalServerError, "Gagal memindahkan file user")
		return
	}
	if _, err := tx.Exec("UPDATE agendas SET created_by = ? WHERE created_by = ?", adminID, id); err != nil {
		writeError(w, http.StatusInternalServerError, "Gagal memindahkan agenda user")
		return
	}
	if _, err := tx.Exec("UPDATE agendas SET validated_by = NULL WHERE validated_by = ?", id); err != nil {
		writeError(w, http.StatusInternalServerError, "Gagal memindahkan validasi user")
		return
	}
	if _, err := tx.Exec("DELETE FROM users WHERE id = ?", id); err != nil {
		writeError(w, http.StatusInternalServerError, "Gagal menghapus user")
		return
	}
	if err := tx.Commit(); err != nil {
		writeError(w, http.StatusInternalServerError, "Gagal menyimpan hapus user")
		return
	}

	app.sessions.DeleteUser(id)
	writeJSON(w, http.StatusOK, map[string]string{"message": "User berhasil dihapus"})
}

func parseUserID(w http.ResponseWriter, path string) (int64, bool) {
	idText := strings.TrimPrefix(path, "/api/users/")
	id, err := strconv.ParseInt(strings.Trim(idText, "/"), 10, 64)
	if err != nil {
		badRequest(w, "ID user tidak valid")
		return 0, false
	}
	return id, true
}
