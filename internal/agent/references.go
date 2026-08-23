package agent

import (
	"regexp"
	"strings"

	agenttools "github.com/Tencent/WeKnora/internal/agent/tools"
	"github.com/Tencent/WeKnora/internal/types"
)

const (
	maxFallbackKnowledgeReferences  = 8
	attributionKindMetadataKey      = "attribution_kind"
	attributionKindInlineCitation   = "inline_citation"
	attributionKindSupportingSearch = "supporting_retrieval"
)

var publicChunkCitationRE = regexp.MustCompile(`(?is)<kb\b[^>]*\bchunk_id\s*=\s*"([^"]+)"[^>]*>`)

// collectStepKnowledgeReferences preserves trusted search results independently
// from whether the model later follows the inline citation protocol.
func collectStepKnowledgeReferences(state *types.AgentState, step types.AgentStep) {
	if state == nil {
		return
	}
	seen := make(map[string]struct{}, len(state.KnowledgeRefs))
	for _, ref := range state.KnowledgeRefs {
		if ref != nil {
			seen[knowledgeReferenceKey(ref)] = struct{}{}
		}
	}
	for _, toolCall := range step.ToolCalls {
		for _, ref := range agenttools.KnowledgeReferencesFromResult(toolCall.Result) {
			if !validKnowledgeReference(ref) {
				continue
			}
			key := knowledgeReferenceKey(ref)
			if _, exists := seen[key]; exists {
				continue
			}
			copyRef := *ref
			if ref.Metadata != nil {
				copyRef.Metadata = make(map[string]string, len(ref.Metadata))
				for metadataKey, value := range ref.Metadata {
					copyRef.Metadata[metadataKey] = value
				}
			}
			state.KnowledgeRefs = append(state.KnowledgeRefs, &copyRef)
			seen[key] = struct{}{}
		}
	}
}

// finalKnowledgeReferences returns only chunks explicitly cited by the final
// answer. Retrieved-but-unused chunks must never be presented as citations.
func finalKnowledgeReferences(
	answer string,
	citedChunkIDs []string,
	candidates []*types.SearchResult,
	citationsEnabled bool,
) []*types.SearchResult {
	if !citationsEnabled || len(candidates) == 0 {
		return nil
	}
	unique := deduplicateKnowledgeReferences(candidates)
	if len(unique) == 0 {
		return nil
	}

	orderedIDs := make([]string, 0, len(citedChunkIDs))
	seenIDs := make(map[string]struct{}, len(citedChunkIDs))
	for _, id := range citedChunkIDs {
		if _, exists := seenIDs[id]; !exists && strings.TrimSpace(id) != "" {
			seenIDs[id] = struct{}{}
			orderedIDs = append(orderedIDs, id)
		}
	}
	// Compatibility path for public <kb/> answers produced outside the private
	// handle codec.
	if len(orderedIDs) == 0 {
		for _, match := range publicChunkCitationRE.FindAllStringSubmatch(answer, -1) {
			if len(match) != 2 {
				continue
			}
			if _, exists := seenIDs[match[1]]; exists {
				continue
			}
			seenIDs[match[1]] = struct{}{}
			orderedIDs = append(orderedIDs, match[1])
		}
	}
	if len(orderedIDs) == 0 {
		limit := len(unique)
		if limit > maxFallbackKnowledgeReferences {
			limit = maxFallbackKnowledgeReferences
		}
		return knowledgeReferencesWithAttribution(
			unique[:limit],
			attributionKindSupportingSearch,
		)
	}
	byID := make(map[string]*types.SearchResult, len(unique))
	for _, ref := range unique {
		byID[ref.ID] = ref
	}
	selected := make([]*types.SearchResult, 0, len(orderedIDs))
	for _, id := range orderedIDs {
		if ref := byID[id]; ref != nil {
			selected = append(selected, ref)
		}
	}
	return knowledgeReferencesWithAttribution(selected, attributionKindInlineCitation)
}

func knowledgeReferencesWithAttribution(
	refs []*types.SearchResult,
	kind string,
) []*types.SearchResult {
	result := make([]*types.SearchResult, 0, len(refs))
	for _, ref := range refs {
		if ref == nil {
			continue
		}
		copyRef := *ref
		copyRef.Metadata = make(map[string]string, len(ref.Metadata)+1)
		for key, value := range ref.Metadata {
			copyRef.Metadata[key] = value
		}
		copyRef.Metadata[attributionKindMetadataKey] = kind
		result = append(result, &copyRef)
	}
	return result
}

func knowledgeReferenceAttribution(refs []*types.SearchResult) string {
	if len(refs) == 0 || refs[0] == nil {
		return ""
	}
	return refs[0].Metadata[attributionKindMetadataKey]
}

func deduplicateKnowledgeReferences(refs []*types.SearchResult) []*types.SearchResult {
	result := make([]*types.SearchResult, 0, len(refs))
	seen := make(map[string]struct{}, len(refs))
	for _, ref := range refs {
		if !validKnowledgeReference(ref) {
			continue
		}
		key := knowledgeReferenceKey(ref)
		if _, exists := seen[key]; exists {
			continue
		}
		result = append(result, ref)
		seen[key] = struct{}{}
	}
	return result
}

func validKnowledgeReference(ref *types.SearchResult) bool {
	return ref != nil && strings.TrimSpace(ref.ID) != "" &&
		strings.TrimSpace(ref.KnowledgeID) != "" &&
		strings.TrimSpace(ref.KnowledgeBaseID) != "" &&
		strings.TrimSpace(ref.Content) != ""
}

func knowledgeReferenceKey(ref *types.SearchResult) string {
	if ref == nil {
		return ""
	}
	return ref.KnowledgeBaseID + "\x00" + ref.ID
}
