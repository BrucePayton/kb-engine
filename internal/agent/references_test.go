package agent

import (
	"context"
	"fmt"
	"testing"
	"time"

	agenttools "github.com/Tencent/WeKnora/internal/agent/tools"
	"github.com/Tencent/WeKnora/internal/event"
	"github.com/Tencent/WeKnora/internal/types"
	"github.com/stretchr/testify/require"
)

func knowledgeRef(id string) *types.SearchResult {
	return &types.SearchResult{
		ID:                id,
		Content:           "supporting content " + id,
		KnowledgeID:       "doc-1",
		KnowledgeBaseID:   "kb-1",
		KnowledgeTitle:    "Plan.md",
		KnowledgeChannel:  types.ChannelFeishu,
		KnowledgeFilename: "Plan.md",
	}
}

func TestCollectStepKnowledgeReferencesDeduplicatesTrustedResults(t *testing.T) {
	first := knowledgeRef("chunk-1")
	state := &types.AgentState{}
	step := types.AgentStep{ToolCalls: []types.ToolCall{{
		Name: agenttools.ToolKnowledgeSearch,
		Result: &types.ToolResult{Success: true, Data: map[string]interface{}{
			agenttools.InternalKnowledgeReferencesKey: []*types.SearchResult{first, first},
		}},
	}}}

	collectStepKnowledgeReferences(state, step)
	require.Len(t, state.KnowledgeRefs, 1)
	require.Equal(t, "chunk-1", state.KnowledgeRefs[0].ID)
	require.NotSame(t, first, state.KnowledgeRefs[0])
}

func TestFinalKnowledgeReferencesPrefersActuallyCitedChunks(t *testing.T) {
	refs := []*types.SearchResult{knowledgeRef("chunk-1"), knowledgeRef("chunk-2")}
	selected := finalKnowledgeReferences(
		`first claim <kb doc="Plan.md" chunk_id="chunk-2" kb_id="kb-1" />`,
		nil,
		refs,
		true,
	)

	require.Len(t, selected, 1)
	require.Equal(t, "chunk-2", selected[0].ID)
}

func TestFinalKnowledgeReferencesMarksUncitedRetrievalsAsSupportingOnly(t *testing.T) {
	refs := make([]*types.SearchResult, 0, maxFallbackKnowledgeReferences+2)
	for i := 0; i < maxFallbackKnowledgeReferences+2; i++ {
		refs = append(refs, knowledgeRef(fmt.Sprintf("chunk-%d", i)))
	}
	selected := finalKnowledgeReferences("answer without citations", nil, refs, true)
	require.Len(t, selected, maxFallbackKnowledgeReferences)
	for _, ref := range selected {
		require.Equal(t, attributionKindSupportingSearch, ref.Metadata[attributionKindMetadataKey])
	}
	require.Empty(t, finalKnowledgeReferences("answer", []string{"chunk-1"}, refs, false))
}

func TestFinalKnowledgeReferencesUsesPrivateCitationOrderAndDeduplicates(t *testing.T) {
	first := knowledgeRef("chunk-1")
	second := knowledgeRef("chunk-2")
	selected := finalKnowledgeReferences(
		`claim [Plan.md](https://example.feishu.cn/wiki/PlanToken)`,
		[]string{"chunk-2", "chunk-1", "chunk-2"},
		[]*types.SearchResult{first, second},
		true,
	)
	require.Len(t, selected, 2)
	require.Equal(t, "chunk-2", selected[0].ID)
	require.Equal(t, "chunk-1", selected[1].ID)
	require.Equal(t, attributionKindInlineCitation, selected[0].Metadata[attributionKindMetadataKey])
}

func TestEmitCompletionEmitsReferencesBeforeComplete(t *testing.T) {
	engine := newTestEngine(t, &mockChat{})
	var emitted []event.EventType
	engine.eventBus.On(event.EventAgentReferences, func(_ context.Context, evt event.Event) error {
		emitted = append(emitted, evt.Type)
		data := evt.Data.(event.AgentReferencesData)
		require.Len(t, data.References.([]*types.SearchResult), 1)
		require.Equal(t, attributionKindInlineCitation, data.AttributionKind)
		return nil
	})
	engine.eventBus.On(event.EventAgentComplete, func(_ context.Context, evt event.Event) error {
		emitted = append(emitted, evt.Type)
		return nil
	})
	state := &types.AgentState{
		FinalAnswer:           "answer with a source link",
		FinalCitationChunkIDs: []string{"chunk-1"},
		KnowledgeRefs:         []*types.SearchResult{knowledgeRef("chunk-1")},
		ReferencesEligible:    true,
	}

	engine.emitCompletionEvent(context.Background(), state, "session-1", "message-1", time.Now())
	require.Equal(t, []event.EventType{event.EventAgentReferences, event.EventAgentComplete}, emitted)
}

func TestEmitCompletionLabelsUncitedRetrievalsAsSupporting(t *testing.T) {
	engine := newTestEngine(t, &mockChat{})
	engine.eventBus.On(event.EventAgentReferences, func(_ context.Context, evt event.Event) error {
		data := evt.Data.(event.AgentReferencesData)
		require.Equal(t, attributionKindSupportingSearch, data.AttributionKind)
		refs := data.References.([]*types.SearchResult)
		require.Len(t, refs, 1)
		require.Equal(t, attributionKindSupportingSearch, refs[0].Metadata[attributionKindMetadataKey])
		return nil
	})
	state := &types.AgentState{
		FinalAnswer:        "answer without inline citations",
		KnowledgeRefs:      []*types.SearchResult{knowledgeRef("chunk-1")},
		ReferencesEligible: true,
	}

	engine.emitCompletionEvent(context.Background(), state, "session-1", "message-1", time.Now())
}

func TestEmitCompletionSuppressesReferencesForFailedOrCancelledAnswer(t *testing.T) {
	engine := newTestEngine(t, &mockChat{})
	referencesEmitted := false
	engine.eventBus.On(event.EventAgentReferences, func(_ context.Context, _ event.Event) error {
		referencesEmitted = true
		return nil
	})
	state := &types.AgentState{
		FinalAnswer:   "Sorry, I was unable to generate a complete answer.",
		KnowledgeRefs: []*types.SearchResult{knowledgeRef("chunk-1")},
	}

	engine.emitCompletionEvent(context.Background(), state, "session-1", "message-1", time.Now())
	require.False(t, referencesEmitted)
	require.Empty(t, state.KnowledgeRefs)
}
