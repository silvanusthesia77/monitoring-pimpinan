package main

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"golang.org/x/crypto/bcrypt"
)

func (app *App) login(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		methodNotAllowed(w)
		return
	}

	var req struct {
		Email    string `json:"email"`
		Password string `json:"password"`
		Role     string `json:"role"`
		Code     string `json:"code"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		badRequest(w, "Format login tidak valid")
		return
	}

	req.Email = strings.TrimSpace(strings.ToLower(req.Email))
	req.Role = strings.TrimSpace(strings.ToLower(req.Role))
	if !validRole(req.Role) {
		expireSessionCookie(w)
		badRequest(w, "Role harus admin, staf, atau pimpinan")
		return
	}
	if strings.ContainsAny(req.Password, " \t\r\n") {
		expireSessionCookie(w)
		badRequest(w, "Password tidak boleh mengandung spasi")
		return
	}

	user, passwordHash, err := app.findUserByEmail(req.Email)
	if err != nil || bcrypt.CompareHashAndPassword([]byte(passwordHash), []byte(req.Password)) != nil {
		expireSessionCookie(w)
		writeError(w, http.StatusUnauthorized, "Email atau password salah")
		return
	}
	if user.Role != req.Role {
		expireSessionCookie(w)
		writeError(w, http.StatusUnauthorized, "Role tidak sesuai dengan akun")
		return
	}
	if strings.HasSuffix(req.Email, "@gmail.com") {
		if strings.TrimSpace(req.Code) == "" {
			app.sendLoginCode(w, req.Email, req.Role)
			return
		}
		if !app.verifyLoginCode(w, req.Email, req.Role, req.Code) {
			return
		}
	}

	token, err := app.sessions.Create(user)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Gagal membuat sesi")
		return
	}

	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookie,
		Value:    token,
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   86400,
	})
	writeJSON(w, http.StatusOK, user)
}

func (app *App) sendLoginCode(w http.ResponseWriter, email, role string) {
	if !app.mailer.Enabled() {
		writeError(w, http.StatusServiceUnavailable, "SMTP Gmail belum dikonfigurasi, kode OTP login tidak bisa dikirim")
		return
	}

	code, err := numericCode(6)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Gagal membuat kode OTP login")
		return
	}
	codeHash, err := bcrypt.GenerateFromPassword([]byte(code), bcrypt.DefaultCost)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Gagal menyimpan kode OTP login")
		return
	}

	if _, err := app.mailer.Send([]string{email}, "Kode OTP login Agenda Monitor", "Kode OTP login Anda: "+code+". Kode berlaku 10 menit."); err != nil {
		writeError(w, http.StatusBadGateway, "Gagal mengirim kode OTP ke Gmail")
		return
	}

	_, err = app.db.Exec("DELETE FROM login_codes WHERE expires_at < NOW()")
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Gagal membersihkan kode OTP lama")
		return
	}

	_, err = app.db.Exec(`
		INSERT INTO login_codes (email, code_hash, role, expires_at)
		VALUES (?, ?, ?, DATE_ADD(NOW(), INTERVAL 10 MINUTE))
		ON DUPLICATE KEY UPDATE
			code_hash = VALUES(code_hash),
			role = VALUES(role),
			expires_at = VALUES(expires_at)`,
		email, string(codeHash), role,
	)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Gagal menyimpan kode OTP login")
		return
	}

	writeJSON(w, http.StatusAccepted, map[string]any{
		"pending_verification": true,
		"message":              "Kode OTP login sudah dikirim ke Gmail",
	})
}

func (app *App) verifyLoginCode(w http.ResponseWriter, email, role, code string) bool {
	var savedRole, codeHash string
	var expiresAt time.Time
	err := app.db.QueryRow("SELECT role, code_hash, expires_at FROM login_codes WHERE email = ?", email).
		Scan(&savedRole, &codeHash, &expiresAt)
	if err == sql.ErrNoRows {
		badRequest(w, "Kode OTP login belum diminta atau sudah kadaluarsa")
		return false
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Gagal membaca kode OTP login")
		return false
	}
	if time.Now().After(expiresAt) {
		_, _ = app.db.Exec("DELETE FROM login_codes WHERE email = ?", email)
		badRequest(w, "Kode OTP login sudah kadaluarsa")
		return false
	}
	if savedRole != role {
		badRequest(w, "Role berubah, kirim ulang kode OTP login")
		return false
	}
	if bcrypt.CompareHashAndPassword([]byte(codeHash), []byte(strings.TrimSpace(code))) != nil {
		badRequest(w, "Kode OTP login salah")
		return false
	}
	_, _ = app.db.Exec("DELETE FROM login_codes WHERE email = ?", email)
	return true
}

func (app *App) register(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		methodNotAllowed(w)
		return
	}

	var req struct {
		Name     string `json:"name"`
		Email    string `json:"email"`
		Password string `json:"password"`
		Role     string `json:"role"`
		Position string `json:"position"`
		Code     string `json:"code"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		badRequest(w, "Format daftar tidak valid")
		return
	}

	req.Name = strings.TrimSpace(req.Name)
	req.Email = strings.TrimSpace(strings.ToLower(req.Email))
	req.Role = strings.TrimSpace(strings.ToLower(req.Role))
	req.Position = strings.TrimSpace(req.Position)
	if req.Name == "" || req.Email == "" || req.Password == "" || req.Position == "" {
		badRequest(w, "Nama, email, password, dan jabatan wajib diisi")
		return
	}
	if req.Role != roleStaff && req.Role != roleLeader {
		badRequest(w, "Daftar hanya untuk role staf atau pimpinan")
		return
	}
	if !strings.HasSuffix(req.Email, "@gmail.com") {
		badRequest(w, "Email daftar harus akun Gmail")
		return
	}
	if len(req.Password) < 6 {
		badRequest(w, "Password minimal 6 karakter")
		return
	}
	if strings.ContainsAny(req.Password, " \t\r\n") {
		badRequest(w, "Password tidak boleh mengandung spasi")
		return
	}

	if strings.TrimSpace(req.Code) == "" {
		app.sendRegistrationCode(w, req.Name, req.Email, req.Password, req.Role, req.Position)
		return
	}

	app.verifyRegistration(w, req.Name, req.Email, req.Password, req.Role, req.Position, req.Code)
}

