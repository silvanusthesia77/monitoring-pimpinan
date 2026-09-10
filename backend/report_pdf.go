package main

import (
	"database/sql"
	"fmt"
	"net/http"
	"path/filepath"
	"strings"
	"time"

	"github.com/phpdave11/gofpdf"
)

type storedReportFile struct {
	FileRecord
	StoredName string
}

func (app *App) downloadReportPDF(w http.ResponseWriter, r *http.Request, id int64) {
	if r.Method != http.MethodGet {
		methodNotAllowed(w)
		return
	}

	agenda, err := app.getAgenda(id)
	if err != nil {
		writeError(w, http.StatusNotFound, "Agenda tidak ditemukan")
		return
	}
	if agenda.Documentation == nil {
		writeError(w, http.StatusConflict, "Berita acara belum tersedia karena dokumentasi belum diupload")
		return
	}

	documentationFiles, err := app.getStoredDocumentationFiles(agenda.ID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Gagal membaca dokumentasi laporan")
		return
	}
	if len(documentationFiles) == 0 {
		documentation, err := app.getStoredReportFile(agenda.Documentation.ID)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "Gagal membaca dokumentasi laporan")
			return
		}
		documentationFiles = append(documentationFiles, documentation)
	}

	pdf, err := buildReportPDF(app.uploadDir, agenda, documentationFiles)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Gagal membuat PDF berita acara")
		return
	}

	filename := "berita-acara-" + slugify(agenda.Title) + ".pdf"
	w.Header().Set("Content-Type", "application/pdf")
	w.Header().Set("Content-Disposition", `attachment; filename="`+safeDownloadName(filename)+`"`)
	if err := pdf.Output(w); err != nil {
		writeError(w, http.StatusInternalServerError, "Gagal mengirim PDF berita acara")
		return
	}
}

func (app *App) getStoredReportFile(id int64) (storedReportFile, error) {
	var file storedReportFile
	var created time.Time
	err := app.db.QueryRow(`
		SELECT id, original_name, stored_name, mime_type, size_bytes, created_at
		FROM files
		WHERE id = ?`, id).
		Scan(&file.ID, &file.OriginalName, &file.StoredName, &file.MimeType, &file.SizeBytes, &created)
	if err != nil {
		if err == sql.ErrNoRows {
			return storedReportFile{}, err
		}
		return storedReportFile{}, err
	}
	file.CreatedAt = created.Format(time.RFC3339)
	return file, nil
}

func buildReportPDF(uploadDir string, agenda Agenda, documentationFiles []storedReportFile) (*gofpdf.Fpdf, error) {
	pdf := gofpdf.New("P", "mm", "A4", "")
	pdf.SetMargins(18, 16, 18)
	pdf.SetAutoPageBreak(true, 18)
	pdf.AddPage()
	tr := pdf.UnicodeTranslatorFromDescriptor("")

	pdf.SetFont("Arial", "B", 12)
	pdf.CellFormat(0, 7, tr("PEMERINTAH KABUPATEN SORONG SELATAN"), "", 1, "C", false, 0, "")
	pdf.SetFont("Arial", "B", 15)
	pdf.CellFormat(0, 9, tr("BERITA ACARA KEGIATAN"), "", 1, "C", false, 0, "")
	pdf.SetFont("Arial", "", 9)
	pdf.CellFormat(0, 5, tr("Aplikasi Monitor Agenda Pimpinan"), "", 1, "C", false, 0, "")
	pdf.Ln(8)

	addPDFSection(pdf, tr, "Data Agenda")
	addPDFRow(pdf, tr, "Nama Kegiatan", agenda.Title)
	addPDFRow(pdf, tr, "Lokasi", agenda.Location)
	addPDFRow(pdf, tr, "Waktu", fmt.Sprintf("%s sampai %s", formatReportTime(agenda.StartAt), formatReportTime(agenda.EndAt)))
	addPDFRow(pdf, tr, "Penyelenggara", agenda.Organizer)
	addPDFRow(pdf, tr, "Keterangan Staf", agenda.StaffNote)

	pdf.Ln(3)
	addPDFSection(pdf, tr, "Validasi Pimpinan")
	addPDFRow(pdf, tr, "Status Kehadiran", attendanceText(agenda))
	addPDFRow(pdf, tr, "Keterangan Pimpinan", fallbackText(agenda.LeaderNote, "Tidak ada keterangan."))
	if agenda.ValidatedAt != nil {
		addPDFRow(pdf, tr, "Waktu Validasi", formatReportTime(*agenda.ValidatedAt))
	}

	pdf.Ln(3)
	addPDFSection(pdf, tr, "Laporan Kegiatan")
	addPDFRow(pdf, tr, "Ringkasan Laporan", agenda.ReportNote)
	addPDFRow(pdf, tr, "Jumlah Lampiran", fmt.Sprintf("%d file dokumentasi", len(documentationFiles)))
	if len(documentationFiles) > 0 {
		addPDFRow(pdf, tr, "Tanggal Upload", formatReportTime(documentationFiles[0].CreatedAt))
	}

	for index, documentation := range documentationFiles {
		if err := addDocumentationImage(pdf, tr, filepath.Join(uploadDir, documentation.StoredName), index+1, documentation.OriginalName); err != nil {
			addPDFRow(pdf, tr, fmt.Sprintf("Lampiran %d", index+1), "Gambar dokumentasi gagal dimuat ke PDF.")
		}
	}

	pdf.Ln(6)
	pdf.SetFont("Arial", "", 9)
	pdf.MultiCell(0, 5, tr("Dokumen ini dibuat otomatis oleh Aplikasi Monitor Agenda Pimpinan Kabupaten Sorong Selatan."), "", "L", false)

	return pdf, pdf.Error()
}

