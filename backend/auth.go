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
		badRequest(w, "Role harus staf atau pimpinan")
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
	return role == roleStaff || role == roleLeader
}
