package main

import (
	"log"
	"net/http"
	"time"
)

func (app *App) notifications(w http.ResponseWriter, r *http.Request) {
	user, ok := app.currentUser(r)
	if !ok {
		writeError(w, http.StatusUnauthorized, "Belum login")
		return
	}

	rows, err := app.db.Query(`
		SELECT id, audience, title, body, email_sent, created_at
		FROM notifications
		WHERE audience IN (?, 'semua')
		ORDER BY created_at DESC
		LIMIT 30`, user.Role)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Gagal mengambil notifikasi")
		return
	}
	defer rows.Close()

	notifications := make([]Notification, 0)
	for rows.Next() {
		var item Notification
		var createdAt time.Time
		if err := rows.Scan(&item.ID, &item.Audience, &item.Title, &item.Body, &item.EmailSent, &createdAt); err != nil {
			writeError(w, http.StatusInternalServerError, "Gagal membaca notifikasi")
			return
		}
		item.CreatedAt = createdAt.Format(time.RFC3339)
		notifications = append(notifications, item)
	}
	writeJSON(w, http.StatusOK, notifications)
}

func (app *App) addNotification(audience, title, body string, emailSent bool) {
	_, err := app.db.Exec(
		"INSERT INTO notifications (audience, title, body, email_sent) VALUES (?, ?, ?, ?)",
		audience, title, body, emailSent,
	)
	if err != nil {
		log.Printf("gagal menyimpan notifikasi: %v", err)
	}
	if emailSent {
		log.Printf("[EMAIL SIMULASI] %s - %s", title, body)
	}
}