func (app *App) sendRegistrationCode(w http.ResponseWriter, name, email, password, role, position string) {
	if !app.mailer.Enabled() {
		writeError(w, http.StatusServiceUnavailable, "SMTP Gmail belum dikonfigurasi, kode verifikasi tidak bisa dikirim")
		return
	}

	passwordHash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Gagal membuat password")
		return
	}

	code, err := numericCode(6)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Gagal membuat kode verifikasi")
		return
	}
	codeHash, err := bcrypt.GenerateFromPassword([]byte(code), bcrypt.DefaultCost)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Gagal menyimpan kode verifikasi")
		return
	}

	if _, err := app.mailer.Send([]string{email}, "Kode verifikasi daftar Agenda Monitor", "Kode verifikasi akun Anda: "+code+". Kode berlaku 10 menit."); err != nil {
		writeError(w, http.StatusBadGateway, "Gagal mengirim kode ke Gmail")
		return
	}

	_, err = app.db.Exec("DELETE FROM registration_codes WHERE expires_at < NOW()")
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Gagal membersihkan kode lama")
		return
	}

	_, err = app.db.Exec(`
		INSERT INTO registration_codes (email, code_hash, name, password_hash, role, position, expires_at)
		VALUES (?, ?, ?, ?, ?, ?, DATE_ADD(NOW(), INTERVAL 10 MINUTE))
		ON DUPLICATE KEY UPDATE
			code_hash = VALUES(code_hash),
			name = VALUES(name),
			password_hash = VALUES(password_hash),
			role = VALUES(role),
			position = VALUES(position),
			expires_at = VALUES(expires_at)`,
		email, string(codeHash), name, string(passwordHash), role, position,
	)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Gagal menyimpan kode verifikasi")
		return
	}

	writeJSON(w, http.StatusAccepted, map[string]any{
		"pending_verification": true,
		"message":              "Kode verifikasi sudah dikirim ke Gmail",
	})
}

