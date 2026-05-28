// Seed program for local development.
//
// Idempotent: every insert is an ON CONFLICT DO UPDATE, so re-running this
// program never creates duplicates. Run with `make seed` (which loads .env)
// or `DATABASE_URL=... go run ./cmd/seed`.
package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/jackc/pgx/v5/pgtype"

	"github.com/anjanvikas2001/portfolio-api/internal/config"
	"github.com/anjanvikas2001/portfolio-api/internal/store"
)

func main() {
	if err := config.LoadDotEnv(".env"); err != nil {
		log.Fatalf("dotenv: %v", err)
	}
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		log.Fatal("DATABASE_URL not set")
	}

	ctx := context.Background()
	pool, err := store.Connect(ctx, dsn)
	if err != nil {
		log.Fatalf("connect: %v", err)
	}
	defer pool.Close()

	q := store.New(pool)

	if err := seedAll(ctx, q); err != nil {
		log.Fatalf("seed: %v", err)
	}

	fmt.Println("seed: ok")
}

func seedAll(ctx context.Context, q *store.Queries) error {
	// ---- Profile ----------------------------------------------------------
	if _, err := q.UpsertProfile(ctx, store.UpsertProfileParams{
		Name:      "Anjan Vikas Reddy",
		Headline:  "Software engineer — building neo-brutalist things on the side",
		Bio:       "Backend-leaning generalist. I write Go, pick fights with CSS, and ship.",
		Location:  "Hyderabad, IN",
		Email:     "anjanvikas2001@gmail.com",
		ResumeUrl: text("https://example.com/resume.pdf"),
		AvatarUrl: text("https://example.com/avatar.jpg"),
	}); err != nil {
		return fmt.Errorf("profile: %w", err)
	}

	// ---- Social links (3) -------------------------------------------------
	socials := []store.UpsertSocialLinkParams{
		{Name: "github", Url: "https://github.com/anjanvikas2001", SortOrder: 0},
		{Name: "linkedin", Url: "https://www.linkedin.com/in/anjanvikas/", SortOrder: 1},
		{Name: "twitter", Url: "https://twitter.com/anjanvikas", SortOrder: 2},
	}
	for _, s := range socials {
		if _, err := q.UpsertSocialLink(ctx, s); err != nil {
			return fmt.Errorf("social %s: %w", s.Name, err)
		}
	}

	// ---- Experience (2) ---------------------------------------------------
	experiences := []store.UpsertExperienceParams{
		{
			Company:     "Mealmind",
			Role:        "Founding engineer",
			Location:    "Remote",
			StartDate:   date(2024, 6, 1),
			EndDate:     pgtype.Date{}, // null = current
			Description: "Built the recipe pipeline, recommendation engine, and admin tooling end-to-end.",
			SortOrder:   1,
		},
		{
			Company:     "Acme Corp",
			Role:        "Software engineer",
			Location:    "Bengaluru, IN",
			StartDate:   date(2022, 7, 1),
			EndDate:     date(2024, 5, 31),
			Description: "Owned the billing service rewrite from Rails to Go. Cut p99 latency by 60%.",
			SortOrder:   0,
		},
	}
	for _, e := range experiences {
		if _, err := q.UpsertExperience(ctx, e); err != nil {
			return fmt.Errorf("experience %s: %w", e.Company, err)
		}
	}

	// ---- Tags (2) ---------------------------------------------------------
	tagGo, err := q.UpsertTag(ctx, store.UpsertTagParams{Slug: "go", Name: "Go"})
	if err != nil {
		return fmt.Errorf("tag go: %w", err)
	}
	tagDesign, err := q.UpsertTag(ctx, store.UpsertTagParams{Slug: "design-systems", Name: "Design systems"})
	if err != nil {
		return fmt.Errorf("tag design: %w", err)
	}

	// ---- Blog series + 2 posts in series + 1 standalone ------------------
	series, err := q.UpsertBlogSeries(ctx, store.UpsertBlogSeriesParams{
		Slug:        "building-a-portfolio",
		Name:        "Building this portfolio",
		Description: "A live diary of designing and shipping this site.",
	})
	if err != nil {
		return fmt.Errorf("series: %w", err)
	}

	post1, err := q.UpsertBlogPost(ctx, store.UpsertBlogPostParams{
		Slug:        "part-1-design-tokens",
		Title:       "Part 1 — Locking the design tokens before writing a line of code",
		Excerpt:     "Why I drew the type ramp and color system in Figma first, and the trade-offs that fell out.",
		Body:        seriesPart1Body,
		SeriesID:    series.ID,
		SeriesOrder: int4(1),
		PublishedAt: ts(2026, 5, 10),
	})
	if err != nil {
		return fmt.Errorf("post 1: %w", err)
	}

	post2, err := q.UpsertBlogPost(ctx, store.UpsertBlogPostParams{
		Slug:        "part-2-stack-decisions",
		Title:       "Part 2 — Picking Go + sqlc over GORM",
		Excerpt:     "Compile-time SQL safety beats ORM ergonomics, even at 10k req/day.",
		Body:        seriesPart2Body,
		SeriesID:    series.ID,
		SeriesOrder: int4(2),
		PublishedAt: ts(2026, 5, 17),
	})
	if err != nil {
		return fmt.Errorf("post 2: %w", err)
	}

	standalone, err := q.UpsertBlogPost(ctx, store.UpsertBlogPostParams{
		Slug:        "scratch-notes-pgx-v5",
		Title:       "Scratch notes — pgx v5 nullable types",
		Excerpt:     "pgtype.Text is fine. Stop wrapping it in *string.",
		Body:        standalonePostBody,
		PublishedAt: ts(2026, 5, 22),
	})
	if err != nil {
		return fmt.Errorf("standalone post: %w", err)
	}

	// Tag the posts.
	for _, link := range []store.LinkBlogPostTagParams{
		{BlogPostID: post1.ID, TagID: tagDesign.ID},
		{BlogPostID: post2.ID, TagID: tagGo.ID},
		{BlogPostID: standalone.ID, TagID: tagGo.ID},
	} {
		if err := q.LinkBlogPostTag(ctx, link); err != nil {
			return fmt.Errorf("link blog tag: %w", err)
		}
	}

	// ---- Project (1, with all three markdown sections) -------------------
	project, err := q.UpsertProject(ctx, store.UpsertProjectParams{
		Slug:         "mealmind",
		Title:        "Mealmind",
		Tagline:      "A spec-driven recipe engine that cooks for you.",
		Summary:      "Pipeline that ingests recipes, normalises ingredients, and recommends meals based on your pantry.",
		BodyOverview: projectOverview,
		BodyWhyBuilt: projectWhyBuilt,
		BodyLearning: projectLearning,
		RepoUrl:      text("https://github.com/anjanvikas2001/mealmind"),
		LiveUrl:      text("https://mealmind.app"),
		SortOrder:    0,
		PublishedAt:  ts(2026, 4, 1),
	})
	if err != nil {
		return fmt.Errorf("project: %w", err)
	}
	for _, link := range []store.LinkProjectTagParams{
		{ProjectID: project.ID, TagID: tagGo.ID},
		{ProjectID: project.ID, TagID: tagDesign.ID},
	} {
		if err := q.LinkProjectTag(ctx, link); err != nil {
			return fmt.Errorf("link project tag: %w", err)
		}
	}

	// ---- Testimonials (2) -------------------------------------------------
	testimonials := []store.UpsertTestimonialParams{
		{
			AuthorName:    "Priya Sharma",
			AuthorRole:    "Eng manager",
			AuthorCompany: "Acme Corp",
			Quote:         "Anjan shipped the billing rewrite ahead of schedule with zero regressions. Rare combo.",
			SortOrder:     0,
		},
		{
			AuthorName:    "Marcus Lee",
			AuthorRole:    "CTO",
			AuthorCompany: "Mealmind",
			Quote:         "He owns problems end-to-end, from the migration file to the cover image.",
			SortOrder:     1,
		},
	}
	for _, t := range testimonials {
		if _, err := q.UpsertTestimonial(ctx, t); err != nil {
			return fmt.Errorf("testimonial %s: %w", t.AuthorName, err)
		}
	}

	return nil
}

