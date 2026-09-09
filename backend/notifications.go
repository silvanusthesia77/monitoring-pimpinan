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
		SELECT id, audience, title, body, email_sent, COALESCE(email_message, ''), created_at
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
		if err := rows.Scan(&item.ID, &item.Audience, &item.Title, &item.Body, &item.EmailSent, &item.EmailMessage, &createdAt); err != nil {
			writeError(w, http.StatusInternalServerError, "Gagal membaca notifikasi")
			return
		}
		item.CreatedAt = createdAt.Format(time.RFC3339)
		notifications = append(notifications, item)
	}
	writeJSON(w, http.StatusOK, notifications)
}

func (app *App) addNotification(audience, title, body string, emailSent bool, emailMessage string) {
	_, err := app.db.Exec(
		"INSERT INTO notifications (audience, title, body, email_sent, email_message) VALUES (?, ?, ?, ?, NULLIF(?, ''))",
		audience, title, body, emailSent, emailMessage,
	)
	if err != nil {
		log.Printf("gagal menyimpan notifikasi: %v", err)
	}
}

func (app *App) notifyRole(audience, title, body string, sendEmail bool) EmailDelivery {
	delivery := EmailDelivery{
		Attempted: sendEmail,
		Message:   "Notifikasi web berhasil dibuat.",
	}

	if sendEmail {
		recipients := app.emailsForRole(audience)
		if len(recipients) == 0 {
			delivery.Message = "Email gagal dikirim: tidak ada alamat penerima untuk role ini."
		} else if !app.mailer.Enabled() {
			delivery.Message = "Email belum terkirim: SMTP Gmail belum dikonfigurasi."
			_, _ = app.mailer.Send(recipients, title, body)
		} else {
			emailSent, err := app.mailer.Send(recipients, title, body)
			delivery.Sent = emailSent
			if err != nil {
				log.Printf("gagal mengirim email notifikasi: %v", err)
				delivery.Message = "Email gagal dikirim: " + err.Error()
			} else if emailSent {
				delivery.Message = "Email berhasil dikirim ke Gmail pimpinan."
			} else {
				delivery.Message = "Email belum terkirim: SMTP Gmail belum mengirim pesan."
			}
		}
	}
	emailMessage := ""
	if delivery.Attempted {
		emailMessage = delivery.Message
	}
	app.addNotification(audience, title, body, delivery.Sent, emailMessage)
	return delivery
}
