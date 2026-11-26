package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/anjanvikas/portfolio-api/internal/store"
)

// regenQueries is the slice of *store.Queries the regeneration loop needs.
type regenQueries interface {
	GetProfile(ctx context.Context) (store.Profile, error)
	ListPublishedPostOGTargets(ctx context.Context) ([]store.ListPublishedPostOGTargetsRow, error)
	SetBlogPostOGImageKeepUpdatedAt(ctx context.Context, arg store.SetBlogPostOGImageKeepUpdatedAtParams) error
}

type ogRenderer interface {
	RenderPost(title, authorName, siteURL string) ([]byte, error)
}

type ogUploader interface {
	PutObject(ctx context.Context, key, contentType string, body []byte) (string, error)
}

type regenOptions struct {
	DryRun  bool
	OutDir  string // dry-run output directory
	Slug    string // limit to one post when non-empty
	SiteURL string
	RunUnix int64 // cache-bust value appended as ?v=
}

// regenerate renders every published post card (or one with opts.Slug), uploads
// it and persists the URL. Individual failures are reported and skipped; the
// returned count is the number of failures.
func regenerate(ctx context.Context, q regenQueries, r ogRenderer, up ogUploader, opts regenOptions, out io.Writer) (done, failed int, err error) {
	prof, err := q.GetProfile(ctx)
	if err != nil {
		return 0, 0, fmt.Errorf("get profile: %w", err)
	}
	posts, err := q.ListPublishedPostOGTargets(ctx)
	if err != nil {
		return 0, 0, fmt.Errorf("list posts: %w", err)
	}
	if opts.Slug != "" {
		var one []store.ListPublishedPostOGTargetsRow
		for _, p := range posts {
			if p.Slug == opts.Slug {
				one = append(one, p)
			}
		}
		if len(one) == 0 {
			return 0, 0, fmt.Errorf("no published post with slug %q", opts.Slug)
		}
		posts = one
	}

	for _, p := range posts {
		if perr := regenOne(ctx, q, r, up, prof.Name, p, opts, out); perr != nil {
			failed++
			fmt.Fprintf(out, "%s -> FAILED: %v\n", p.Slug, perr)
			continue
		}
		done++
	}
	fmt.Fprintf(out, "done: %d ok, %d failed\n", done, failed)
	return done, failed, nil
}

func regenOne(ctx context.Context, q regenQueries, r ogRenderer, up ogUploader, author string, p store.ListPublishedPostOGTargetsRow, opts regenOptions, out io.Writer) error {
	png, err := r.RenderPost(p.Title, author, opts.SiteURL)
	if err != nil {
		return fmt.Errorf("render: %w", err)
	}
	if opts.DryRun {
		if err := os.MkdirAll(opts.OutDir, 0o755); err != nil {
			return err
		}
		path := filepath.Join(opts.OutDir, p.Slug+".png")
		if err := os.WriteFile(path, png, 0o644); err != nil {
			return err
		}
		fmt.Fprintf(out, "%s -> %s (dry-run)\n", p.Slug, path)
		return nil
	}
	if up == nil {
		return errors.New("no uploader configured")
	}
	publicURL, err := up.PutObject(ctx, "og/posts/"+p.Slug+".png", "image/png", png)
	if err != nil {
		return fmt.Errorf("upload: %w", err)
	}
	stored := fmt.Sprintf("%s?v=%d", publicURL, opts.RunUnix)
	if err := q.SetBlogPostOGImageKeepUpdatedAt(ctx, store.SetBlogPostOGImageKeepUpdatedAtParams{OgImageUrl: stored, ID: p.ID}); err != nil {
		return fmt.Errorf("persist: %w", err)
	}
	fmt.Fprintf(out, "%s -> %s\n", p.Slug, stored)
	return nil
}