func (app *App) verifyRegistration(w http.ResponseWriter, name, email, password, role, position, code string) {
	var savedName, savedPasswordHash, savedRole, savedPosition, codeHash string
	var expiresAt time.Time
	err := app.db.QueryRow(`
		SELECT name, password_hash, role, position, code_hash, expires_at
		FROM registration_codes
		WHERE email = ?`, email).
		Scan(&savedName, &savedPasswordHash, &savedRole, &savedPosition, &codeHash, &expiresAt)
	if err == sql.ErrNoRows {
		badRequest(w, "Kode verifikasi belum diminta atau sudah kadaluarsa")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Gagal membaca kode verifikasi")
		return
	}
	if time.Now().After(expiresAt) {
		_, _ = app.db.Exec("DELETE FROM registration_codes WHERE email = ?", email)
		badRequest(w, "Kode verifikasi sudah kadaluarsa")
		return
	}
	if bcrypt.CompareHashAndPassword([]byte(codeHash), []byte(strings.TrimSpace(code))) != nil {
		badRequest(w, "Kode verifikasi salah")
		return
	}
	if savedRole != role || savedName != name || savedPosition != position {
		badRequest(w, "Data daftar berubah, kirim ulang kode verifikasi")
		return
	}

	existingUser, _, err := app.findUserByEmail(email)
	if err == nil && existingUser.Role != role {
		writeError(w, http.StatusUnauthorized, "Role tidak sesuai dengan email terdaftar")
		return
	}
	if err != nil && err != sql.ErrNoRows {
		writeError(w, http.StatusInternalServerError, "Gagal memeriksa email")
		return
	}

	user := User{Name: name, Email: email, Role: role, Position: position}
	if existingUser.ID > 0 {
		_, err = app.db.Exec(
			"UPDATE users SET name = ?, position = ?, password_hash = ? WHERE id = ?",
			name, position, savedPasswordHash, existingUser.ID,
		)
		user.ID = existingUser.ID
	} else {
		result, insertErr := app.db.Exec(
			"INSERT INTO users (name, email, password_hash, role, position) VALUES (?, ?, ?, ?, ?)",
			name, email, savedPasswordHash, role, position,
		)
		err = insertErr
		if err == nil {
			user.ID, _ = result.LastInsertId()
		}
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Gagal menyimpan akun")
		return
	}
	_, _ = app.db.Exec("DELETE FROM registration_codes WHERE email = ?", email)

	token, err := app.sessions.Create(user)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Akun dibuat, tetapi sesi gagal dibuat")
		return
	}

	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookie,
		Value:    token,
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   86400,
	})
	writeJSON(w, http.StatusCreated, user)
}

func (app *App) logout(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		methodNotAllowed(w)
		return
	}

	cookie, err := r.Cookie(sessionCookie)
	if err == nil {
		app.sessions.Delete(cookie.Value)
	}
	expireSessionCookie(w)
	writeJSON(w, http.StatusOK, map[string]string{"message": "Keluar dari sistem"})
}

func (app *App) me(w http.ResponseWriter, r *http.Request) {
	user, ok := app.currentUser(r)
	if !ok {
		writeError(w, http.StatusUnauthorized, "Belum login")
		return
	}
	writeJSON(w, http.StatusOK, user)
}

func (app *App) findUserByEmail(email string) (User, string, error) {
	var user User
	var passwordHash string
	err := app.db.QueryRow(
		"SELECT id, name, email, password_hash, role, position FROM users WHERE email = ?",
		email,
	).Scan(&user.ID, &user.Name, &user.Email, &passwordHash, &user.Role, &user.Position)
	return user, passwordHash, err
}

func expireSessionCookie(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{Name: sessionCookie, Path: "/", MaxAge: -1})
}

func validRole(role string) bool {
	return role == roleAdmin || role == roleStaff || role == roleLeader
}
