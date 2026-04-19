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
// bands, new-repo discovery, and topic-based exploration for AI/ML areas.
// Matches the v1 strategy that yielded ~30k repos before the v3 rewrite
// collapsed discovery to a single query.
//
// Dates are rendered relative to `now` so tests are deterministic.
func BuildQuerySet(now time.Time) QuerySet {
	recent := now.AddDate(0, -3, 0).Format("2006-01-02")  // 3 months ago
	active := now.AddDate(0, -6, 0).Format("2006-01-02")  // 6 months ago
	newRepo := now.AddDate(0, 0, -90).Format("2006-01-02") // 90 days ago

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

	languages := []string{
		"Python", "TypeScript", "JavaScript", "Go", "Rust",
		"Java", "C++", "C#", "Swift", "Kotlin",
		"Dart", "Ruby", "PHP", "Scala", "Elixir",
	}
	langStarRanges := []string{
		"stars:>=10000",
		"stars:1000..9999",
		fmt.Sprintf("stars:100..999 pushed:>%s", active),
	}

	topics := []string{
		"llm", "ai-agent", "generative-ai", "machine-learning",
		"large-language-model", "rag", "vector-database",
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
	return qs
}
