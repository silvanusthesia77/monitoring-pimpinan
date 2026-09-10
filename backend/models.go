package main

import (
	"database/sql"
	"time"
)

const (
	roleStaff                 = "staf"
	roleLeader                = "pimpinan"
	roleAll                   = "semua"
	statusWait                = "menunggu"
	statusAttend              = "hadir"
	statusDelegate            = "diwakili"
	phaseWaitingValidation    = "menunggu_validasi"
	phaseValidatedAttend      = "tervalidasi_hadir"
	phaseValidatedDelegate    = "tervalidasi_diwakili"
	phaseLocked               = "terkunci"
	phaseOngoing              = "berlangsung"
	phaseWaitingDocumentation = "menunggu_dokumentasi"
	phaseDone                 = "selesai"
	sessionCookie             = "agenda_session"
	defaultPort               = "8080"
)

type App struct {
	db          *sql.DB
	uploadDir   string
	frontendDir string
	sessions    *SessionStore
	mailer      Mailer
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
	ID            int64        `json:"id"`
	Title         string       `json:"title"`
	Location      string       `json:"location"`
	StartAt       string       `json:"start_at"`
	EndAt         string       `json:"end_at"`
	Organizer     string       `json:"organizer"`
	StaffNote     string       `json:"staff_note"`
	Status        string       `json:"status"`
	Delegate      string       `json:"delegate"`
	LeaderNote    string       `json:"leader_note"`
	ValidatedAt   *string      `json:"validated_at"`
	PulledBackAt  *string      `json:"pulled_back_at"`
	ReportNote    string       `json:"report_note"`
	CreatedAt     string       `json:"created_at"`
	Phase         string       `json:"phase"`
	DisplayStatus string       `json:"display_status"`
	CanRevise     bool         `json:"can_revise"`
	CanValidate   bool         `json:"can_validate"`
	CanPullback   bool         `json:"can_pullback"`
	CanUploadDoc  bool         `json:"can_upload_documentation"`
	IsLocked      bool         `json:"is_locked"`
	HoursUntil    int64        `json:"hours_until"`
	Invitation    *FileRecord  `json:"invitation"`
	Documentation *FileRecord  `json:"documentation"`
	Documents     []FileRecord `json:"documentation_files"`
}

type Notification struct {
	ID           int64  `json:"id"`
	Audience     string `json:"audience"`
	AgendaID     *int64 `json:"agenda_id"`
	Title        string `json:"title"`
	Body         string `json:"body"`
	EmailSent    bool   `json:"email_sent"`
	EmailMessage string `json:"email_message"`
	CreatedAt    string `json:"created_at"`
}

type EmailDelivery struct {
	Attempted bool   `json:"attempted"`
	Sent      bool   `json:"sent"`
	Message   string `json:"message"`
}

type EmailOption struct {
	Body            string
	Attachments     []EmailAttachment
	RelatedAgendaID int64
}

type AgendaCreateResponse struct {
	Agenda      Agenda        `json:"agenda"`
	EmailStatus EmailDelivery `json:"email_status"`
}

type agendaScanner interface {
	Scan(dest ...any) error
}

func canRevise(startAt time.Time) bool {
	return time.Until(startAt) >= 24*time.Hour
}

func canUploadDocumentation(endAt time.Time) bool {
	return !time.Now().Before(endAt)
}

func canValidateAgenda(status string, startAt time.Time) bool {
	if !time.Now().Before(startAt) {
		return false
	}
	if status == statusWait {
		return true
	}
	return canRevise(startAt)
}

func agendaLifecycle(decision string, startAt, endAt time.Time, hasDocumentation bool) (string, string) {
	now := time.Now()
	if hasDocumentation {
		return phaseDone, "Selesai"
	}
	if decision == statusWait {
		if time.Until(startAt) < 24*time.Hour {
			return phaseLocked, "Agenda Terkunci"
		}
		return phaseWaitingValidation, "Menunggu Validasi"
	}
	if !now.Before(endAt) {
		return phaseWaitingDocumentation, "Menunggu Dokumentasi"
	}
	if !now.Before(startAt) && now.Before(endAt) {
		return phaseOngoing, "Sedang Berlangsung"
	}
	if time.Until(startAt) < 24*time.Hour {
		return phaseLocked, "Agenda Terkunci"
	}
	if decision == statusAttend {
		return phaseValidatedAttend, "Tervalidasi - Hadir"
	}
	if decision == statusDelegate {
		return phaseValidatedDelegate, "Tervalidasi - Diwakili"
	}
	return phaseWaitingValidation, "Menunggu Validasi"
}
