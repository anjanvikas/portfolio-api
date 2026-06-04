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

	"github.com/anjanvikas/portfolio-api/internal/config"
	"github.com/anjanvikas/portfolio-api/internal/content"
	"github.com/anjanvikas/portfolio-api/internal/store"
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
		Headline:  "Senior Software Engineer @ Wells Fargo",
		Bio:       "I build LLM-powered backend systems — distributed pipelines, GenAI workflows, and IAM platforms at production scale. Currently shipping a 0→1 LLM Risk & Compliance platform at Wells Fargo. Go, Python, Kafka, and a lot of FastAPI.",
		Location:  "Bengaluru, KA, India",
		Email:     "anjanvikas2001@gmail.com",
		ResumeUrl: text("https://example.com/resume.pdf"),
		AvatarUrl: text("https://example.com/avatar.jpg"),
	}); err != nil {
		return fmt.Errorf("profile: %w", err)
	}

	// ---- Social links (5) -------------------------------------------------
	// Order matches the footer wireframe: GH · LI · TW · YT · LC.
	socials := []store.UpsertSocialLinkParams{
		{Name: "github", Url: "https://github.com/anjanvikas", SortOrder: 0},
		{Name: "linkedin", Url: "https://www.linkedin.com/in/anjanvikas/", SortOrder: 1},
		{Name: "twitter", Url: "https://x.com/Anjanq5ld", SortOrder: 2},
		{Name: "youtube", Url: "https://www.youtube.com/@Rewiring101", SortOrder: 3},
		{Name: "leetcode", Url: "https://leetcode.com/u/anjanvikas2001/", SortOrder: 4},
	}
	for _, s := range socials {
		if _, err := q.UpsertSocialLink(ctx, s); err != nil {
			return fmt.Errorf("social %s: %w", s.Name, err)
		}
	}

	// ---- Experience (3) ---------------------------------------------------
	// sort_order is newest-first on the /about timeline.
	experiences := []store.UpsertExperienceParams{
		{
			Company:     "Wells Fargo",
			Role:        "Senior Software Engineer",
			Location:    "Bengaluru, KA",
			StartDate:   date(2025, 10, 1),
			EndDate:     pgtype.Date{}, // null = current
			Description: "Led 0→1 development of a production-grade LLM-powered IAM Risk & Compliance platform — distributed Kafka + MongoDB pipelines, FastAPI microservices on OpenShift (K8), and async job processing achieving 10ms median submission latency and P99 < 500ms retrieval. Delivered $2.5M+ annual cost savings while leading LLD, code reviews, and a team of 4 engineers.",
			SortOrder:   2,
		},
		{
			Company:     "Wells Fargo",
			Role:        "Software Engineer",
			Location:    "Bengaluru, KA",
			StartDate:   date(2023, 8, 1),
			EndDate:     date(2025, 10, 1),
			Description: "Automated IAM workflows across enterprise platforms (SIMBA, AIMS, ART) responsible for access provisioning across the bank. Analyzed 700+ production assets, identified 34 high-impact workflows covering 33% of system traffic, and shipped scalable automation resilient to form drift. Delivered $13M in cost savings and eliminated 11 FTEs.",
			SortOrder:   1,
		},
		{
			Company:     "Mathologic",
			Role:        "Software Engineering Intern",
			Location:    "Bengaluru, KA",
			StartDate:   date(2022, 9, 1),
			EndDate:     date(2022, 12, 31),
			Description: "Solved Locomotive Shed Allocation under real-world constraints (capacity, zone preferences, locomotive type); deployed as a RESTful API in Go on internal servers.",
			SortOrder:   0,
		},
	}
	for _, e := range experiences {
		if _, err := q.UpsertExperience(ctx, e); err != nil {
			return fmt.Errorf("experience %s: %w", e.Company, err)
		}
	}

	// ---- Tags -------------------------------------------------------------
	tagGo, err := q.UpsertTag(ctx, store.UpsertTagParams{Slug: "go", Name: "Go"})
	if err != nil {
		return fmt.Errorf("tag go: %w", err)
	}
	tagDesign, err := q.UpsertTag(ctx, store.UpsertTagParams{Slug: "design-systems", Name: "Design systems"})
	if err != nil {
		return fmt.Errorf("tag design: %w", err)
	}
	tagRag, err := q.UpsertTag(ctx, store.UpsertTagParams{Slug: "rag", Name: "RAG"})
	if err != nil {
		return fmt.Errorf("tag rag: %w", err)
	}
	tagNext, err := q.UpsertTag(ctx, store.UpsertTagParams{Slug: "nextjs", Name: "Next.js"})
	if err != nil {
		return fmt.Errorf("tag nextjs: %w", err)
	}
	tagPython, err := q.UpsertTag(ctx, store.UpsertTagParams{Slug: "python", Name: "Python"})
	if err != nil {
		return fmt.Errorf("tag python: %w", err)
	}
	tagLLM, err := q.UpsertTag(ctx, store.UpsertTagParams{Slug: "llm", Name: "LLMs"})
	if err != nil {
		return fmt.Errorf("tag llm: %w", err)
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
		Slug:            "part-1-design-tokens",
		Title:           "Part 1 — Locking the design tokens before writing a line of code",
		Excerpt:         "Why I drew the type ramp and color system in Figma first, and the trade-offs that fell out.",
		Body:            seriesPart1Body,
		SeriesID:        series.ID,
		SeriesOrder:     int4(1),
		PublishedAt:     ts(2026, 5, 10),
		ReadingTimeMins: content.ReadingTimeMins(seriesPart1Body),
	})
	if err != nil {
		return fmt.Errorf("post 1: %w", err)
	}

	post2, err := q.UpsertBlogPost(ctx, store.UpsertBlogPostParams{
		Slug:            "part-2-stack-decisions",
		Title:           "Part 2 — Picking Go + sqlc over GORM",
		Excerpt:         "Compile-time SQL safety beats ORM ergonomics, even at 10k req/day.",
		Body:            seriesPart2Body,
		SeriesID:        series.ID,
		SeriesOrder:     int4(2),
		PublishedAt:     ts(2026, 5, 17),
		ReadingTimeMins: content.ReadingTimeMins(seriesPart2Body),
	})
	if err != nil {
		return fmt.Errorf("post 2: %w", err)
	}

	standalone, err := q.UpsertBlogPost(ctx, store.UpsertBlogPostParams{
		Slug:            "scratch-notes-pgx-v5",
		Title:           "Scratch notes — pgx v5 nullable types",
		Excerpt:         "pgtype.Text is fine. Stop wrapping it in *string.",
		Body:            standalonePostBody,
		PublishedAt:     ts(2026, 5, 22),
		ReadingTimeMins: content.ReadingTimeMins(standalonePostBody),
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

	// ---- Projects (3 featured, with markdown sections) -------------------
	// The homepage "featured work" strip renders up to 3 cards; seed exactly
	// that many so the section reads as designed. sort_order drives the order.
	resumeRanker, err := q.UpsertProject(ctx, store.UpsertProjectParams{
		Slug:         "ai-resume-ranker",
		Title:        "AI Resume Ranker",
		Tagline:      "3-stage GenAI pipeline for resume screening.",
		Summary:      "AI-powered resume screening platform — resume parsing, semantic similarity shortlisting with Sentence-Transformers, and deep LLM-based candidate evaluation.",
		BodyOverview: resumeRankerOverview,
		BodyWhyBuilt: resumeRankerWhyBuilt,
		BodyLearning: resumeRankerLearning,
		RepoUrl:      text("https://github.com/anjanvikas/ai-resume-ranker"),
		SortOrder:    0,
		Featured:     true,
		PublishedAt:  ts(2026, 3, 1),
	})
	if err != nil {
		return fmt.Errorf("project ai-resume-ranker: %w", err)
	}

	ragDeepDive, err := q.UpsertProject(ctx, store.UpsertProjectParams{
		Slug:         "rag-deep-dive",
		Title:        "RAG Deep Dive",
		Tagline:      "Conversational AI with hybrid search + GraphRAG.",
		Summary:      "Production-grade conversational AI with hybrid retrieval (dense + BM25 + cross-encoder reranking) across Qdrant and Neo4j, plus a transparent pipeline explainer UI.",
		BodyOverview: ragDeepDiveOverview,
		BodyWhyBuilt: ragDeepDiveWhyBuilt,
		BodyLearning: ragDeepDiveLearning,
		RepoUrl:      text("https://github.com/anjanvikas/rag-deep-dive"),
		SortOrder:    1,
		Featured:     true,
		PublishedAt:  ts(2026, 2, 1),
	})
	if err != nil {
		return fmt.Errorf("project rag-deep-dive: %w", err)
	}

	portfolioProj, err := q.UpsertProject(ctx, store.UpsertProjectParams{
		Slug:         "this-portfolio",
		Title:        "This portfolio",
		Tagline:      "Neo-brutalist portfolio built deliberately.",
		Summary:      "Tokens-first Next.js 16 + Go + Postgres portfolio. sqlc, pgx, golang-migrate on the backend; ISR + design-token-driven components on the frontend.",
		BodyOverview: portfolioOverview,
		BodyWhyBuilt: portfolioWhyBuilt,
		BodyLearning: portfolioLearning,
		RepoUrl:      text("https://github.com/anjanvikas/portfolio"),
		SortOrder:    2,
		Featured:     true,
		PublishedAt:  ts(2026, 5, 1),
	})
	if err != nil {
		return fmt.Errorf("project portfolio: %w", err)
	}

	for _, link := range []store.LinkProjectTagParams{
		{ProjectID: resumeRanker.ID, TagID: tagPython.ID},
		{ProjectID: resumeRanker.ID, TagID: tagLLM.ID},
		{ProjectID: ragDeepDive.ID, TagID: tagPython.ID},
		{ProjectID: ragDeepDive.ID, TagID: tagRag.ID},
		{ProjectID: ragDeepDive.ID, TagID: tagLLM.ID},
		{ProjectID: portfolioProj.ID, TagID: tagGo.ID},
		{ProjectID: portfolioProj.ID, TagID: tagNext.ID},
		{ProjectID: portfolioProj.ID, TagID: tagDesign.ID},
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

const resumeRankerOverview = "## Overview\n\n" +
	"AI Resume Ranker is a screening platform that turns a stack of PDFs\n" +
	"and a job description into a ranked shortlist with explainable scores.\n\n" +
	"- Parse resumes into structured candidate profiles.\n" +
	"- Shortlist with Sentence-Transformer embeddings for semantic similarity.\n" +
	"- Deep-evaluate the top candidates with an LLM rubric.\n" +
	"- Async background jobs with real-time progress polling for bulk uploads.\n"

const resumeRankerWhyBuilt = "## Why I built this\n\n" +
	"Keyword filters are too dumb; reading 500 resumes by hand is too slow.\n" +
	"I wanted a pipeline that combines cheap-and-fast semantic shortlisting\n" +
	"with expensive-and-careful LLM evaluation — and shows its work.\n"

const resumeRankerLearning = "## Learning journey\n\n" +
	"1. Embedding-only ranking is good enough for the top 20% — pay the LLM\n" +
	"   bill only for that bucket.\n" +
	"2. Async jobs + progress polling beats long-running HTTP every time.\n" +
	"3. Google OAuth2 was the right call over rolling auth myself.\n"

const ragDeepDiveOverview = "## Overview\n\n" +
	"A production-grade conversational AI platform built to *teach* RAG, not\n" +
	"just use it.\n\n" +
	"- Hybrid retrieval: dense embeddings + BM25 + cross-encoder reranking.\n" +
	"- GraphRAG and multi-hop retrieval across Qdrant and Neo4j.\n" +
	"- Pipeline explainer UI that exposes every retrieval stage live.\n" +
	"- Interactive Learning Hub with hands-on RAG component demos.\n"

const ragDeepDiveWhyBuilt = "## Why I built this\n\n" +
	"Most RAG tutorials stop at \"top-k from a vector DB.\" That's not how\n" +
	"production systems retrieve. I wanted a single place to see hybrid\n" +
	"search, reranking, and graph traversal side-by-side and understand\n" +
	"why each layer earns its complexity.\n"

const ragDeepDiveLearning = "## Learning journey\n\n" +
	"1. Cross-encoder reranking changes everything — and it's cheap if you\n" +
	"   only rerank the top 50.\n" +
	"2. Graph + vector beats either alone on multi-hop questions.\n" +
	"3. The explainer UI taught me more about my own pipeline than logs ever did.\n"

const portfolioOverview = "## Overview\n\n" +
	"The site you're reading. A tokens-first, neo-brutalist portfolio built\n" +
	"to read well on a recruiter scan *and* survive a 10-minute deep read.\n\n" +
	"- Frontend: Next.js 16 + TypeScript + Tailwind, ISR for project/blog pages.\n" +
	"- Backend: Go + chi + sqlc + pgx + golang-migrate on Postgres.\n" +
	"- Local dev: Docker (Postgres + Redis). Deploy cutover: Neon + Fly.io.\n"

const portfolioWhyBuilt = "## Why I built this\n\n" +
	"Most engineer portfolios are either templates or one-off React projects.\n" +
	"I wanted something that doubles as a working showcase of the stack I\n" +
	"actually ship in — Go backend, typed SQL, design tokens, admin CMS — and\n" +
	"forces me to live with my own decisions.\n"

const portfolioLearning = "## Learning journey\n\n" +
	"1. sqlc + pgx beats GORM for portfolio-shaped CRUD. Compile-time SQL safety\n" +
	"   over runtime convenience.\n" +
	"2. Design tokens locked in Figma *before* writing components saved a refactor.\n" +
	"3. Next.js `force-cache` is great for static prod but a footgun in dev —\n" +
	"   use `revalidate` for anything that comes from your own DB.\n"