// ---- small helpers for pgtype null wrappers --------------------------------

func text(s string) pgtype.Text {
	return pgtype.Text{String: s, Valid: true}
}

func int4(n int32) pgtype.Int4 {
	return pgtype.Int4{Int32: n, Valid: true}
}

func date(y int, m time.Month, d int) pgtype.Date {
	return pgtype.Date{Time: time.Date(y, m, d, 0, 0, 0, 0, time.UTC), Valid: true}
}

func ts(y int, m time.Month, d int) pgtype.Timestamptz {
	return pgtype.Timestamptz{Time: time.Date(y, m, d, 12, 0, 0, 0, time.UTC), Valid: true}
}

// ---- inline markdown bodies (real headings, code, mermaid, lists) ---------

const seriesPart1Body = "# Locking the tokens first\n\n" +
	"Before writing any code I drew the entire type ramp, color palette, and\n" +
	"spacing scale in Figma. The tokens are the contract; the components are\n" +
	"the implementation.\n\n" +
	"## Why this order matters\n\n" +
	"If you build components first, every token decision is a retrofit and\n" +
	"a refactor. Tokens first means components are born already on-brand.\n\n" +
	"- Pick the type ramp before the headline component exists.\n" +
	"- Pick the color tokens before the button has a hover state.\n" +
	"- Pick the spacing scale before the page has a header.\n\n" +
	"```ts\n" +
	"export const ramp = {\n" +
	"  display: '64/72',\n" +
	"  h1: '48/56',\n" +
	"  h2: '32/40',\n" +
	"} as const;\n" +
	"```\n\n" +
	"## The flow\n\n" +
	"```mermaid\n" +
	"flowchart LR\n" +
	"  tokens --> components --> pages --> ship\n" +
	"```\n\n" +
	"That's the whole skeleton.\n"

