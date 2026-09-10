package main

import (
	"database/sql"
	"log"
	"net/http"
	"strings"
	"time"
)

func (app *App) notifications(w http.ResponseWriter, r *http.Request) {
	user, ok := app.currentUser(r)
	if !ok {
		writeError(w, http.StatusUnauthorized, "Belum login")
		return
	}

	rows, err := app.db.Query(`
		SELECT id, audience, agenda_id, title, body, email_sent, COALESCE(email_message, ''), created_at
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
		var agendaID sql.NullInt64
		if err := rows.Scan(&item.ID, &item.Audience, &agendaID, &item.Title, &item.Body, &item.EmailSent, &item.EmailMessage, &createdAt); err != nil {
			writeError(w, http.StatusInternalServerError, "Gagal membaca notifikasi")
			return
		}
		if agendaID.Valid {
			item.AgendaID = &agendaID.Int64
		}
		item.CreatedAt = createdAt.Format(time.RFC3339)
		notifications = append(notifications, item)
	}
	writeJSON(w, http.StatusOK, notifications)
}

func (app *App) addNotification(audience, title, body string, agendaID int64, emailSent bool, emailMessage string) {
	relatedAgendaID := sql.NullInt64{Int64: agendaID, Valid: agendaID > 0}
	_, err := app.db.Exec(
		"INSERT INTO notifications (audience, agenda_id, title, body, email_sent, email_message) VALUES (?, ?, ?, ?, ?, NULLIF(?, ''))",
		audience, relatedAgendaID, title, body, emailSent, emailMessage,
	)
	if err != nil {
		log.Printf("gagal menyimpan notifikasi: %v", err)
	}
}

func (app *App) notifyRole(audience, title, body string, sendEmail bool, options ...EmailOption) EmailDelivery {
	delivery := EmailDelivery{
		Attempted: sendEmail,
		Message:   "Notifikasi web berhasil dibuat.",
	}

	if sendEmail {
		recipients := app.emailsForRole(audience)
		messageBody := body
		attachments := []EmailAttachment{}
		for _, option := range options {
			if strings.TrimSpace(option.Body) != "" {
				messageBody = option.Body
			}
			attachments = append(attachments, option.Attachments...)
		}

		if len(recipients) == 0 {
			delivery.Message = "Email gagal dikirim: tidak ada alamat penerima untuk role ini."
		} else if !app.mailer.Enabled() {
			delivery.Message = "Email belum terkirim: SMTP Gmail belum dikonfigurasi."
			_, _ = app.mailer.Send(recipients, title, messageBody, attachments...)
		} else {
			emailSent, err := app.mailer.Send(recipients, title, messageBody, attachments...)
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
	var agendaID int64
	for _, option := range options {
		if option.RelatedAgendaID > 0 {
			agendaID = option.RelatedAgendaID
		}
	}
	app.addNotification(audience, title, body, agendaID, delivery.Sent, emailMessage)
	return delivery
}
