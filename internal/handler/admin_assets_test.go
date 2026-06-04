package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgtype"

	"github.com/anjanvikas/portfolio-api/internal/store"
)

// fakeAssetQ records the params it was called with and returns canned rows.
type fakeAssetQ struct {
	upserted store.UpsertAssetParams
	rows     []store.Asset
	deleted  pgtype.UUID
}

func (f *fakeAssetQ) UpsertAsset(_ context.Context, arg store.UpsertAssetParams) (store.Asset, error) {
	f.upserted = arg
	return store.Asset{
		ID:        pgtype.UUID{Bytes: [16]byte{1}, Valid: true},
		Filename:  arg.Filename,
		MimeType:  arg.MimeType,
		SizeBytes: arg.SizeBytes,
		R2Key:     arg.R2Key,
		Width:     arg.Width,
		Height:    arg.Height,
		CreatedAt: pgtype.Timestamptz{Time: time.Date(2026, 5, 30, 0, 0, 0, 0, time.UTC), Valid: true},
	}, nil
}

func (f *fakeAssetQ) ListAssets(context.Context) ([]store.Asset, error) { return f.rows, nil }

func (f *fakeAssetQ) SoftDeleteAsset(_ context.Context, id pgtype.UUID) error {
	f.deleted = id
	return nil
}

// stubPresigner is a deterministic assetPresigner for handler tests.
type stubPresigner struct{ base string }

func (s stubPresigner) PresignPutObject(key, contentType string, _ time.Duration) (string, error) {
	return "https://upload.example/" + key + "?ct=" + contentType + "&sig=abc", nil
}
func (s stubPresigner) PublicURL(key string) string { return s.base + "/" + key }
func (s stubPresigner) KeyFromPublicURL(raw string) (string, bool) {
	prefix := s.base + "/"
	if !strings.HasPrefix(raw, prefix) {
		return "", false
	}
	return strings.TrimPrefix(raw, prefix), true
}

func newAssetHandler(q assetQueries) *AdminAssets {
	h := NewAdminAssets(q)
	h.Presigner = stubPresigner{base: "https://assets.example.com"}
	return h
}

func TestPresign_ReturnsUploadAndPublicURLs(t *testing.T) {
	h := newAssetHandler(&fakeAssetQ{})
	body := strings.NewReader(`{"filename":"My Photo.png","content_type":"image/png"}`)
	rr := httptest.NewRecorder()
	h.Presign(rr, httptest.NewRequest(http.MethodPost, "/api/v1/admin/assets/presign", body))

	if rr.Code != http.StatusOK {
		t.Fatalf("status: got %d want 200; body=%s", rr.Code, rr.Body.String())
	}
	var got presignResponse
	if err := json.NewDecoder(rr.Body).Decode(&got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if !strings.HasPrefix(got.Key, "assets/") || !strings.HasSuffix(got.Key, "-My-Photo.png") {
		t.Errorf("key: got %q (want assets/<rand>-My-Photo.png)", got.Key)
	}
	if !strings.Contains(got.UploadURL, got.Key) {
		t.Errorf("upload url should contain key: %q", got.UploadURL)
	}
	if got.PublicURL != "https://assets.example.com/"+got.Key {
		t.Errorf("public url: got %q", got.PublicURL)
	}
}

func TestPresign_MissingFilename(t *testing.T) {
	h := newAssetHandler(&fakeAssetQ{})
	rr := httptest.NewRecorder()
	h.Presign(rr, httptest.NewRequest(http.MethodPost, "/x", strings.NewReader(`{"filename":"  "}`)))
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("status: got %d want 400", rr.Code)
	}
}

func TestPresign_Unconfigured503(t *testing.T) {
	h := NewAdminAssets(&fakeAssetQ{}) // no presigner
	rr := httptest.NewRecorder()
	h.Presign(rr, httptest.NewRequest(http.MethodPost, "/x", strings.NewReader(`{"filename":"a.png"}`)))
	if rr.Code != http.StatusServiceUnavailable {
		t.Fatalf("status: got %d want 503", rr.Code)
	}
}

func TestRegister_DerivesKeyAndUpserts(t *testing.T) {
	q := &fakeAssetQ{}
	h := newAssetHandler(q)
	body := `{"filename":"photo.png","url":"https://assets.example.com/assets/x1-photo.png","type":"image/png","size":2048}`
	rr := httptest.NewRecorder()
	h.Register(rr, httptest.NewRequest(http.MethodPost, "/api/v1/admin/assets", strings.NewReader(body)))

	if rr.Code != http.StatusCreated {
		t.Fatalf("status: got %d want 201; body=%s", rr.Code, rr.Body.String())
	}
	if q.upserted.R2Key != "assets/x1-photo.png" {
		t.Errorf("derived key: got %q", q.upserted.R2Key)
	}
	if q.upserted.SizeBytes != 2048 || q.upserted.MimeType != "image/png" {
		t.Errorf("upsert params: %+v", q.upserted)
	}
	var dto assetDTO
	json.NewDecoder(rr.Body).Decode(&dto)
	if dto.URL != "https://assets.example.com/assets/x1-photo.png" {
		t.Errorf("dto url: got %q", dto.URL)
	}
}

func TestRegister_RejectsForeignURL(t *testing.T) {
	h := newAssetHandler(&fakeAssetQ{})
	body := `{"filename":"photo.png","url":"https://evil.example.org/assets/x.png","type":"image/png","size":10}`
	rr := httptest.NewRecorder()
	h.Register(rr, httptest.NewRequest(http.MethodPost, "/x", strings.NewReader(body)))
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("status: got %d want 400; body=%s", rr.Code, rr.Body.String())
	}
}

func TestList_MapsRowsToDTO(t *testing.T) {
	q := &fakeAssetQ{rows: []store.Asset{{
		ID:        pgtype.UUID{Bytes: [16]byte{9}, Valid: true},
		Filename:  "cover.jpg",
		MimeType:  "image/jpeg",
		SizeBytes: 5000,
		R2Key:     "assets/abc-cover.jpg",
		CreatedAt: pgtype.Timestamptz{Time: time.Now(), Valid: true},
	}}}
	h := newAssetHandler(q)
	rr := httptest.NewRecorder()
	h.List(rr, httptest.NewRequest(http.MethodGet, "/api/v1/admin/assets", nil))

	if rr.Code != http.StatusOK {
		t.Fatalf("status: got %d want 200", rr.Code)
	}
	var out []assetDTO
	json.NewDecoder(rr.Body).Decode(&out)
	if len(out) != 1 || out[0].URL != "https://assets.example.com/assets/abc-cover.jpg" {
		t.Fatalf("unexpected list output: %+v", out)
	}
}

func TestAssetKey_Sanitises(t *testing.T) {
	k := assetKey(`C:\Users\me\My Résumé (final)!.PNG`)
	if !strings.HasPrefix(k, "assets/") {
		t.Fatalf("missing prefix: %q", k)
	}
	suffix := strings.TrimPrefix(k, "assets/")
	// random8 + "-" + sanitised basename, only safe chars remain.
	for _, r := range suffix {
		ok := (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '.' || r == '_' || r == '-'
		if !ok {
			t.Fatalf("unsafe char %q in key %q", r, k)
		}
	}
}
