package service

import (
	"context"
	"errors"
	"testing"

	"github.com/Tencent/WeKnora/internal/types"
)

type cognitivePathRepoStub struct{ saved []*types.CognitivePath }

func (r *cognitivePathRepoStub) List(context.Context, uint64, types.CognitivePathFilter) ([]*types.CognitivePath, int64, error) {
	return r.saved, int64(len(r.saved)), nil
}
func (r *cognitivePathRepoStub) Get(context.Context, uint64, string, string) (*types.CognitivePath, error) {
	return r.saved[0], nil
}
func (r *cognitivePathRepoStub) UpsertBatch(_ context.Context, paths []*types.CognitivePath) ([]*types.CognitivePath, error) {
	r.saved = paths
	return paths, nil
}

func TestCognitivePathServiceSupportsDomainDefinedSchemas(t *testing.T) {
	repo := &cognitivePathRepoStub{}
	svc := NewCognitivePathService(repo)
	paths, err := svc.UpsertBatch(context.Background(), 7, "kb-1", "engineer-1", []*types.CognitivePath{{
		Domain: "compliance", PathType: "eligibility", SchemaName: "vendor-onboarding", SchemaVersion: "2",
		SubjectType: "vendor", SubjectKey: "vendor-a", SubjectName: "供应商A",
		RequiredNodeTypes: []string{"subject", "rule", "evidence", "decision"},
		Nodes: []types.CognitivePathNode{
			{Position: 1, NodeType: "subject", NodeKey: "vendor", NodeValue: "供应商A"},
			{Position: 2, NodeType: "rule", NodeKey: "license", NodeValue: "许可证有效"},
			{Position: 3, NodeType: "decision", NodeKey: "eligible", NodeValue: "准入"},
		},
		Edges: []types.CognitivePathEdge{{FromNodeKey: "vendor", ToNodeKey: "license", RelationType: "requires"}},
	}})
	if err != nil {
		t.Fatalf("UpsertBatch() error = %v", err)
	}
	path := paths[0]
	if path.TenantID != 7 || path.KnowledgeBaseID != "kb-1" {
		t.Fatalf("scope not injected: %#v", path)
	}
	if path.CompletenessStatus != types.CognitivePathCompletenessIncomplete {
		t.Fatalf("completeness = %s", path.CompletenessStatus)
	}
	if len(path.MissingNodeTypes) != 1 || path.MissingNodeTypes[0] != "evidence" {
		t.Fatalf("missing = %#v", path.MissingNodeTypes)
	}
	if path.PathKey == "" || path.FactHash == "" {
		t.Fatal("expected deterministic path hashes")
	}
	if path.Nodes[0].NodeKey == "" || path.Nodes[0].ID == "" {
		t.Fatal("node identity not prepared")
	}
}

func TestCognitivePathWithoutSchemaIsUnassessedNotInspectionSpecific(t *testing.T) {
	repo := &cognitivePathRepoStub{}
	svc := NewCognitivePathService(repo)
	paths, err := svc.UpsertBatch(context.Background(), 1, "kb", "engineer", []*types.CognitivePath{{
		Domain: "troubleshooting", SubjectType: "service", SubjectKey: "checkout",
		Nodes: []types.CognitivePathNode{{NodeType: "symptom", NodeValue: "timeout"}},
	}})
	if err != nil {
		t.Fatal(err)
	}
	if paths[0].CompletenessStatus != types.CognitivePathCompletenessUnassessed {
		t.Fatalf("status = %s", paths[0].CompletenessStatus)
	}
}

func TestCognitivePathServiceRejectsInvalidTopology(t *testing.T) {
	repo := &cognitivePathRepoStub{}
	svc := NewCognitivePathService(repo)
	_, err := svc.UpsertBatch(context.Background(), 1, "kb", "engineer", []*types.CognitivePath{{
		Nodes: []types.CognitivePathNode{{NodeType: "subject", NodeKey: "subject"}},
		Edges: []types.CognitivePathEdge{{FromNodeKey: "subject", ToNodeKey: "missing", RelationType: "leads_to"}},
	}})
	if !errors.Is(err, ErrCognitivePathEdgeRef) {
		t.Fatalf("error = %v, want ErrCognitivePathEdgeRef", err)
	}
	if len(repo.saved) != 0 {
		t.Fatal("invalid path must not reach the repository")
	}
}

func TestCognitivePathServiceAlwaysRecomputesCanonicalHashes(t *testing.T) {
	repo := &cognitivePathRepoStub{}
	svc := NewCognitivePathService(repo)
	paths, err := svc.UpsertBatch(context.Background(), 1, "kb", "engineer", []*types.CognitivePath{{
		Domain: "operations", PathType: "procedure", SubjectType: "service", SubjectKey: "billing",
		PathKey: "caller-controlled-path", FactHash: "caller-controlled-fact",
		Nodes: []types.CognitivePathNode{{NodeType: "step", NodeKey: "restart", NodeValue: "restart worker"}},
	}})
	if err != nil {
		t.Fatal(err)
	}
	if paths[0].PathKey == "caller-controlled-path" || paths[0].FactHash == "caller-controlled-fact" {
		t.Fatal("caller-provided hashes must not bypass canonical grouping")
	}
}
