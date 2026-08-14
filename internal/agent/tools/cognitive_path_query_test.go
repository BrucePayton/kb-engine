package tools

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/Tencent/WeKnora/internal/types"
)

type cognitiveQueryRepoStub struct {
	filter types.CognitivePathFilter
}

func (r *cognitiveQueryRepoStub) List(_ context.Context, _ uint64, filter types.CognitivePathFilter) ([]*types.CognitivePath, int64, error) {
	r.filter = filter
	return []*types.CognitivePath{{
		Domain: "legal", PathType: "eligibility", SchemaName: "claim", SchemaVersion: "1",
		SubjectType: "claim", SubjectKey: "c-1", SubjectName: "Claim 1", Status: "active", CompletenessStatus: "complete",
		Nodes:   []types.CognitivePathNode{{Position: 1, NodeType: "condition", NodeKey: "age", NodeValue: ">=18"}},
		Sources: []types.CognitivePathSource{{DocumentTitle: "Policy.md", SourceLocation: "Eligibility / Age"}},
	}}, 1, nil
}
func (r *cognitiveQueryRepoStub) Get(context.Context, uint64, string, string) (*types.CognitivePath, error) {
	return nil, nil
}
func (r *cognitiveQueryRepoStub) UpsertBatch(context.Context, []*types.CognitivePath) ([]*types.CognitivePath, error) {
	return nil, nil
}

func TestCognitivePathQueryIsReadOnlyAndScopedToBoundKBs(t *testing.T) {
	repo := &cognitiveQueryRepoStub{}
	tool := NewCognitivePathQueryTool(repo, types.SearchTargets{{Type: types.SearchTargetTypeKnowledgeBase, KnowledgeBaseID: "kb-1"}})
	ctx := context.WithValue(context.Background(), types.TenantIDContextKey, uint64(9))
	args, _ := json.Marshal(CognitivePathQueryInput{Domain: "legal", NodeType: "condition", Limit: 20})
	result, err := tool.Execute(ctx, args)
	if err != nil || !result.Success {
		t.Fatalf("Execute() = %#v, %v", result, err)
	}
	if len(repo.filter.KnowledgeBaseIDs) != 1 || repo.filter.KnowledgeBaseIDs[0] != "kb-1" {
		t.Fatalf("KB scope = %#v", repo.filter.KnowledgeBaseIDs)
	}
	if repo.filter.Domain != "legal" || repo.filter.NodeType != "condition" {
		t.Fatalf("filter = %#v", repo.filter)
	}
	if !strings.Contains(result.Output, "Domain/path: legal / eligibility") || !strings.Contains(result.Output, "Policy.md") {
		t.Fatalf("unexpected output: %s", result.Output)
	}
}
