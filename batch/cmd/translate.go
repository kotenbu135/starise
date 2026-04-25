package cmd

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/kotenbu135/starise/batch/internal/db"
	"github.com/kotenbu135/starise/batch/internal/translate"
	"github.com/spf13/cobra"
)

var (
	translateProvider    string
	translateCacheDir    string
	translateSourceDir   string
	translateLimit       int
	translateBatchSize   int
	translateConcurrency int
	translateHalve       bool
	translateModel       string
	translateAPIKey      string
)

var translateCmd = &cobra.Command{
	Use:   "translate",
	Short: "Translate English repository descriptions to Japanese",
	Long: `Translate descriptions to Japanese, populating the content-addressed
cache under data/translations/. Cache hits are reused forever; only new
or changed descriptions hit the API.

Sources:
  --source-dir DIR    read descriptions from data/repos/*.json (initial seed)
  (default)           read descriptions from the SQLite DB

Providers:
  --provider claude        Anthropic API (uses ANTHROPIC_API_KEY, requires credit)
  --provider claude-code   Claude CLI subprocess (uses your subscription, no API credit)
  --provider gemini        Gemini API (uses GEMINI_API_KEY, free tier)
`,
	RunE: func(cmd *cobra.Command, args []string) error {
		tr, err := buildTranslator()
		if err != nil {
			return err
		}

		var srcs []string
		if translateSourceDir != "" {
			srcs, err = translate.ReadDescriptionsFromRepoDir(translateSourceDir)
			if err != nil {
				return fmt.Errorf("read source dir: %w", err)
			}
		} else {
			d, err := openDB()
			if err != nil {
				return err
			}
			defer d.Close()
			repos, err := db.ListAllRepositories(d)
			if err != nil {
				return fmt.Errorf("list repos: %w", err)
			}
			srcs = make([]string, 0, len(repos))
			for _, r := range repos {
				if r.Description != "" {
					srcs = append(srcs, r.Description)
				}
			}
		}

		runner := &translate.Runner{
			Cache:           &translate.Cache{Dir: translateCacheDir},
			Translator:      tr,
			BatchSize:       translateBatchSize,
			Concurrency:     translateConcurrency,
			HalveOnMismatch: translateHalve,
			ErrorLog:        os.Stderr,
		}

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		start := time.Now()
		stats, err := runner.Run(ctx, srcs, translateLimit)
		fmt.Printf("translate: provider=%s sources=%d hits=%d translated=%d failed=%d skipped=%d elapsed=%s\n",
			tr.Name(), stats.Total, stats.CacheHits, stats.Translated, stats.Failed, stats.Skipped,
			time.Since(start).Round(time.Second))
		return err
	},
}

func buildTranslator() (translate.Translator, error) {
	switch strings.ToLower(translateProvider) {
	case "claude":
		key := translateAPIKey
		if key == "" {
			key = os.Getenv("ANTHROPIC_API_KEY")
		}
		if key == "" {
			return nil, fmt.Errorf("claude: ANTHROPIC_API_KEY not set (or pass --api-key)")
		}
		return &translate.ClaudeTranslator{APIKey: key, Model: translateModel}, nil
	case "claude-code", "claudecode", "cc":
		// Subscription-funded path: shells out to `claude -p`. No API key
		// or credit balance required.
		return &translate.ClaudeCodeTranslator{Model: translateModel}, nil
	case "gemini", "":
		key := translateAPIKey
		if key == "" {
			key = os.Getenv("GEMINI_API_KEY")
		}
		if key == "" {
			return nil, fmt.Errorf("gemini: GEMINI_API_KEY not set (or pass --api-key)")
		}
		return &translate.GeminiTranslator{APIKey: key, Model: translateModel}, nil
	default:
		return nil, fmt.Errorf("unknown provider %q (want claude, claude-code, or gemini)", translateProvider)
	}
}

func init() {
	translateCmd.Flags().StringVar(&translateProvider, "provider", "gemini", "translation provider: claude | gemini")
	translateCmd.Flags().StringVar(&translateCacheDir, "cache-dir", "../data/translations", "translation cache root")
	translateCmd.Flags().StringVar(&translateSourceDir, "source-dir", "", "read descriptions from JSON files here (default: read from DB)")
	translateCmd.Flags().IntVar(&translateLimit, "limit", 0, "cap new translations per run (0 = unlimited)")
	translateCmd.Flags().IntVar(&translateBatchSize, "batch-size", 50, "strings per provider call")
	translateCmd.Flags().IntVar(&translateConcurrency, "concurrency", 1, "parallel in-flight provider calls (try 5 for claude-code on Max plan)")
	translateCmd.Flags().BoolVar(&translateHalve, "halve-on-mismatch", true, "on length mismatch, halve the batch and retry recursively (recovers from LLM dedup/drop)")
	translateCmd.Flags().StringVar(&translateModel, "model", "", "override provider model name")
	translateCmd.Flags().StringVar(&translateAPIKey, "api-key", "", "API key (default: env var)")
}
