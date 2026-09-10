package main

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"log"
	"mime"
	"net/smtp"
	"os"
	"strconv"
	"strings"
)

type Mailer struct {
	Host     string
	Port     string
	Username string
	Password string
	From     string
}

type EmailAttachment struct {
	Filename string
	MimeType string
	Path     string
	Inline   bool
}

func newMailerFromEnv() Mailer {
	return Mailer{
		Host:     env("SMTP_HOST", "smtp.gmail.com"),
		Port:     env("SMTP_PORT", "587"),
		Username: env("SMTP_USERNAME", "danpixelwrld@gmail.com"),
		Password: env("SMTP_PASSWORD", ""),
		From:     env("SMTP_FROM", env("SMTP_USERNAME", "danpixelwrld@gmail.com")),
	}
}

func (mailer Mailer) Enabled() bool {
	return mailer.Host != "" && mailer.Username != "" && mailer.Password != "" && mailer.From != ""
}

func (mailer Mailer) Send(to []string, subject, body string, attachments ...EmailAttachment) (bool, error) {
	if len(to) == 0 {
		return false, nil
	}

	if !mailer.Enabled() {
		log.Printf("[EMAIL SIMULASI] to=%s subject=%s body=%s", strings.Join(to, ","), subject, body)
		return false, nil
	}

	port, err := strconv.Atoi(mailer.Port)
	if err != nil || port <= 0 {
		port = 587
	}

	address := fmt.Sprintf("%s:%d", mailer.Host, port)
	auth := smtp.PlainAuth("", mailer.Username, mailer.Password, mailer.Host)
	message, err := mailer.message(to, subject, body, attachments)
	if err != nil {
		return false, err
	}

	if err := smtp.SendMail(address, auth, mailer.From, to, []byte(message)); err != nil {
		return false, err
	}
	return true, nil
}

func (mailer Mailer) message(to []string, subject, body string, attachments []EmailAttachment) (string, error) {
	headers := []string{
		"From: " + mailer.From,
		"To: " + strings.Join(to, ","),
		"Subject: " + subject,
		"MIME-Version: 1.0",
	}

	if len(attachments) == 0 {
		return strings.Join(append(headers, "Content-Type: text/plain; charset=UTF-8", "", body), "\r\n") + "\r\n", nil
	}

	boundary := "agenda-monitor-" + strings.ReplaceAll(randomBoundary(), "=", "")
	var buffer bytes.Buffer
	headers = append(headers, `Content-Type: multipart/mixed; boundary="`+boundary+`"`)
	buffer.WriteString(strings.Join(headers, "\r\n") + "\r\n\r\n")
	buffer.WriteString("--" + boundary + "\r\n")
	buffer.WriteString("Content-Type: text/plain; charset=UTF-8\r\n")
	buffer.WriteString("Content-Transfer-Encoding: 8bit\r\n\r\n")
	buffer.WriteString(body + "\r\n")

	for _, attachment := range attachments {
		data, err := os.ReadFile(attachment.Path)
		if err != nil {
			return "", err
		}
		filename := safeDownloadName(attachment.Filename)
		mimeType := attachment.MimeType
		if mimeType == "" {
			mimeType = "application/octet-stream"
		}
		disposition := "attachment"
		if attachment.Inline {
			disposition = "inline"
		}

		buffer.WriteString("--" + boundary + "\r\n")
		buffer.WriteString("Content-Type: " + mimeType + `; name="` + mime.QEncoding.Encode("UTF-8", filename) + `"` + "\r\n")
		buffer.WriteString("Content-Transfer-Encoding: base64\r\n")
		buffer.WriteString("Content-Disposition: " + disposition + `; filename="` + mime.QEncoding.Encode("UTF-8", filename) + `"` + "\r\n\r\n")
		writeBase64Lines(&buffer, data)
		buffer.WriteString("\r\n")
	}

	buffer.WriteString("--" + boundary + "--\r\n")
	return buffer.String(), nil
}

func writeBase64Lines(buffer *bytes.Buffer, data []byte) {
	encoded := base64.StdEncoding.EncodeToString(data)
	for len(encoded) > 76 {
		buffer.WriteString(encoded[:76] + "\r\n")
		encoded = encoded[76:]
	}
	buffer.WriteString(encoded)
}

func randomBoundary() string {
	token, err := randomToken()
	if err != nil {
		return "fallback"
	}
	return token
}

func (app *App) emailsForRole(role string) []string {
	rows, err := app.db.Query("SELECT email FROM users WHERE role = ?", role)
	if err != nil {
		log.Printf("gagal mengambil email role %s: %v", role, err)
		return nil
	}
	defer rows.Close()

	emails := make([]string, 0)
	for rows.Next() {
		var email string
		if err := rows.Scan(&email); err == nil {
			emails = append(emails, email)
		}
	}
	return emails
}
