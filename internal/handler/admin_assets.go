package handler

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"path"
	"regexp"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/anjanvikas/portfolio-api/internal/store"
)

// assetQueries is the slice of store.Queries the asset registry needs, declared
// as an interface so the handler is unit-testable with a fake (the pattern every
// other handler in this package follows).
type assetQueries interface {
	UpsertAsset(ctx context.Context, arg store.UpsertAssetParams) (store.Asset, error)
	ListAssets(ctx context.Context) ([]store.Asset, error)
	SoftDeleteAsset(ctx context.Context, id pgtype.UUID) error
}

// assetPresigner is the subset of *service.R2Presigner the asset pipeline uses.
// Kept as an interface so the handler can be tested without real R2 creds.
type assetPresigner interface {
	PresignPutObject(key, contentType string, expiry time.Duration) (string, error)
	PublicURL(key string) string
	KeyFromPublicURL(rawURL string) (string, bool)
}

// AdminAssets serves the upload pipeline under /api/v1/admin/assets (SCRUM-67):
// presign a direct browser→R2 PUT, register the uploaded object in the asset
// table, and list the registry for the image picker / asset page. The Go server
// never carries the file bytes — the browser PUTs straight to R2.
//
// Presigner is nil when R2 isn't configured; every endpoint then returns 503 so
// the admin gets an actionable error instead of a broken upload.
type AdminAssets struct {
	Q         assetQueries
	Presigner assetPresigner
}

// NewAdminAssets wires the handler against the live sqlc queries. The presigner
// is set separately by the router (and may be nil).
func NewAdminAssets(q assetQueries) *AdminAssets {
	return &AdminAssets{Q: q}
}

const presignTTL = 15 * time.Minute

// ---- request / response DTOs ---------------------------------------------

type presignRequest struct {
	Filename    string `json:"filename"`
	ContentType string `json:"content_type"`
}

type presignResponse struct {
	UploadURL string `json:"upload_url"`
	PublicURL string `json:"public_url"`
	Key       string `json:"key"`
}

type registerRequest struct {
	Filename string `json:"filename"`
	URL      string `json:"url"`
	Type     string `json:"type"`
	Size     int64  `json:"size"`
	Width    *int32 `json:"width"`
	Height   *int32 `json:"height"`
}

// assetDTO is one row of the asset registry as the admin UI consumes it.
type assetDTO struct {
	ID        string  `json:"id"`
	Filename  string  `json:"filename"`
	URL       string  `json:"url"`
	Type      string  `json:"type"`
	Size      int64   `json:"size"`
	Width     *int32  `json:"width"`
	Height    *int32  `json:"height"`
	CreatedAt *string `json:"created_at"`
}

// ---- presign -------------------------------------------------------------

// Presign handles POST /api/v1/admin/assets/presign. Generates a unique object
// key, returns a presigned PUT URL the browser uploads to directly, and the
// stable public URL the object will live at once uploaded.
func (a *AdminAssets) Presign(w http.ResponseWriter, r *http.Request) {
	if a.Presigner == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "uploads are not configured (R2 unavailable)"})
		return
	}
	var req presignRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		return
	}
	if strings.TrimSpace(req.Filename) == "" {
		writeJSON(w, http.StatusBadRequest, map[string]any{"errors": map[string]string{"filename": "filename is required"}})
		return
	}

	key := assetKey(req.Filename)
	// Bind the declared Content-Type into the SigV4 signature so the browser's
	// PUT (which sets the same header) matches. Without this, R2 returns 403
	// SignatureDoesNotMatch the moment the client sends any Content-Type.
	uploadURL, err := a.Presigner.PresignPutObject(key, strings.TrimSpace(req.ContentType), presignTTL)
	if err != nil {
		slog.ErrorContext(r.Context(), "presign put", slog.String("error", err.Error()))
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal error"})
		return
	}
	writeJSON(w, http.StatusOK, presignResponse{
		UploadURL: uploadURL,
		PublicURL: a.Presigner.PublicURL(key),
		Key:       key,
	})
}

// ---- register ------------------------------------------------------------

