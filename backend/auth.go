package main

import (
	"encoding/json"
	"net/http"

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

	user, passwordHash, err := app.findUserByEmail(req.Email)
	if err != nil || bcrypt.CompareHashAndPassword([]byte(passwordHash), []byte(req.Password)) != nil {
		writeError(w, http.StatusUnauthorized, "Email atau password salah")
		return
	}
	if req.Role != "" && user.Role != req.Role {
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

func (app *App) logout(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		methodNotAllowed(w)
		return
	}

	cookie, err := r.Cookie(sessionCookie)
	if err == nil {
		app.sessions.Delete(cookie.Value)
	}
	http.SetCookie(w, &http.Cookie{Name: sessionCookie, Path: "/", MaxAge: -1})
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
