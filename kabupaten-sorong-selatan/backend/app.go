package main

import (
	"context"
	"database/sql"
	"errors"
	"net/http"
	"os"
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
	if err := seedUsers(db); err != nil {
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
	mux.Handle("/", http.FileServer(http.Dir(app.frontendDir)))
	return mux
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

func seedUsers(db *sql.DB) error {
	var count int
	if err := db.QueryRow("SELECT COUNT(*) FROM users").Scan(&count); err != nil {
		return err
	}
	if count > 0 {
		return nil
	}

	users := []User{
		{Name: "Admin Staf", Email: "staf@sorsel.go.id", Role: roleStaff, Position: "Staf Protokol"},
		{Name: "Pimpinan Daerah", Email: "pimpinan@sorsel.go.id", Role: roleLeader, Position: "Pimpinan Kabupaten Sorong Selatan"},
	}
	for _, user := range users {
		hash, err := bcrypt.GenerateFromPassword([]byte(defaultPassword), bcrypt.DefaultCost)
		if err != nil {
			return err
		}
		_, err = db.Exec(
			"INSERT INTO users (name, email, password_hash, role, position) VALUES (?, ?, ?, ?, ?)",
			user.Name, user.Email, string(hash), user.Role, user.Position,
		)
		if err != nil {
			return err
		}
	}
	return nil
}
