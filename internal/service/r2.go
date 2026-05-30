package service

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"time"
)

// R2Presigner builds AWS SigV4 presigned GET URLs for objects in a Cloudflare
// R2 bucket. R2 exposes an S3-compatible API, so the standard SigV4 query-param
// signing scheme applies; R2 ignores the region and expects the literal
// "auto". Path-style addressing (<endpoint>/<bucket>/<key>) is used.
//
// We hand-roll the signature rather than pull in the aws-sdk-go-v2 dependency
// tree — a presigned GET is a small, well-specified algorithm.
type R2Presigner struct {
	accessKey string
	secretKey string
	bucket    string
	endpoint  string // e.g. https://<account-id>.r2.cloudflarestorage.com
	region    string
	now       func() time.Time
}

// NewR2Presigner constructs a presigner for the given bucket and account
// endpoint.
func NewR2Presigner(accessKey, secretKey, bucket, endpoint string) *R2Presigner {
	return &R2Presigner{
		accessKey: accessKey,
		secretKey: secretKey,
		bucket:    bucket,
		endpoint:  strings.TrimRight(endpoint, "/"),
		region:    "auto",
		now:       time.Now,
	}
}

const presignService = "s3"

// PresignGetObject returns a URL that grants temporary, unauthenticated GET
// access to the object at key, valid for the expiry window.
func (p *R2Presigner) PresignGetObject(key string, expiry time.Duration) (string, error) {
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
		"GET",
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
