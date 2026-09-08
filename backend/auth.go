package main

import (
	"encoding/json"
	"net/http"
	"strings"

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
	if !validRole(req.Role) {
		badRequest(w, "Role harus admin, staf, atau pimpinan")
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

	hash, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Gagal membuat password")
		return
	}

	existingUser, _, err := app.findUserByEmail(req.Email)
	if err != nil {
		writeError(w, http.StatusForbidden, "Email belum terdaftar di sistem")
		return
	}
	if existingUser.Role != req.Role {
		writeError(w, http.StatusUnauthorized, "Role tidak sesuai dengan email terdaftar")
		return
	}

	_, err = app.db.Exec(
		"UPDATE users SET name = ?, position = ?, password_hash = ? WHERE id = ?",
		req.Name, req.Position, string(hash), existingUser.ID,
	)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Gagal memperbarui akun")
		return
	}

	user := User{
		ID:       existingUser.ID,
		Name:     req.Name,
		Email:    req.Email,
		Role:     req.Role,
		Position: req.Position,
	}
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
