package main

import (
	"fmt"
	"log"
	"net/smtp"
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

func (mailer Mailer) Send(to []string, subject, body string) (bool, error) {
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
	message := []byte(
		"From: " + mailer.From + "\r\n" +
			"To: " + strings.Join(to, ",") + "\r\n" +
			"Subject: " + subject + "\r\n" +
			"MIME-Version: 1.0\r\n" +
			"Content-Type: text/plain; charset=UTF-8\r\n\r\n" +
			body + "\r\n",
	)

	if err := smtp.SendMail(address, auth, mailer.From, to, message); err != nil {
		return false, err
	}
	return true, nil
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
