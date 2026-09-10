package main

import (
	"context"
	"database/sql"
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	_ "github.com/go-sql-driver/mysql"
	"golang.org/x/crypto/bcrypt"
)

const (
	defaultDSN         = "root:root@tcp(127.0.0.1:3306)/agenda_monitor?parseTime=true&multiStatements=true"
	defaultUploadDir   = "./uploads"
	defaultFrontendDir = "../frontend"
	defaultSchemaFile  = "../database/schema.sql"
	defaultPassword    = "agenda123"
	defaultStaffEmail  = "staf@sorsel.go.id"
	defaultLeaderEmail = "silvanusthesia1@gmail.com"
)

func newApp() (*App, error) {
	db, err := sql.Open("mysql", env("MYSQL_DSN", defaultDSN))
	if err != nil {
		return nil, err
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if err := waitForDB(ctx, db); err != nil {
		return nil, err
	}
	if err := applySchema(db); err != nil {
		return nil, err
	}
	if err := migrateNotificationEmailMessage(db); err != nil {
		return nil, err
	}
	if err := migrateNotificationAgendaID(db); err != nil {
		return nil, err
	}
	if err := migrateDefaultLeaderEmail(db); err != nil {
		return nil, err
	}
	if err := seedUsers(db); err != nil {
		return nil, err
	}
	if err := removeAdminRole(db); err != nil {
		return nil, err
	}
	if err := seedDemoAgendas(db); err != nil {
		return nil, err
	}

	uploadDir := env("UPLOAD_DIR", defaultUploadDir)
	if err := os.MkdirAll(uploadDir, 0755); err != nil {
		return nil, err
	}

	return &App{
		db:          db,
		uploadDir:   uploadDir,
		frontendDir: env("FRONTEND_DIR", defaultFrontendDir),
		sessions:    NewSessionStore(),
		mailer:      newMailerFromEnv(),
	}, nil
}

func (app *App) routes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/login", app.login)
	mux.HandleFunc("/api/logout", app.logout)
	mux.HandleFunc("/api/me", app.me)
	mux.HandleFunc("/api/agendas", app.agendas)
	mux.HandleFunc("/api/agendas/", app.agendaAction)
	mux.HandleFunc("/api/notifications", app.notifications)
	mux.HandleFunc("/api/files/", app.downloadFile)
	mux.HandleFunc("/", app.frontend)
	return mux
}

func (app *App) frontend(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	if r.URL.Path == "/" || filepath.Ext(r.URL.Path) == "" {
		http.ServeFile(w, r, filepath.Join(app.frontendDir, "index.html"))
		return
	}

	cleanPath := strings.TrimPrefix(filepath.Clean(r.URL.Path), string(filepath.Separator))
	http.ServeFile(w, r, filepath.Join(app.frontendDir, cleanPath))
}

func waitForDB(ctx context.Context, db *sql.DB) error {
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()

	for {
		if err := db.PingContext(ctx); err == nil {
			return nil
		}
		select {
		case <-ctx.Done():
			return errors.New("database tidak dapat dihubungi")
		case <-ticker.C:
		}
	}
}

func applySchema(db *sql.DB) error {
	schema, err := os.ReadFile(env("SCHEMA_FILE", defaultSchemaFile))
	if err != nil {
		return err
	}
	_, err = db.Exec(string(schema))
	return err
}

func migrateNotificationEmailMessage(db *sql.DB) error {
	var columnName string
	err := db.QueryRow(`
		SELECT COLUMN_NAME
		FROM INFORMATION_SCHEMA.COLUMNS
		WHERE TABLE_SCHEMA = DATABASE()
			AND TABLE_NAME = 'notifications'
			AND COLUMN_NAME = 'email_message'`).Scan(&columnName)
	if err == nil {
		return nil
	}
	if err != sql.ErrNoRows {
		return err
	}
	_, err = db.Exec("ALTER TABLE notifications ADD COLUMN email_message VARCHAR(255) NULL AFTER email_sent")
	return err
}

func migrateNotificationAgendaID(db *sql.DB) error {
	var columnName string
	err := db.QueryRow(`
		SELECT COLUMN_NAME
		FROM INFORMATION_SCHEMA.COLUMNS
		WHERE TABLE_SCHEMA = DATABASE()
			AND TABLE_NAME = 'notifications'
			AND COLUMN_NAME = 'agenda_id'`).Scan(&columnName)
	if err == nil {
		return nil
	}
	if err != sql.ErrNoRows {
		return err
	}
	_, err = db.Exec("ALTER TABLE notifications ADD COLUMN agenda_id BIGINT NULL AFTER audience")
	return err
}