func addPDFSection(pdf *gofpdf.Fpdf, tr func(string) string, title string) {
	pdf.SetFont("Arial", "B", 11)
	pdf.SetFillColor(241, 243, 245)
	pdf.CellFormat(0, 8, tr(title), "1", 1, "L", true, 0, "")
}

func addPDFRow(pdf *gofpdf.Fpdf, tr func(string) string, label, value string) {
	pdf.SetFont("Arial", "B", 9)
	pdf.CellFormat(43, 7, tr(label), "1", 0, "L", false, 0, "")
	pdf.SetFont("Arial", "", 9)
	pdf.MultiCell(0, 7, tr(fallbackText(value, "-")), "1", "L", false)
}

func addDocumentationImage(pdf *gofpdf.Fpdf, tr func(string) string, imagePath string, index int, filename string) error {
	pdf.Ln(4)
	pdf.SetFont("Arial", "B", 10)
	pdf.CellFormat(0, 7, tr(fmt.Sprintf("Lampiran %d", index)), "", 1, "L", false, 0, "")
	pdf.SetFont("Arial", "", 8)
	pdf.CellFormat(0, 5, tr(filename), "", 1, "L", false, 0, "")

	options := gofpdf.ImageOptions{ReadDpi: true}
	info := pdf.RegisterImageOptions(imagePath, options)
	if info == nil {
		return pdf.Error()
	}

	pageWidth, pageHeight := pdf.GetPageSize()
	left, top, right, bottom := pdf.GetMargins()
	maxWidth := pageWidth - left - right
	maxHeight := pageHeight - top - bottom - 20
	width, height := maxWidth, maxWidth*info.Height()/info.Width()
	if height > maxHeight {
		height = maxHeight
		width = maxHeight * info.Width() / info.Height()
	}
	if pdf.GetY()+height > pageHeight-bottom {
		pdf.AddPage()
	}
	x := left + (maxWidth-width)/2
	pdf.ImageOptions(imagePath, x, pdf.GetY(), width, height, false, options, 0, "")
	pdf.Ln(height + 3)
	return pdf.Error()
}

func attendanceText(agenda Agenda) string {
	if agenda.Status == statusDelegate {
		return "Diwakili oleh " + fallbackText(agenda.Delegate, "-")
	}
	return "Pimpinan hadir sendiri"
}

func formatReportTime(value string) string {
	parsed, err := parseInputTime(value)
	if err != nil {
		return value
	}
	return parsed.Local().Format("02 Jan 2006 15:04")
}

func fallbackText(value, fallback string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return fallback
	}
	return value
}

func isPDFImage(mimeType string) bool {
	switch strings.ToLower(strings.TrimSpace(mimeType)) {
	case "image/jpeg", "image/jpg", "image/png", "image/gif":
		return true
	default:
		return false
	}
}

func slugify(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	var builder strings.Builder
	lastDash := false
	for _, char := range value {
		if (char >= 'a' && char <= 'z') || (char >= '0' && char <= '9') {
			builder.WriteRune(char)
			lastDash = false
			continue
		}
		if !lastDash {
			builder.WriteByte('-')
			lastDash = true
		}
	}
	slug := strings.Trim(builder.String(), "-")
	if slug == "" {
		return "agenda"
	}
	return slug
}
