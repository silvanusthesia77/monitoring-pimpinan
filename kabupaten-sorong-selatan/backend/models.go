package main

import (
	"database/sql"
	"time"
)

const (
	roleStaff      = "staf"
	roleLeader     = "pimpinan"
	roleAll        = "semua"
	statusWait     = "menunggu"
	statusAttend   = "hadir"
	statusDelegate = "diwakili"
	sessionCookie  = "agenda_session"
	defaultPort    = "8080"
)

type App struct {
	db          *sql.DB
	uploadDir   string
	frontendDir string
	sessions    *SessionStore
}

type User struct {
	ID       int64  `json:"id"`
	Name     string `json:"name"`
	Email    string `json:"email"`
	Role     string `json:"role"`
	Position string `json:"position"`
}

type FileRecord struct {
	ID           int64  `json:"id"`
	OriginalName string `json:"original_name"`
	MimeType     string `json:"mime_type"`
	SizeBytes    int64  `json:"size_bytes"`
	CreatedAt    string `json:"created_at"`
}

type Agenda struct {
	ID            int64       `json:"id"`
	Title         string      `json:"title"`
	Location      string      `json:"location"`
	StartAt       string      `json:"start_at"`
	EndAt         string      `json:"end_at"`
	Organizer     string      `json:"organizer"`
	StaffNote     string      `json:"staff_note"`
	Status        string      `json:"status"`
	Delegate      string      `json:"delegate"`
	LeaderNote    string      `json:"leader_note"`
	ValidatedAt   *string     `json:"validated_at"`
	PulledBackAt  *string     `json:"pulled_back_at"`
	ReportNote    string      `json:"report_note"`
	CreatedAt     string      `json:"created_at"`
	CanRevise     bool        `json:"can_revise"`
	HoursUntil    int64       `json:"hours_until"`
	Invitation    *FileRecord `json:"invitation"`
	Documentation *FileRecord `json:"documentation"`
}

type Notification struct {
	ID        int64  `json:"id"`
	Audience  string `json:"audience"`
	Title     string `json:"title"`
	Body      string `json:"body"`
	EmailSent bool   `json:"email_sent"`
	CreatedAt string `json:"created_at"`
}

type agendaScanner interface {
	Scan(dest ...any) error
}

func canRevise(startAt time.Time) bool {
	return time.Until(startAt) >= 24*time.Hour
}
