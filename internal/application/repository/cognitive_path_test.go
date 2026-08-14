package repository

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"github.com/Tencent/WeKnora/internal/types"
)

func cognitivePathTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file:"+uuid.NewString()+"?mode=memory&cache=shared&_foreign_keys=on"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&types.CognitivePath{}, &types.CognitivePathNode{}, &types.CognitivePathEdge{}, &types.CognitivePathSource{}); err != nil {
		t.Fatal(err)
	}
	return db
}

func TestCognitivePathRepositoryScopesAndFiltersGenericNodes(t *testing.T) {
	db := cognitivePathTestDB(t)
	repo := NewCognitivePathRepository(db)
	now := time.Now().UTC()
	path := &types.CognitivePath{
		ID: uuid.NewString(), TenantID: 1, KnowledgeBaseID: "kb-a", Domain: "support", PathType: "troubleshooting",
		SubjectType: "service", SubjectKey: "checkout", SubjectName: "Checkout", ScopeConditions: types.JSONMap{}, Outcome: types.JSONMap{},
		PathKey: "path", FactHash: "fact", CompletenessStatus: "complete", Status: "active", Metadata: types.JSONMap{}, Revision: 1, CreatedAt: now, UpdatedAt: now,
		Nodes:   []types.CognitivePathNode{{ID: uuid.NewString(), Position: 1, NodeType: "symptom", NodeKey: "timeout", NodeValue: "gateway timeout", Attributes: types.JSONMap{}, CreatedAt: now, UpdatedAt: now}},
		Sources: []types.CognitivePathSource{{ID: uuid.NewString(), DocumentTitle: "Runbook.md", SourceLocation: "Timeouts", Metadata: types.JSONMap{}, CreatedAt: now, UpdatedAt: now}},
	}
	if _, err := repo.UpsertBatch(context.Background(), []*types.CognitivePath{path}); err != nil {
		t.Fatal(err)
	}

	rows, total, err := repo.List(context.Background(), 1, types.CognitivePathFilter{KnowledgeBaseIDs: []string{"kb-a"}, Domain: "support", NodeType: "symptom", NodeValue: "timeout", SourceTitle: "Runbook", Limit: 10})
	if err != nil {
		t.Fatal(err)
	}
	if total != 1 || len(rows) != 1 {
		t.Fatalf("total=%d rows=%d", total, len(rows))
	}
	if len(rows[0].Nodes) != 1 || len(rows[0].Sources) != 1 {
		t.Fatalf("preloads missing: %#v", rows[0])
	}

	otherTenant, total, err := repo.List(context.Background(), 2, types.CognitivePathFilter{KnowledgeBaseIDs: []string{"kb-a"}})
	if err != nil {
		t.Fatal(err)
	}
	if total != 0 || len(otherTenant) != 0 {
		t.Fatal("cross-tenant path leaked")
	}
}
