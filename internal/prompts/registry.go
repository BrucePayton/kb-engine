// Package prompts is the explicit inventory of static prompts managed through
// Langfuse. It references existing Go constants and YAML source files rather
// than replacing application call sites or altering third-party-owned code.
package prompts

import (
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/Tencent/WeKnora/internal/agent"
	"github.com/Tencent/WeKnora/internal/tracing/langfuse"
	"gopkg.in/yaml.v3"
)

type yamlFile struct {
	Templates []struct {
		ID      string `yaml:"id"`
		Content string `yaml:"content"`
		User    string `yaml:"user"`
	} `yaml:"templates"`
}

// RegisterAll lists prompts that WeKnora owns and intends to manage in
// Langfuse. Adding a prompt is an explicit, low-risk registry-only change.
func RegisterAll() []langfuse.PromptRecord {
	prompts := registerYAMLTemplates("config/prompt_templates")
	prompts = append(prompts, []langfuse.PromptRecord{
		{Name: "wiki_taxonomy_plan", Content: agent.WikiTaxonomyPlanPrompt, Category: "wiki_ingest", Tags: []string{"wiki", "ingest", "taxonomy"}},
		{Name: "wiki_summary", Content: agent.WikiSummaryPrompt, Category: "wiki_ingest", Tags: []string{"wiki", "ingest", "summary"}},
		{Name: "wiki_extraction", Content: agent.WikiKnowledgeExtractPrompt, Category: "wiki_ingest", Tags: []string{"wiki", "ingest", "extraction"}},
		{Name: "wiki_candidate_slug", Content: agent.WikiCandidateSlugPrompt, Category: "wiki_ingest", Tags: []string{"wiki", "ingest", "candidate"}},
		{Name: "wiki_citation", Content: agent.WikiChunkCitationPrompt, Category: "wiki_ingest", Tags: []string{"wiki", "ingest", "citation"}},
		{Name: "wiki_page_modify_system", Content: agent.WikiPageModifySystemPrompt, Category: "wiki_ingest", Tags: []string{"wiki", "ingest", "modify"}},
		{Name: "wiki_page_modify_user", Content: agent.WikiPageModifyUserPrompt, Category: "wiki_ingest", Tags: []string{"wiki", "ingest", "modify"}},
		{Name: "wiki_index_intro", Content: agent.WikiIndexIntroPrompt, Category: "wiki_ingest", Tags: []string{"wiki", "index", "intro"}},
		{Name: "wiki_index_intro_update", Content: agent.WikiIndexIntroUpdatePrompt, Category: "wiki_ingest", Tags: []string{"wiki", "index", "intro"}},
		{Name: "wiki_dedup", Content: agent.WikiDeduplicationPrompt, Category: "wiki_ingest", Tags: []string{"wiki", "ingest", "dedup"}},
	}...)
	for i := range prompts {
		prompts[i].Type = "text"
		prompts[i].Labels = []string{"production"}
		prompts[i].Version = 1
	}
	sort.Slice(prompts, func(i, j int) bool { return prompts[i].Name < prompts[j].Name })
	return prompts
}

func registerYAMLTemplates(dir string) []langfuse.PromptRecord {
	paths, _ := filepath.Glob(filepath.Join(dir, "*.yaml"))
	result := []langfuse.PromptRecord{}
	for _, path := range paths {
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		var file yamlFile
		if yaml.Unmarshal(data, &file) != nil {
			continue
		}
		category := "yaml_template:" + strings.TrimSuffix(filepath.Base(path), ".yaml")
		for _, template := range file.Templates {
			if template.ID == "" || template.Content == "" {
				continue
			}
			result = append(result, langfuse.PromptRecord{Name: template.ID, Content: template.Content, Category: category, Tags: []string{"yaml-template", filepath.Base(path)}})
			if template.User != "" {
				result = append(result, langfuse.PromptRecord{Name: template.ID + "_user", Content: template.User, Category: category, Tags: []string{"yaml-template", "user-message", filepath.Base(path)}})
			}
		}
	}
	return result
}
