package handler

import (
	"errors"
	"io"
	"log/slog"
	"net/http"
	"strings"

	"github.com/anjanvikas2001/portfolio-api/internal/service"
)

// maxDocxBytes caps the multipart upload accepted by the docx converter. Real
// blog drafts are well under this; the cap keeps a runaway upload from being
// buffered into memory.
const maxDocxBytes = 25 << 20 // 25 MiB

// AdminConvert serves POST /api/v1/admin/convert/docx (SCRUM-67): accept a docx
// upload, convert it to markdown via the injected converter, and return the
// text. The file is never persisted (the converter writes a transient temp file
// it deletes); only the resulting markdown is returned.
type AdminConvert struct {
	Converter service.DocxConverter
}

// NewAdminConvert wires the handler against a docx converter.
func NewAdminConvert(c service.DocxConverter) *AdminConvert {
	return &AdminConvert{Converter: c}
}

type convertResponse struct {
	Markdown string `json:"markdown"`
}

// Docx handles the multipart upload. The file field is "file". Returns 400 on a
// missing/oversized/non-docx upload, 503 when pandoc isn't installed, 500 on a
// conversion failure.
func (a *AdminConvert) Docx(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, maxDocxBytes)
	if err := r.ParseMultipartForm(maxDocxBytes); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "file too large or malformed upload"})
		return
	}

	file, header, err := r.FormFile("file")
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "missing 'file' upload"})
		return
	}
	defer file.Close()

	if !strings.HasSuffix(strings.ToLower(header.Filename), ".docx") {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "only .docx files are supported"})
		return
	}

	data, err := io.ReadAll(file)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "could not read upload"})
		return
	}
	if len(data) == 0 {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "uploaded file is empty"})
		return
	}

	markdown, err := a.Converter.ConvertDocx(r.Context(), data)
	if err != nil {
		if errors.Is(err, service.ErrConverterUnavailable) {
			writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "docx conversion is unavailable (pandoc not installed)"})
			return
		}
		slog.ErrorContext(r.Context(), "docx conversion", slog.String("error", err.Error()))
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "conversion failed"})
		return
	}
	writeJSON(w, http.StatusOK, convertResponse{Markdown: markdown})
}
