package discover

import (
	"fmt"
	"time"
)

// QuerySet holds an ordered list of GitHub Search API queries that, together,
// explore the OSS universe without hitting the Search API's 1000-result cap
// per query. Each query is independent — the caller fires them in parallel.
type QuerySet []string

// BuildQuerySet assembles the production discovery query list. Split across
// star-count bands, language × star bands, recent-activity filters for lower
// bands, new-repo discovery, topic-based exploration, framework-specific
// queries, and recent-activity splits for trending capture.
//
// The 2026-04-21 telemetry showed each query costs ~1 pt against the 5000
// pts/hour PAT budget, so the 131-query preset consumes <3% of budget —
// plenty of headroom if further expansion becomes useful.
//
// Dates are rendered relative to `now` so tests are deterministic.
func BuildQuerySet(now time.Time) QuerySet {
	recent := now.AddDate(0, -3, 0).Format("2006-01-02")   // 3 months ago
	active := now.AddDate(0, -6, 0).Format("2006-01-02")   // 6 months ago
	newRepo := now.AddDate(0, 0, -90).Format("2006-01-02") // 90 days ago
	lastWeek := now.AddDate(0, 0, -7).Format("2006-01-02")
	lastMonth := now.AddDate(0, 0, -30).Format("2006-01-02")

	base := "fork:false archived:false"

	starRanges := []string{
		"stars:>=50000",
		"stars:20000..49999",
		"stars:10000..19999",
		"stars:7000..9999",
		"stars:5000..6999",
		"stars:2000..4999",
		"stars:1000..1999",
		fmt.Sprintf("stars:600..999 pushed:>%s", active),
		fmt.Sprintf("stars:300..599 pushed:>%s", active),
		fmt.Sprintf("stars:100..299 pushed:>%s", active),
	}

	// Low-star tiers for breakout-axis candidates (1 <= start < 100).
	// Without these, the breakout ranking slots stayed empty because the
	// core preset only went down to stars:100. pushed:>active filters out
	// abandoned micro-repos that would just bloat the DB.
	lowStarRanges := []string{
		fmt.Sprintf("stars:50..99 pushed:>%s", active),
		fmt.Sprintf("stars:30..49 pushed:>%s", active),
		fmt.Sprintf("stars:10..29 pushed:>%s", recent),
		fmt.Sprintf("stars:5..9 pushed:>%s", recent),
	}

	languages := []string{
		"Python", "TypeScript", "JavaScript", "Go", "Rust",
		"Java", "C++", "C#", "Swift", "Kotlin",
		"Dart", "Ruby", "PHP", "Scala", "Elixir",
	}
	// Extended languages: niche / declining-but-active ecosystems that
	// produce breakout OSS but were absent from the v1 preset.
	extendedLanguages := []string{
		"Lua", "Haskell", "Zig", "OCaml", "Elm",
		"Nim", "Crystal", "Clojure", "F#", "R", "Shell",
	}
	langStarRanges := []string{
		"stars:>=10000",
		"stars:1000..9999",
		fmt.Sprintf("stars:100..999 pushed:>%s", active),
	}

	// AI/ML topics — original preset.
	topics := []string{
		"llm", "ai-agent", "generative-ai", "machine-learning",
		"large-language-model", "rag", "vector-database",
	}
	// Extended domain topics beyond AI/ML.
	extendedTopics := []string{
		"blockchain", "web3", "gamedev", "devops", "security",
		"nlp", "computer-vision", "robotics", "iot", "database",
		"deep-learning", "data-science", "backend", "frontend", "cli",
	}
	// Framework-specific topics — surface popular framework ecosystems
	// whose repos often carry just the framework's topic.
	frameworkTopics := []string{
		"react", "vue", "svelte", "nextjs", "fastapi",
		"django", "rails", "laravel", "flutter", "tensorflow",
	}

	var qs QuerySet
	for _, sr := range starRanges {
		qs = append(qs, fmt.Sprintf("%s %s sort:stars-desc", sr, base))
	}
	for _, lang := range languages {
		for _, sr := range langStarRanges {
			qs = append(qs, fmt.Sprintf("language:%s %s %s sort:stars-desc", lang, sr, base))
		}
	}
	qs = append(qs,
		fmt.Sprintf("stars:>50 %s created:>%s sort:stars-desc", base, newRepo),
		fmt.Sprintf("stars:10..50 %s created:>%s pushed:>%s sort:stars-desc", base, newRepo, recent),
	)
	for _, t := range topics {
		qs = append(qs, fmt.Sprintf("topic:%s stars:>30 %s pushed:>%s sort:stars-desc", t, base, recent))
	}

	// --- Expansion (2026-04-22) ------------------------------------------
	for _, sr := range lowStarRanges {
		qs = append(qs, fmt.Sprintf("%s %s sort:stars-desc", sr, base))
	}
	for _, lang := range extendedLanguages {
		for _, sr := range langStarRanges {
			qs = append(qs, fmt.Sprintf("language:%s %s %s sort:stars-desc", lang, sr, base))
		}
	}
	for _, t := range extendedTopics {
		qs = append(qs, fmt.Sprintf("topic:%s stars:>30 %s pushed:>%s sort:stars-desc", t, base, recent))
	}
	for _, t := range frameworkTopics {
		qs = append(qs, fmt.Sprintf("topic:%s stars:>30 %s pushed:>%s sort:stars-desc", t, base, recent))
	}
	// Recent-activity splits catch fresh trending repos that get buried
	// under older heavyweights in the broad star-band queries.
	qs = append(qs,
		fmt.Sprintf("stars:>100 %s pushed:>%s sort:stars-desc", base, lastWeek),
		fmt.Sprintf("stars:>50 %s pushed:>%s sort:stars-desc", base, lastWeek),
		fmt.Sprintf("stars:>=1000 %s pushed:>%s sort:stars-desc", base, lastMonth),
		fmt.Sprintf("stars:>=500 %s pushed:>%s sort:stars-desc", base, lastMonth),
		fmt.Sprintf("stars:>=100 %s pushed:>%s sort:stars-desc", base, lastMonth),
	)

	return qs
}
