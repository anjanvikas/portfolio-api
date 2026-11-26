package main

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5/pgtype"

	"github.com/anjanvikas/portfolio-api/internal/store"
)

type fakeQ struct {
	posts []store.ListPublishedPostOGTargetsRow
	saved map[string]string // id byte -> url
	sets  int
}

func (f *fakeQ) GetProfile(context.Context) (store.Profile, error) {
	return store.Profile{Name: "Test Author"}, nil
}
func (f *fakeQ) ListPublishedPostOGTargets(context.Context) ([]store.ListPublishedPostOGTargetsRow, error) {
	return f.posts, nil
}
func (f *fakeQ) SetBlogPostOGImageKeepUpdatedAt(_ context.Context, a store.SetBlogPostOGImageKeepUpdatedAtParams) error {
	f.sets++
	f.saved[string(rune('0'+a.ID.Bytes[0]))] = a.OgImageUrl
	return nil
}

type fakeR struct{ authors []string }

func (r *fakeR) RenderPost(title, author, site string) ([]byte, error) {
	r.authors = append(r.authors, author)
	return []byte("png-" + title), nil
}

type fakeUp struct {
	keys, types []string
	failKey     string
}

func (u *fakeUp) PutObject(_ context.Context, key, ct string, _ []byte) (string, error) {
	if key == u.failKey {
		return "", errors.New("boom")
	}
	u.keys = append(u.keys, key)
	u.types = append(u.types, ct)
	return "https://r2.example/" + key, nil
}

func posts() []store.ListPublishedPostOGTargetsRow {
	mk := func(b byte, slug string) store.ListPublishedPostOGTargetsRow {
		return store.ListPublishedPostOGTargetsRow{ID: pgtype.UUID{Bytes: [16]byte{b}, Valid: true}, Slug: slug, Title: "T " + slug}
	}
	return []store.ListPublishedPostOGTargetsRow{mk(1, "a"), mk(2, "b"), mk(3, "c")}
}

func TestRegenerate(t *testing.T) {
	t.Run("all posts uploaded and persisted with cache-buster", func(t *testing.T) {
		q, up := &fakeQ{posts: posts(), saved: map[string]string{}}, &fakeUp{}
		var out bytes.Buffer
		done, failed, err := regenerate(context.Background(), q, &fakeR{}, up, regenOptions{RunUnix: 1764150000}, &out)
		if err != nil || done != 3 || failed != 0 {
			t.Fatalf("done=%d failed=%d err=%v", done, failed, err)
		}
		if strings.Join(up.keys, ",") != "og/posts/a.png,og/posts/b.png,og/posts/c.png" || up.types[0] != "image/png" {
			t.Errorf("keys %v types %v", up.keys, up.types)
		}
		if got := q.saved["1"]; got != "https://r2.example/og/posts/a.png?v=1764150000" {
			t.Errorf("saved %q", got)
		}
	})
	t.Run("one failing upload does not stop others", func(t *testing.T) {
		q, up := &fakeQ{posts: posts(), saved: map[string]string{}}, &fakeUp{failKey: "og/posts/b.png"}
		var out bytes.Buffer
		done, failed, err := regenerate(context.Background(), q, &fakeR{}, up, regenOptions{RunUnix: 1}, &out)
		if err != nil || done != 2 || failed != 1 || q.sets != 2 {
			t.Fatalf("done=%d failed=%d sets=%d err=%v", done, failed, q.sets, err)
		}
		if !strings.Contains(out.String(), "b -> FAILED") {
			t.Errorf("output %q", out.String())
		}
	})
	t.Run("dry-run writes files, no upload, no db write", func(t *testing.T) {
		dir := t.TempDir()
		q := &fakeQ{posts: posts(), saved: map[string]string{}}
		var out bytes.Buffer
		done, failed, err := regenerate(context.Background(), q, &fakeR{}, nil, regenOptions{DryRun: true, OutDir: dir}, &out)
		if err != nil || done != 3 || failed != 0 || q.sets != 0 {
			t.Fatalf("done=%d failed=%d sets=%d err=%v", done, failed, q.sets, err)
		}
		if _, err := os.Stat(filepath.Join(dir, "a.png")); err != nil {
			t.Error(err)
		}
	})
	t.Run("slug limits to one post", func(t *testing.T) {
		q, up := &fakeQ{posts: posts(), saved: map[string]string{}}, &fakeUp{}
		var out bytes.Buffer
		done, _, err := regenerate(context.Background(), q, &fakeR{}, up, regenOptions{Slug: "c"}, &out)
		if err != nil || done != 1 || len(up.keys) != 1 || up.keys[0] != "og/posts/c.png" {
			t.Fatalf("done=%d keys=%v err=%v", done, up.keys, err)
		}
	})
	t.Run("unknown slug errors", func(t *testing.T) {
		q := &fakeQ{posts: posts(), saved: map[string]string{}}
		if _, _, err := regenerate(context.Background(), q, &fakeR{}, &fakeUp{}, regenOptions{Slug: "zzz"}, &bytes.Buffer{}); err == nil {
			t.Fatal("expected error")
		}
	})
}
