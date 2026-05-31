package service

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

// R2Presigner builds AWS SigV4 presigned GET/PUT URLs for objects in a
// Cloudflare R2 bucket. R2 exposes an S3-compatible API, so the standard SigV4
// query-param signing scheme applies; R2 ignores the region and expects the
// literal "auto". Path-style addressing (<endpoint>/<bucket>/<key>) is used.
//
// We hand-roll the signature rather than pull in the aws-sdk-go-v2 dependency
// tree — a presigned URL is a small, well-specified algorithm.
//
// publicBase, when set, is the bucket's public base URL (an r2.dev domain or a
// custom domain bound to the bucket); PublicURL composes it with an object key
// to form the stable, unauthenticated URL stored in the asset registry.
type R2Presigner struct {
	accessKey  string
	secretKey  string
	bucket     string
	endpoint   string // e.g. https://<account-id>.r2.cloudflarestorage.com
	publicBase string // e.g. https://assets.example.com (no trailing slash)
	region     string
	now        func() time.Time
}

// NewR2Presigner constructs a presigner for the given bucket and account
// endpoint. publicBase may be empty when no public domain is configured.
func NewR2Presigner(accessKey, secretKey, bucket, endpoint, publicBase string) *R2Presigner {
	return &R2Presigner{
		accessKey:  accessKey,
		secretKey:  secretKey,
		bucket:     bucket,
		endpoint:   strings.TrimRight(endpoint, "/"),
		publicBase: strings.TrimRight(publicBase, "/"),
		region:     "auto",
		now:        time.Now,
	}
}

const presignService = "s3"

// PublicURL returns the stable, unauthenticated URL for the object at key,
// using the configured public base domain. Falls back to the path-style R2
// endpoint URL when no public base is set (won't be publicly reachable, but
// keeps the value well-formed in dev).
func (p *R2Presigner) PublicURL(key string) string {
	if p.publicBase != "" {
		return p.publicBase + "/" + escapePath(key)
	}
	return p.endpoint + "/" + p.bucket + "/" + escapePath(key)
}

// KeyFromPublicURL recovers the object key from a public URL produced by
// PublicURL, by stripping the known base prefix. Returns ("", false) when the
// URL doesn't match the configured base (so the caller can reject a forged or
// foreign URL rather than register a bogus key).
func (p *R2Presigner) KeyFromPublicURL(rawURL string) (string, bool) {
	prefix := p.publicBase
	if prefix == "" {
		prefix = p.endpoint + "/" + p.bucket
	}
	prefix += "/"
	if !strings.HasPrefix(rawURL, prefix) {
		return "", false
	}
	escaped := strings.TrimPrefix(rawURL, prefix)
	key, err := url.PathUnescape(escaped)
	if err != nil || key == "" {
		return "", false
	}
	return key, true
}

// PresignGetObject returns a URL that grants temporary, unauthenticated GET
// access to the object at key, valid for the expiry window.
func (p *R2Presigner) PresignGetObject(key string, expiry time.Duration) (string, error) {
	return p.presign(http.MethodGet, key, expiry)
}

// PresignPutObject returns a URL that grants temporary, unauthenticated PUT
// access to the object at key, valid for the expiry window. The browser uploads
// the file bytes directly to this URL (the Go server never carries them). Only
// the host header is signed, so the client may send any Content-Type.
func (p *R2Presigner) PresignPutObject(key string, expiry time.Duration) (string, error) {
	return p.presign(http.MethodPut, key, expiry)
}

// presign builds a SigV4 query-string-signed URL for the given HTTP method and
// object key. Shared by the GET (download) and PUT (upload) flows: the signing
// algorithm is identical, only the HTTP verb in the canonical request changes.
func (p *R2Presigner) presign(method, key string, expiry time.Duration) (string, error) {
	u, err := url.Parse(p.endpoint)
	if err != nil {
		return "", fmt.Errorf("parse R2 endpoint: %w", err)
	}
	host := u.Host

	// Path-style canonical URI: /<bucket>/<key>, each path segment escaped but
	// the separating slashes preserved.
	canonicalURI := "/" + p.bucket + "/" + escapePath(key)

	now := p.now().UTC()
	amzDate := now.Format("20060102T150405Z")
	dateStamp := now.Format("20060102")
	scope := strings.Join([]string{dateStamp, p.region, presignService, "aws4_request"}, "/")

	q := url.Values{}
	q.Set("X-Amz-Algorithm", "AWS4-HMAC-SHA256")
	q.Set("X-Amz-Credential", p.accessKey+"/"+scope)
	q.Set("X-Amz-Date", amzDate)
	q.Set("X-Amz-Expires", strconv.Itoa(int(expiry.Seconds())))
	q.Set("X-Amz-SignedHeaders", "host")

	canonicalHeaders := "host:" + host + "\n"
	signedHeaders := "host"
	payloadHash := "UNSIGNED-PAYLOAD"

	canonicalRequest := strings.Join([]string{
		method,
		canonicalURI,
		q.Encode(), // sorted, RFC3986-escaped; signature not yet present
		canonicalHeaders,
		signedHeaders,
		payloadHash,
	}, "\n")

	stringToSign := strings.Join([]string{
		"AWS4-HMAC-SHA256",
		amzDate,
		scope,
		hashHex(canonicalRequest),
	}, "\n")

	signingKey := deriveSigningKey(p.secretKey, dateStamp, p.region, presignService)
	signature := hex.EncodeToString(hmacSHA256(signingKey, stringToSign))

	q.Set("X-Amz-Signature", signature)
	return p.endpoint + canonicalURI + "?" + q.Encode(), nil
}

// escapePath URI-escapes each segment of an object key, leaving the slashes
// that separate them intact (S3 single-encodes the path).
func escapePath(key string) string {
	segments := strings.Split(key, "/")
	for i, s := range segments {
		segments[i] = url.PathEscape(s)
	}
	return strings.Join(segments, "/")
}

func deriveSigningKey(secret, dateStamp, region, service string) []byte {
	kDate := hmacSHA256([]byte("AWS4"+secret), dateStamp)
	kRegion := hmacSHA256(kDate, region)
	kService := hmacSHA256(kRegion, service)
	return hmacSHA256(kService, "aws4_request")
}

func hmacSHA256(key []byte, data string) []byte {
	h := hmac.New(sha256.New, key)
	h.Write([]byte(data))
	return h.Sum(nil)
}

func hashHex(s string) string {
	sum := sha256.Sum256([]byte(s))
	return hex.EncodeToString(sum[:])
}
