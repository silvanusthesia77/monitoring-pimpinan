package main

import (
	"encoding/json"
	"log"
	"net/http"
	"strings"
)

func (app *App) currentUser(r *http.Request) (User, bool) {
	cookie, err := r.Cookie(sessionCookie)
	if err != nil {
		return User{}, false
	}
	return app.sessions.Get(cookie.Value)
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]string{"error": message})
}

func badRequest(w http.ResponseWriter, message string) {
	writeError(w, http.StatusBadRequest, message)
}

func forbidden(w http.ResponseWriter) {
	writeError(w, http.StatusForbidden, "Akses ditolak untuk role ini")
}

func methodNotAllowed(w http.ResponseWriter) {
	writeError(w, http.StatusMethodNotAllowed, "Method tidak diizinkan")
}

func requiredFormValues(w http.ResponseWriter, r *http.Request, fields ...string) ([]string, bool) {
	values := make([]string, len(fields))
	for index, field := range fields {
		value := strings.TrimSpace(r.FormValue(field))
		if value == "" {
			badRequest(w, "Field "+field+" wajib diisi")
			return nil, false
		}
		values[index] = value
	}
	return values, true
}

func logRequest(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		log.Printf("%s %s", r.Method, r.URL.Path)
		next.ServeHTTP(w, r)
	})
}
