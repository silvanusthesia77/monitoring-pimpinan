package main

import (
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

func (app *App) downloadFile(w http.ResponseWriter, r *http.Request) {
	if _, ok := app.currentUser(r); !ok {
		writeError(w, http.StatusUnauthorized, "Belum login")
		return
	}

	id, ok := parseFileID(w, r.URL.Path)
	if !ok {
		return
	}

	var storedName, originalName, mimeType string
	err := app.db.QueryRow("SELECT stored_name, original_name, mime_type FROM files WHERE id = ?", id).
		Scan(&storedName, &originalName, &mimeType)
	if err != nil {
		http.NotFound(w, r)
		return
	}

	w.Header().Set("Content-Type", mimeType)
	w.Header().Set("Content-Disposition", `attachment; filename="`+safeDownloadName(originalName)+`"`)
	http.ServeFile(w, r, filepath.Join(app.uploadDir, storedName))
}

func (app *App) saveUpload(fh *multipart.FileHeader, userID int64) (int64, error) {
	src, err := fh.Open()
	if err != nil {
		return 0, err
	}
	defer src.Close()

	storedName, err := storedFileName(fh.Filename)
	if err != nil {
		return 0, err
	}

	targetPath := filepath.Join(app.uploadDir, storedName)
	dst, err := os.Create(targetPath)
	if err != nil {
		return 0, err
	}
	defer dst.Close()

	if _, err := io.Copy(dst, src); err != nil {
		return 0, err
	}

	result, err := app.db.Exec(
		"INSERT INTO files (original_name, stored_name, mime_type, size_bytes, uploaded_by) VALUES (?, ?, ?, ?, ?)",
		fh.Filename, storedName, contentType(fh), fh.Size, userID,
	)
	if err != nil {
		return 0, err
	}
	return result.LastInsertId()
}

func firstUploadedFile(r *http.Request, field string) (*multipart.FileHeader, bool) {
	files := r.MultipartForm.File[field]
	if len(files) == 0 {
		return nil, false
	}
	return files[0], true
}

func parseFileID(w http.ResponseWriter, path string) (int64, bool) {
	idText := strings.TrimPrefix(path, "/api/files/")
	idText = strings.TrimSuffix(idText, "/download")
	id, err := strconv.ParseInt(strings.Trim(idText, "/"), 10, 64)
	if err != nil {
		badRequest(w, "ID file tidak valid")
		return 0, false
	}
	return id, true
}

func storedFileName(originalName string) (string, error) {
	token, err := randomToken()
	if err != nil {
		return "", err
	}
	return token + filepath.Ext(originalName), nil
}

func contentType(fh *multipart.FileHeader) string {
	mimeType := fh.Header.Get("Content-Type")
	if mimeType == "" {
		return "application/octet-stream"
	}
	return mimeType
}

func safeDownloadName(name string) string {
	return strings.ReplaceAll(name, `"`, "")
}