// Register handles POST /api/v1/admin/assets. Called by the browser after a
// successful direct upload to record the object in the asset table. The key is
// recovered from the public URL (which must match the configured bucket base),
// so a forged or foreign URL is rejected rather than stored.
func (a *AdminAssets) Register(w http.ResponseWriter, r *http.Request) {
	if a.Presigner == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "uploads are not configured (R2 unavailable)"})
		return
	}
	var req registerRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		return
	}

	fieldErrs := map[string]string{}
	if strings.TrimSpace(req.Filename) == "" {
		fieldErrs["filename"] = "filename is required"
	}
	key, ok := a.Presigner.KeyFromPublicURL(strings.TrimSpace(req.URL))
	if !ok {
		fieldErrs["url"] = "url does not match the configured asset bucket"
	}
	if req.Size < 0 {
		fieldErrs["size"] = "size must be non-negative"
	}
	if len(fieldErrs) > 0 {
		writeJSON(w, http.StatusBadRequest, map[string]any{"errors": fieldErrs})
		return
	}

	asset, err := a.Q.UpsertAsset(r.Context(), store.UpsertAssetParams{
		Filename:  strings.TrimSpace(req.Filename),
		MimeType:  strings.TrimSpace(req.Type),
		SizeBytes: req.Size,
		R2Key:     key,
		Width:     int4Ptr(req.Width),
		Height:    int4Ptr(req.Height),
	})
	if err != nil {
		slog.ErrorContext(r.Context(), "register asset", slog.String("error", err.Error()))
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal error"})
		return
	}
	writeJSON(w, http.StatusCreated, a.assetToDTO(asset))
}

// ---- list ----------------------------------------------------------------

// List handles GET /api/v1/admin/assets. Returns the full (non-deleted) asset
// registry, newest first, for the asset page table and the editor image picker.
func (a *AdminAssets) List(w http.ResponseWriter, r *http.Request) {
	if a.Presigner == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "uploads are not configured (R2 unavailable)"})
		return
	}
	rows, err := a.Q.ListAssets(r.Context())
	if err != nil {
		slog.ErrorContext(r.Context(), "list assets", slog.String("error", err.Error()))
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal error"})
		return
	}
	out := make([]assetDTO, 0, len(rows))
	for _, row := range rows {
		out = append(out, a.assetToDTO(row))
	}
	writeJSON(w, http.StatusOK, out)
}

// ---- delete --------------------------------------------------------------

// Delete handles DELETE /api/v1/admin/assets/{id}. Soft-deletes the registry
// row (the R2 object is left in place; cleaning the bucket is a separate
// concern). Idempotent: a missing/already-deleted id still returns 204.
func (a *AdminAssets) Delete(w http.ResponseWriter, r *http.Request) {
	id, ok := parsePathUUID(w, r)
	if !ok {
		return
	}
	if err := a.Q.SoftDeleteAsset(r.Context(), id); err != nil {
		slog.ErrorContext(r.Context(), "soft delete asset", slog.String("error", err.Error()))
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal error"})
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// ---- helpers -------------------------------------------------------------

func (a *AdminAssets) assetToDTO(row store.Asset) assetDTO {
	created := isoDate(row.CreatedAt)
	dto := assetDTO{
		ID:       uuidString(row.ID),
		Filename: row.Filename,
		URL:      a.Presigner.PublicURL(row.R2Key),
		Type:     row.MimeType,
		Size:     row.SizeBytes,
	}
	if created != "" {
		dto.CreatedAt = &created
	}
	if row.Width.Valid {
		w := row.Width.Int32
		dto.Width = &w
	}
	if row.Height.Valid {
		h := row.Height.Int32
		dto.Height = &h
	}
	return dto
}

func int4Ptr(v *int32) pgtype.Int4 {
	if v == nil {
		return pgtype.Int4{}
	}
	return pgtype.Int4{Int32: *v, Valid: true}
}

var unsafeKeyChars = regexp.MustCompile(`[^a-zA-Z0-9._-]+`)

// assetKey builds a collision-free object key under assets/ from a user
// filename. The basename is sanitised (path components stripped, unsafe runes
// replaced with '-') and prefixed with a short random segment so re-uploading
// the same name never overwrites an existing object.
func assetKey(filename string) string {
	base := path.Base(filepathSlash(filename))
	base = strings.TrimSpace(base)
	if base == "" || base == "." || base == "/" {
		base = "file"
	}
	base = unsafeKeyChars.ReplaceAllString(base, "-")
	base = strings.Trim(base, "-")
	if base == "" {
		base = "file"
	}
	return "assets/" + uuid.NewString()[:8] + "-" + base
}

// filepathSlash normalises Windows-style backslashes to forward slashes so
// path.Base strips a "C:\\Users\\…\\photo.png" basename correctly too.
func filepathSlash(p string) string {
	return strings.ReplaceAll(p, `\`, "/")
}
