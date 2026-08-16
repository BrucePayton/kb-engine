package service

import (
	"context"
	"strings"
	"testing"

	"github.com/Tencent/WeKnora/internal/agent"
	"github.com/Tencent/WeKnora/internal/types"
)

func TestFormatExistingTaxonomyForPrompt(t *testing.T) {
	got := formatExistingTaxonomyForPrompt([][]string{
		{"春节", "传统习俗"},
		{"春节", "文化习俗", "节日习俗"},
		{"春节习俗"},
		{"产品定位"},
	})
	want := "产品定位\n" +
		"春节\n" +
		"  传统习俗\n" +
		"  文化习俗\n" +
		"    节日习俗\n" +
		"春节习俗"
	if got != want {
		t.Fatalf("formatExistingTaxonomyForPrompt():\n%q\nwant:\n%q", got, want)
	}
}

func TestFormatExistingTaxonomyForPromptEmpty(t *testing.T) {
	if got := formatExistingTaxonomyForPrompt(nil); got != "" {
		t.Fatalf("formatExistingTaxonomyForPrompt(nil) = %q, want empty", got)
	}
}

func TestParseTaxonomyAssignments(t *testing.T) {
	raw := "```json\n{\"assignments\":[" +
		"{\"slug\":\"entity/zhang-san\",\"path\":[\"人物\"]}," +
		"{\"slug\":\"concept/spring\",\"path\":[\"节日\",\"传统节日\"]}," +
		"{\"slug\":\"  \",\"path\":[\"X\"]}," +
		"{\"slug\":\"entity/unclassified\",\"path\":[]}" +
		"]}\n```"

	got := parseTaxonomyAssignments(raw)
	if len(got) != 3 {
		t.Fatalf("parseTaxonomyAssignments() returned %d entries, want 3 (blank slug dropped): %v", len(got), got)
	}
	if strings.Join(got["entity/zhang-san"], "/") != "人物" {
		t.Fatalf("zhang-san path = %v, want [人物]", got["entity/zhang-san"])
	}
	if strings.Join(got["concept/spring"], "/") != "节日/传统节日" {
		t.Fatalf("spring path = %v, want [节日 传统节日]", got["concept/spring"])
	}
	if p, ok := got["entity/unclassified"]; !ok || len(p) != 0 {
		t.Fatalf("unclassified path = %v (ok=%v), want empty slice present", p, ok)
	}
}

func TestParseTaxonomyAssignmentsMalformed(t *testing.T) {
	if got := parseTaxonomyAssignments("not json at all"); got != nil {
		t.Fatalf("parseTaxonomyAssignments(garbage) = %v, want nil", got)
	}
}

func TestCosineSimilarity(t *testing.T) {
	if got := cosineSimilarity([]float32{1, 0}, []float32{1, 0}); got < 0.999 {
		t.Fatalf("identical vectors sim = %v, want ~1", got)
	}
	if got := cosineSimilarity([]float32{1, 0}, []float32{0, 1}); got != 0 {
		t.Fatalf("orthogonal vectors sim = %v, want 0", got)
	}
	if got := cosineSimilarity([]float32{1, 0}, nil); got != 0 {
		t.Fatalf("mismatched length sim = %v, want 0", got)
	}
	if got := cosineSimilarity([]float32{0, 0}, []float32{1, 1}); got != 0 {
		t.Fatalf("zero-norm sim = %v, want 0", got)
	}
}

func TestSelectFoldersByVectors(t *testing.T) {
	deeper := [][]string{
		{"AI", "厂商"}, // 0
		{"AI", "模型"}, // 1
		{"地理", "城市"}, // 2
	}
	folderVecs := [][]float32{
		{1, 0, 0},
		{0.9, 0.1, 0},
		{0, 0, 1},
	}
	// Item closest to folder 0 (and 1 as runner-up); far from 2.
	itemVecs := [][]float32{{1, 0, 0}}

	got := selectFoldersByVectors(deeper, folderVecs, itemVecs, 2)
	if len(got) != 2 {
		t.Fatalf("selectFoldersByVectors() returned %d folders, want 2: %v", len(got), got)
	}
	// Input order preserved: folders 0 and 1, not the orthogonal 2.
	if strings.Join(got[0], "/") != "AI/厂商" || strings.Join(got[1], "/") != "AI/模型" {
		t.Fatalf("selectFoldersByVectors() = %v, want [[AI 厂商] [AI 模型]]", got)
	}
}

func TestSelectFoldersByVectorsGuards(t *testing.T) {
	if got := selectFoldersByVectors([][]string{{"a"}}, [][]float32{{1}, {2}}, [][]float32{{1}}, 1); got != nil {
		t.Fatalf("mismatched lengths should return nil, got %v", got)
	}
	if got := selectFoldersByVectors([][]string{{"a"}}, [][]float32{{1}}, nil, 1); got != nil {
		t.Fatalf("no items should return nil, got %v", got)
	}
}

