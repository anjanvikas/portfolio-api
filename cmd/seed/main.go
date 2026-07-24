// Seed program for local development.
//
// Mirrors the actual content currently in the local Docker Postgres instance
// (profile, experience, tags, the published blog post, projects, testimonials,
// and the media-library asset rows) so a fresh `make seed` reproduces the real
// site instead of placeholder copy.
//
// Idempotent: every insert is an ON CONFLICT DO UPDATE, so re-running this
// program never creates duplicates. Run with `make seed` (which loads .env)
// or `DATABASE_URL=... go run ./cmd/seed`.
package main

import (
	"context"
	_ "embed"
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
		Headline:  "Senior Software Engineer",
		Bio:       "I build production-scale LLM systems end to end — distributed backend pipelines, agent orchestration, the full stack. Go, Python, JavaScript, Kafka, Redis, PostgreSQL/MongoDB, vector databases. I care about CI/CD and systems that actually hold up at scale.",
		Location:  "Bengaluru, KA, India",
		Email:     "anjanvikas2001@gmail.com",
		ResumeUrl: text("https://pub-8309b9ffc8124004831c19e903d8bc4a.r2.dev/resume.pdf"),
		AvatarUrl: text("https://pub-8309b9ffc8124004831c19e903d8bc4a.r2.dev/assets/c6744887-Professional_headshot_of_the_young_202605241149.jpeg"),
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

	// ---- Tags ---------------------------------------------------------
	// Mirrors every tag slug actually created via the admin editor — including
	// the accidental near-duplicates ("llm"/"llms", "next-js"/"nextjs") that
	// exist in the real data because they were typed in separately over time.
	tagSeeds := []store.UpsertTagParams{
		{Slug: "go", Name: "Go"},
		{Slug: "design-systems", Name: "Design systems"},
		{Slug: "rag", Name: "RAG"},
		{Slug: "nextjs", Name: "Next.js"},
		{Slug: "next-js", Name: "Next.js"},
		{Slug: "python", Name: "Python"},
		{Slug: "llm", Name: "LLMs"},
		{Slug: "llms", Name: "LLMs"},
		{Slug: "transistor", Name: "Transistor"},
		{Slug: "smoke", Name: "Smoke"},
		{Slug: "computer-architecture", Name: "Computer Architecture"},
		{Slug: "alu", Name: "ALU"},
		{Slug: "mosfet", Name: "MosFet"},
		{Slug: "pgvector", Name: "pgvector"},
		{Slug: "testing", Name: "Testing"},
	}
	tags := make(map[string]store.Tag, len(tagSeeds))
	for _, t := range tagSeeds {
		row, err := q.UpsertTag(ctx, t)
		if err != nil {
			return fmt.Errorf("tag %s: %w", t.Slug, err)
		}
		tags[t.Slug] = row
	}

	// ---- Blog series (currently unused by any published post, kept as-is) --
	if _, err := q.UpsertBlogSeries(ctx, store.UpsertBlogSeriesParams{
		Slug:        "building-a-portfolio",
		Name:        "Building this portfolio",
		Description: "A live diary of designing and shipping this site.",
	}); err != nil {
		return fmt.Errorf("blog series: %w", err)
	}

	// ---- Blog post ----------------------------------------------------
	sandPost, err := q.UpsertBlogPost(ctx, store.UpsertBlogPostParams{
		Slug:            "from-a-grain-of-sand-to-a-computer-brain",
		Title:           "From a Grain of Sand to a Computer Brain",
		Excerpt:         "",
		Body:            blogGrainOfSandBody,
		CoverUrl:        "https://pub-8309b9ffc8124004831c19e903d8bc4a.r2.dev/assets/e145823c-Gemini_Generated_Image_c5zw90c5zw90c5zw.png",
		PublishedAt:     tsAt(2026, 6, 4, 14, 17, 50),
		ReadingTimeMins: content.ReadingTimeMins(blogGrainOfSandBody),
	})
	if err != nil {
		return fmt.Errorf("blog post sand-to-cpu: %w", err)
	}

	for _, link := range []store.LinkBlogPostTagParams{
		{BlogPostID: sandPost.ID, TagID: tags["alu"].ID},
		{BlogPostID: sandPost.ID, TagID: tags["computer-architecture"].ID},
		{BlogPostID: sandPost.ID, TagID: tags["mosfet"].ID},
		{BlogPostID: sandPost.ID, TagID: tags["transistor"].ID},
	} {
		if err := q.LinkBlogPostTag(ctx, link); err != nil {
			return fmt.Errorf("link blog tag: %w", err)
		}
	}

	// ---- Projects (2 featured) ------------------------------------------
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
	if _, err := q.UpdateProject(ctx, store.UpdateProjectParams{
		ID:           resumeRanker.ID,
		Slug:         resumeRanker.Slug,
		Title:        resumeRanker.Title,
		Tagline:      resumeRanker.Tagline,
		Summary:      resumeRanker.Summary,
		BodyOverview: resumeRanker.BodyOverview,
		BodyWhyBuilt: resumeRanker.BodyWhyBuilt,
		BodyLearning: resumeRanker.BodyLearning,
		CoverUrl:     "https://pub-8309b9ffc8124004831c19e903d8bc4a.r2.dev/assets/d8d0fe63-Gemini_Generated_Image_520inl520inl520i.png",
		RepoUrl:      resumeRanker.RepoUrl,
		LiveUrl:      resumeRanker.LiveUrl,
		Featured:     resumeRanker.Featured,
	}); err != nil {
		return fmt.Errorf("project ai-resume-ranker cover: %w", err)
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
	if _, err := q.UpdateProject(ctx, store.UpdateProjectParams{
		ID:           ragDeepDive.ID,
		Slug:         ragDeepDive.Slug,
		Title:        ragDeepDive.Title,
		Tagline:      ragDeepDive.Tagline,
		Summary:      ragDeepDive.Summary,
		BodyOverview: ragDeepDive.BodyOverview,
		BodyWhyBuilt: ragDeepDive.BodyWhyBuilt,
		BodyLearning: ragDeepDive.BodyLearning,
		CoverUrl:     "https://pub-8309b9ffc8124004831c19e903d8bc4a.r2.dev/assets/f8332b50-Gemini_Generated_Image_fyszq4fyszq4fysz.png",
		RepoUrl:      ragDeepDive.RepoUrl,
		LiveUrl:      ragDeepDive.LiveUrl,
		Featured:     ragDeepDive.Featured,
	}); err != nil {
		return fmt.Errorf("project rag-deep-dive cover: %w", err)
	}

	for _, link := range []store.LinkProjectTagParams{
		{ProjectID: resumeRanker.ID, TagID: tags["llms"].ID},
		{ProjectID: resumeRanker.ID, TagID: tags["python"].ID},
		{ProjectID: ragDeepDive.ID, TagID: tags["llms"].ID},
		{ProjectID: ragDeepDive.ID, TagID: tags["python"].ID},
		{ProjectID: ragDeepDive.ID, TagID: tags["rag"].ID},
	} {
		if err := q.LinkProjectTag(ctx, link); err != nil {
			return fmt.Errorf("link project tag: %w", err)
		}
	}

	// ---- Testimonials (2, both currently hidden via the admin toggle) -----
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
		row, err := q.UpsertTestimonial(ctx, t)
		if err != nil {
			return fmt.Errorf("testimonial %s: %w", t.AuthorName, err)
		}
		if _, err := q.SetTestimonialVisibility(ctx, store.SetTestimonialVisibilityParams{
			ID:      row.ID,
			Visible: false,
		}); err != nil {
			return fmt.Errorf("testimonial %s visibility: %w", t.AuthorName, err)
		}
	}

	// ---- Asset library (media uploaded via the admin editor) -------------
	assets := []store.UpsertAssetParams{
		{Filename: "Professional_headshot_of_the_young_202605241149.jpeg", MimeType: "image/jpeg", SizeBytes: 2385600, R2Key: "assets/c6744887-Professional_headshot_of_the_young_202605241149.jpeg", Width: int4(2752), Height: int4(1536)},
		{Filename: "svgviewer-png-output.png", MimeType: "image/png", SizeBytes: 308479, R2Key: "assets/bb95680a-svgviewer-png-output.png", Width: int4(2000), Height: int4(1440)},
		{Filename: "svgviewer-png-output (1).png", MimeType: "image/png", SizeBytes: 200646, R2Key: "assets/f160db88-svgviewer-png-output-1-.png", Width: int4(2000), Height: int4(960)},
		{Filename: "svgviewer-png-output (2).png", MimeType: "image/png", SizeBytes: 229272, R2Key: "assets/480c902d-svgviewer-png-output-2-.png", Width: int4(2000), Height: int4(1120)},
		{Filename: "svgviewer-png-output (3).png", MimeType: "image/png", SizeBytes: 152222, R2Key: "assets/9ac52125-svgviewer-png-output-3-.png", Width: int4(2000), Height: int4(1040)},
		{Filename: "svgviewer-png-output (4).png", MimeType: "image/png", SizeBytes: 232989, R2Key: "assets/5ec589cb-svgviewer-png-output-4-.png", Width: int4(2000), Height: int4(1080)},
		{Filename: "svgviewer-png-output (5).png", MimeType: "image/png", SizeBytes: 193638, R2Key: "assets/0117c3f2-svgviewer-png-output-5-.png", Width: int4(2000), Height: int4(1080)},
		{Filename: "svgviewer-png-output (6).png", MimeType: "image/png", SizeBytes: 167896, R2Key: "assets/2b4b1931-svgviewer-png-output-6-.png", Width: int4(2000), Height: int4(1240)},
		{Filename: "svgviewer-png-output (7).png", MimeType: "image/png", SizeBytes: 159693, R2Key: "assets/e5d79c6c-svgviewer-png-output-7-.png", Width: int4(2000), Height: int4(1000)},
		{Filename: "sand_to_cpu_cover.png", MimeType: "image/png", SizeBytes: 47216, R2Key: "assets/dc767fd9-sand_to_cpu_cover.png", Width: int4(670), Height: int4(385)},
		{Filename: "Gemini_Generated_Image_wttfb8wttfb8wttf.png", MimeType: "image/png", SizeBytes: 5713015, R2Key: "assets/562661ab-Gemini_Generated_Image_wttfb8wttfb8wttf.png", Width: int4(3200), Height: int4(1312)},
		{Filename: "Gemini_Generated_Image_520inl520inl520i.png", MimeType: "image/png", SizeBytes: 4861000, R2Key: "assets/d8d0fe63-Gemini_Generated_Image_520inl520inl520i.png", Width: int4(3200), Height: int4(1312)},
		{Filename: "Gemini_Generated_Image_fyszq4fyszq4fysz.png", MimeType: "image/png", SizeBytes: 4958876, R2Key: "assets/f8332b50-Gemini_Generated_Image_fyszq4fyszq4fysz.png", Width: int4(3200), Height: int4(1312)},
		{Filename: "Gemini_Generated_Image_c5zw90c5zw90c5zw.png", MimeType: "image/png", SizeBytes: 327173, R2Key: "assets/e145823c-Gemini_Generated_Image_c5zw90c5zw90c5zw.png", Width: int4(1600), Height: int4(656)},
	}
	for _, a := range assets {
		if _, err := q.UpsertAsset(ctx, a); err != nil {
			return fmt.Errorf("asset %s: %w", a.Filename, err)
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

func tsAt(y int, m time.Month, d, h, mi, s int) pgtype.Timestamptz {
	return pgtype.Timestamptz{Time: time.Date(y, m, d, h, mi, s, 0, time.UTC), Valid: true}
}

// ---- real content, embedded from cmd/seed/content/*.md --------------------

//go:embed content/blog-grain-of-sand.md
var blogGrainOfSandBody string

//go:embed content/project-ai-resume-ranker-overview.md
var resumeRankerOverview string

//go:embed content/project-ai-resume-ranker-why-built.md
var resumeRankerWhyBuilt string

//go:embed content/project-ai-resume-ranker-learning.md
var resumeRankerLearning string

//go:embed content/project-rag-deep-dive-overview.md
var ragDeepDiveOverview string

//go:embed content/project-rag-deep-dive-why-built.md
var ragDeepDiveWhyBuilt string

//go:embed content/project-rag-deep-dive-learning.md
var ragDeepDiveLearning string