func seedUsers(db *sql.DB) error {
	users := []User{
		{Name: "Staf Protokol", Email: defaultStaffEmail, Role: roleStaff, Position: "Staf Protokol"},
		{Name: "Pimpinan Daerah", Email: defaultLeaderEmail, Role: roleLeader, Position: "Pimpinan Kabupaten Sorong Selatan"},
	}

	for _, user := range users {
		hash, err := bcrypt.GenerateFromPassword([]byte(defaultPassword), bcrypt.DefaultCost)
		if err != nil {
			return err
		}
		_, err = db.Exec(
			`INSERT INTO users (name, email, password_hash, role, position)
			VALUES (?, ?, ?, ?, ?)
			ON DUPLICATE KEY UPDATE
				name = VALUES(name),
				role = VALUES(role),
				position = VALUES(position)`,
			user.Name, user.Email, string(hash), user.Role, user.Position,
		)
		if err != nil {
			return err
		}
	}
	return nil
}

func migrateDefaultLeaderEmail(db *sql.DB) error {
	legacyEmails := []string{"pimpinan@sorsel.go.id", "sergiodyego45@gmail.com"}

	var leaderID int64
	err := db.QueryRow("SELECT id FROM users WHERE email = ?", defaultLeaderEmail).Scan(&leaderID)
	if err == sql.ErrNoRows {
		for _, legacyEmail := range legacyEmails {
			result, err := db.Exec(
				"UPDATE users SET email = ? WHERE email = ? AND role = ?",
				defaultLeaderEmail, legacyEmail, roleLeader,
			)
			if err != nil {
				return err
			}
			if rows, _ := result.RowsAffected(); rows > 0 {
				break
			}
		}
		err = db.QueryRow("SELECT id FROM users WHERE email = ?", defaultLeaderEmail).Scan(&leaderID)
	}
	if err == sql.ErrNoRows {
		return nil
	}
	if err != nil {
		return err
	}

	for _, legacyEmail := range legacyEmails {
		var legacyID int64
		err := db.QueryRow("SELECT id FROM users WHERE email = ? AND role = ?", legacyEmail, roleLeader).Scan(&legacyID)
		if err == sql.ErrNoRows {
			continue
		}
		if err != nil {
			return err
		}
		if legacyID == leaderID {
			continue
		}
		if _, err := db.Exec(
			"UPDATE files SET uploaded_by = ? WHERE uploaded_by = ?",
			leaderID, legacyID,
		); err != nil {
			return err
		}
		if _, err := db.Exec(
			"UPDATE agendas SET created_by = ? WHERE created_by = ?",
			leaderID, legacyID,
		); err != nil {
			return err
		}
		if _, err := db.Exec(
			"UPDATE agendas SET validated_by = ? WHERE validated_by = ?",
			leaderID, legacyID,
		); err != nil {
			return err
		}
		if _, err := db.Exec(
			"DELETE FROM users WHERE id = ?",
			legacyID,
		); err != nil {
			return err
		}
	}

	return nil
}

func removeAdminRole(db *sql.DB) error {
	var staffID int64
	if err := db.QueryRow("SELECT id FROM users WHERE email = ?", defaultStaffEmail).Scan(&staffID); err != nil {
		return err
	}

	if _, err := db.Exec("ALTER TABLE users MODIFY role ENUM('admin', 'staf', 'pimpinan') NOT NULL"); err != nil {
		return err
	}

	if _, err := db.Exec(`
		UPDATE files
		SET uploaded_by = ?
		WHERE uploaded_by IN (SELECT id FROM users WHERE role = 'admin')`, staffID); err != nil {
		return err
	}
	if _, err := db.Exec(`
		UPDATE agendas
		SET created_by = ?
		WHERE created_by IN (SELECT id FROM users WHERE role = 'admin')`, staffID); err != nil {
		return err
	}
	if _, err := db.Exec(`
		UPDATE agendas
		SET validated_by = NULL
		WHERE validated_by IN (SELECT id FROM users WHERE role = 'admin')`); err != nil {
		return err
	}
	if _, err := db.Exec("DELETE FROM users WHERE role = 'admin'"); err != nil {
		return err
	}
	if _, err := db.Exec("DROP TABLE IF EXISTS registration_codes"); err != nil {
		return err
	}
	_, err := db.Exec("ALTER TABLE users MODIFY role ENUM('staf', 'pimpinan') NOT NULL")
	return err
}