func TestCollectTaxonomyItems(t *testing.T) {
	slugUpdates := map[string][]SlugUpdate{
		"entity/b": {{Type: types.WikiPageTypeEntity, Item: extractedItem{Name: "B", Description: "about B"}}},
		"entity/a": {{Type: types.WikiPageTypeConcept, Item: extractedItem{Name: "A"}}},
		"sum/x":    {{Type: "summary"}},
		"ret/y":    {{Type: "retract"}},
	}
	items := collectTaxonomyItems(slugUpdates)
	if len(items) != 2 {
		t.Fatalf("collectTaxonomyItems() = %d items, want 2 (summary/retract skipped): %+v", len(items), items)
	}
	// Deterministic slug order so chunk boundaries are stable.
	if items[0].slug != "entity/a" || items[1].slug != "entity/b" {
		t.Fatalf("collectTaxonomyItems() order = [%s, %s], want [entity/a, entity/b]", items[0].slug, items[1].slug)
	}
	if items[1].about != "about B" {
		t.Fatalf("items[1].about = %q, want %q", items[1].about, "about B")
	}
}

func TestPlanBatchTaxonomyIncludesKBExtractionInstructions(t *testing.T) {
	const taxonomyInstructions = "目录固定为：领域 / 产品线 / 主题"
	model := &templateCaptureChatModel{
		response: `{"assignments":[{"slug":"concept/cache-invalidation","path":["工程","后端","缓存"]}]}`,
	}
	service := &wikiIngestService{}
	kb := &types.KnowledgeBase{
		ID: "kb-quality-inspection",
		WikiConfig: &types.WikiConfig{
			ExtractionInstructions: taxonomyInstructions,
		},
	}

	got := service.planBatchTaxonomy(
		context.Background(),
		model,
		kb,
		map[string][]SlugUpdate{
			"concept/cache-invalidation": {{
				Type: types.WikiPageTypeConcept,
				Item: extractedItem{
					Name:        "缓存失效策略",
					Description: "后端缓存的更新与失效规则",
				},
			}},
		},
		"简体中文",
	)

	if !strings.Contains(model.prompt, taxonomyInstructions) {
		t.Fatalf("taxonomy prompt does not include KB extraction instructions: %q", model.prompt)
	}
	if !strings.Contains(model.prompt, "<wiki_taxonomy_business_instructions>") {
		t.Fatalf("taxonomy instructions must use the scoped business-instruction wrapper: %q", model.prompt)
	}
	if path := strings.Join(got["concept/cache-invalidation"], "/"); path != "工程/后端/缓存" {
		t.Fatalf("planned path = %q, want %q", path, "工程/后端/缓存")
	}
}

func TestPlanBatchTaxonomyRendersMixedGovernanceContract(t *testing.T) {
	const businessInstructions = "一级固定，二级受控，第三级动态；种子是优先词汇，不是硬白名单"
	model := &templateCaptureChatModel{
		response: `{"assignments":[{"slug":"concept/distributed-consistency","path":["软件工程","分布式系统","一致性"]}]}`,
	}
	service := &wikiIngestService{}
	kb := &types.KnowledgeBase{
		ID:         "kb-mixed-taxonomy",
		WikiConfig: &types.WikiConfig{ExtractionInstructions: businessInstructions},
	}

	got := service.planBatchTaxonomy(
		context.Background(),
		model,
		kb,
		map[string][]SlugUpdate{
			"concept/distributed-consistency": {{
				Type: types.WikiPageTypeConcept,
				Item: extractedItem{
					Name:        "分布式一致性",
					Description: "分布式系统中的一致性模型与实践",
				},
			}},
		},
		"简体中文",
	)

	prompt := model.prompt
	for _, required := range []string{
		"fixed levels",
		"controlled levels",
		"dynamic levels",
		"dynamic final level",
		"preferred vocabulary",
		"not a hard allowlist",
		"no existing or seed label fits",
		businessInstructions,
		"<wiki_taxonomy_business_instructions>",
	} {
		if !strings.Contains(prompt, required) {
			t.Fatalf("rendered taxonomy prompt is missing %q: %s", required, prompt)
		}
	}
	if path := strings.Join(got["concept/distributed-consistency"], "/"); path != "软件工程/分布式系统/一致性" {
		t.Fatalf("planned path = %q, want %q", path, "软件工程/分布式系统/一致性")
	}
}

func TestPlanBatchTaxonomyWithoutWikiConfigKeepsDefaultPrompt(t *testing.T) {
	model := &templateCaptureChatModel{
		response: `{"assignments":[{"slug":"concept/system-overview","path":["基础概念"]}]}`,
	}
	service := &wikiIngestService{}
	kb := &types.KnowledgeBase{ID: "kb-default"}

	service.planBatchTaxonomy(
		context.Background(),
		model,
		kb,
		map[string][]SlugUpdate{
			"concept/system-overview": {{
				Type: types.WikiPageTypeConcept,
				Item: extractedItem{Name: "系统概览"},
			}},
		},
		"简体中文",
	)

	if strings.Contains(model.prompt, "<wiki_taxonomy_business_instructions>") {
		t.Fatalf("empty KB instructions must keep the default taxonomy prompt: %q", model.prompt)
	}
}

func TestWikiTaxonomyPromptAllowsStorageSupportedThreeLevelPaths(t *testing.T) {
	if !strings.Contains(agent.WikiTaxonomyPlanPrompt, "at most 3 levels") {
		t.Fatalf("taxonomy prompt must allow the storage-supported three-level path")
	}
}

func TestWikiTaxonomyPromptAllowsKBSpecificStableHierarchy(t *testing.T) {
	if !strings.Contains(agent.WikiTaxonomyPlanPrompt, "domain-specific stable hierarchy") {
		t.Fatalf("taxonomy prompt must allow KB business instructions to define a domain hierarchy")
	}
}
