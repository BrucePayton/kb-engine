// Command sync_prompts publishes the explicit static-prompt registry or reads
// one Langfuse prompt for inspection. It never modifies local source files.
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/Tencent/WeKnora/internal/prompts"
	"github.com/Tencent/WeKnora/internal/tracing/langfuse"
)

func main() {
	all := flag.Bool("all", false, "sync all registered prompts")
	category := flag.String("category", "", "sync one registry category")
	dryRun := flag.Bool("dry-run", false, "show prompts without publishing")
	pull := flag.Bool("pull", false, "read one prompt from Langfuse")
	name := flag.String("name", "", "Langfuse prompt name (required with --pull)")
	label := flag.String("label", "production", "Langfuse label for --pull")
	flag.Parse()
	cfg := langfuse.LoadConfigFromEnv()
	if !cfg.HasCredentials() {
		fmt.Fprintln(os.Stderr, "Langfuse is not configured")
		os.Exit(2)
	}
	client := langfuse.NewPromptSyncClient(cfg)
	ctx := context.Background()
	if *pull {
		if *name == "" {
			fmt.Fprintln(os.Stderr, "--name is required with --pull")
			os.Exit(2)
		}
		p, err := client.FetchPrompt(ctx, *name, *label)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		fmt.Printf("Fetched %s (v%d, %s)\nTags: %s\n\n%s\n", p.Name, p.Version, p.Type, strings.Join(p.Tags, ", "), p.Content)
		return
	}
	if !*all && *category == "" {
		fmt.Fprintln(os.Stderr, "specify --all or --category")
		os.Exit(2)
	}
	entries := prompts.RegisterAll()
	filtered := entries[:0]
	for _, p := range entries {
		if *category == "" || p.Category == *category {
			filtered = append(filtered, p)
		}
	}
	if len(filtered) == 0 {
		fmt.Println("No registered prompts found.")
		return
	}
	for _, p := range filtered {
		fmt.Printf("[%s] %s\n", p.Category, p.Name)
		if !*dryRun {
			if err := client.SyncPrompt(ctx, p); err != nil {
				fmt.Fprintf(os.Stderr, "  failed: %v\n", err)
			}
		}
	}
}
