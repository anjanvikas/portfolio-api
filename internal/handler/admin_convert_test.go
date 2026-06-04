package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/anjanvikas/portfolio-api/internal/service"
)

type fakeConverter struct {
	out string
	err error
}

func (f fakeConverter) ConvertDocx(context.Context, []byte) (string, error) { return f.out, f.err }

// multipartDocx builds a multipart body with one "file" field.
func multipartDocx(t *testing.T, filename string, content []byte) (*bytes.Buffer, string) {
	t.Helper()
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	fw, err := mw.CreateFormFile("file", filename)
	if err != nil {
		t.Fatal(err)
	}
	fw.Write(content)
	mw.Close()
	return &buf, mw.FormDataContentType()
}

func TestConvertDocx_Success(t *testing.T) {
	h := NewAdminConvert(fakeConverter{out: "# Heading\n\nbody"})
	body, ct := multipartDocx(t, "draft.docx", []byte("PK fake-docx-bytes"))
	req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/convert/docx", body)
	req.Header.Set("Content-Type", ct)
	rr := httptest.NewRecorder()
	h.Docx(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status: got %d want 200; body=%s", rr.Code, rr.Body.String())
	}
	var got convertResponse
	json.NewDecoder(rr.Body).Decode(&got)
	if got.Markdown != "# Heading\n\nbody" {
		t.Errorf("markdown: got %q", got.Markdown)
	}
}

func TestConvertDocx_MissingFile(t *testing.T) {
	h := NewAdminConvert(fakeConverter{})
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	mw.WriteField("other", "x")
	mw.Close()
	req := httptest.NewRequest(http.MethodPost, "/x", &buf)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	rr := httptest.NewRecorder()
	h.Docx(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("status: got %d want 400", rr.Code)
	}
}

func TestConvertDocx_WrongExtension(t *testing.T) {
	h := NewAdminConvert(fakeConverter{})
	body, ct := multipartDocx(t, "draft.txt", []byte("hello"))
	req := httptest.NewRequest(http.MethodPost, "/x", body)
	req.Header.Set("Content-Type", ct)
	rr := httptest.NewRecorder()
	h.Docx(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("status: got %d want 400", rr.Code)
	}
}

func TestConvertDocx_Unavailable503(t *testing.T) {
	h := NewAdminConvert(fakeConverter{err: service.ErrConverterUnavailable})
	body, ct := multipartDocx(t, "draft.docx", []byte("PK fake"))
	req := httptest.NewRequest(http.MethodPost, "/x", body)
	req.Header.Set("Content-Type", ct)
	rr := httptest.NewRecorder()
	h.Docx(rr, req)
	if rr.Code != http.StatusServiceUnavailable {
		t.Fatalf("status: got %d want 503", rr.Code)
	}
}