const seriesPart2Body = "# Picking Go + sqlc over GORM\n\n" +
	"GORM is faster to start with. sqlc is faster to live with.\n\n" +
	"## The trade I made\n\n" +
	"1. Compile-time SQL safety — broken queries fail `make sqlc`, not at 2am.\n" +
	"2. No N+1 surprises hiding behind `.Preload(...)`.\n" +
	"3. Better signal for a Go-shaped portfolio.\n\n" +
	"```go\n" +
	"q := store.New(pool)\n" +
	"profile, err := q.GetProfile(ctx)\n" +
	"```\n\n" +
	"## Where ORM ergonomics still win\n\n" +
	"Dynamic filters. sqlc generates static functions; if the WHERE clause\n" +
	"shifts at runtime you fall back to a hand-rolled `pool.Query` call.\n" +
	"For a portfolio CRUD that almost never happens.\n\n" +
	"```mermaid\n" +
	"sequenceDiagram\n" +
	"  participant H as Handler\n" +
	"  participant S as Store\n" +
	"  participant D as Postgres\n" +
	"  H->>S: q.UpsertProject(...)\n" +
	"  S->>D: INSERT ... ON CONFLICT\n" +
	"  D-->>S: row\n" +
	"  S-->>H: Project\n" +
	"```\n"

const standalonePostBody = "# pgx v5 nullable types\n\n" +
	"`pgtype.Text` is not a footgun. It already models NULL correctly and\n" +
	"Scan handles it without ceremony.\n\n" +
	"- `*string` is one more indirection for nothing.\n" +
	"- JSON marshalling of `pgtype.Text` emits the inner string or `null`.\n" +
	"- The zero value is the NULL value. That's the whole API.\n\n" +
	"```go\n" +
	"var t pgtype.Text\n" +
	"t = pgtype.Text{String: \"hi\", Valid: true}\n" +
	"```\n"

const projectOverview = "## Overview\n\n" +
	"Mealmind is a spec-driven recipe engine. You describe what's in the\n" +
	"pantry and what you want to eat this week; it returns a shopping list\n" +
	"and a plan.\n\n" +
	"- Ingest from RSS, structured-data sites, and a manual editor.\n" +
	"- Normalise ingredients via a controlled vocabulary.\n" +
	"- Recommend meals using a small cosine-similarity engine.\n"

const projectWhyBuilt = "## Why I built this\n\n" +
	"I was tired of meal-planning apps that assumed an empty fridge. The\n" +
	"interesting constraint is *what you already have*, not *what you could\n" +
	"buy*. Mealmind starts from your pantry.\n"

const projectLearning = "## Learning journey\n\n" +
	"Three things I learned that I would not have predicted:\n\n" +
	"1. Ingredient normalisation is 60% of the work and 0% of the demo.\n" +
	"2. A small embedding model beat the bigger one because latency mattered.\n" +
	"3. The admin editor was the highest-leverage UI surface in the whole app.\n"