func seedDemoAgendas(db *sql.DB) error {
	var staffID, leaderID int64
	if err := db.QueryRow("SELECT id FROM users WHERE email = ?", defaultStaffEmail).Scan(&staffID); err != nil {
		if err == sql.ErrNoRows {
			return nil
		}
		return err
	}
	if err := db.QueryRow("SELECT id FROM users WHERE email = ?", defaultLeaderEmail).Scan(&leaderID); err != nil {
		if err == sql.ErrNoRows {
			return nil
		}
		return err
	}

	demos := []struct {
		title      string
		location   string
		organizer  string
		staffNote  string
		status     string
		delegate   string
		leaderNote string
		startSQL   string
		endSQL     string
	}{
		{
			title:     "Demo Agenda - Menunggu Validasi",
			location:  "Aula Kantor Bupati Sorong Selatan",
			organizer: "Bagian Protokol",
			staffNote: "Agenda contoh untuk validasi pimpinan.",
			status:    statusWait,
			startSQL:  "DATE_ADD(NOW(), INTERVAL 7 DAY)",
			endSQL:    "DATE_ADD(NOW(), INTERVAL 7 DAY) + INTERVAL 2 HOUR",
		},
		{
			title:      "Demo Agenda - Diwakili",
			location:   "Ruang Rapat Sekretariat Daerah",
			organizer:  "Sekretariat Daerah",
			staffNote:  "Agenda contoh yang sudah divalidasi.",
			status:     statusDelegate,
			delegate:   "Sekretaris Daerah",
			leaderNote: "Pimpinan diwakili oleh Sekretaris Daerah.",
			startSQL:   "DATE_ADD(NOW(), INTERVAL 9 DAY)",
			endSQL:     "DATE_ADD(NOW(), INTERVAL 9 DAY) + INTERVAL 2 HOUR",
		},
		{
			title:      "Demo Agenda - Hadir Sendiri",
			location:   "Gedung Serbaguna Teminabuan",
			organizer:  "Panitia Kegiatan Daerah",
			staffNote:  "Agenda contoh pimpinan hadir sendiri.",
			status:     statusAttend,
			leaderNote: "Pimpinan hadir langsung pada kegiatan.",
			startSQL:   "DATE_ADD(NOW(), INTERVAL 12 DAY)",
			endSQL:     "DATE_ADD(NOW(), INTERVAL 12 DAY) + INTERVAL 2 HOUR",
		},
		{
			title:      "Demo Agenda - Siap Upload Laporan",
			location:   "Aula Distrik Teminabuan",
			organizer:  "Bagian Pemerintahan",
			staffNote:  "Agenda contoh yang sudah selesai dan siap diupload dokumentasi.",
			status:     statusAttend,
			leaderNote: "Kegiatan sudah divalidasi dan menunggu dokumentasi.",
			startSQL:   "DATE_SUB(NOW(), INTERVAL 2 DAY)",
			endSQL:     "DATE_SUB(NOW(), INTERVAL 2 DAY) + INTERVAL 2 HOUR",
		},
	}

	for _, demo := range demos {
		validatedBy := sql.NullInt64{}
		validatedAt := "NULL"
		if demo.status != statusWait {
			validatedBy = sql.NullInt64{Int64: leaderID, Valid: true}
			validatedAt = "NOW()"
		}

		query := `
			INSERT INTO agendas
				(title, location, start_at, end_at, organizer, staff_note, status, delegate, leader_note, validated_by, validated_at, created_by)
			SELECT ?, ?, ` + demo.startSQL + `, ` + demo.endSQL + `, ?, ?, ?, NULLIF(?, ''), NULLIF(?, ''), ?, ` + validatedAt + `, ?
			WHERE NOT EXISTS (SELECT 1 FROM agendas WHERE title = ?)`
		if _, err := db.Exec(query, demo.title, demo.location, demo.organizer, demo.staffNote, demo.status, demo.delegate, demo.leaderNote, validatedBy, staffID, demo.title); err != nil {
			return err
		}
	}
	return nil
}
