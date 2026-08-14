package interfaces

import (
	"context"

	"github.com/Tencent/WeKnora/internal/types"
)

type CognitivePathRepository interface {
	List(ctx context.Context, tenantID uint64, filter types.CognitivePathFilter) ([]*types.CognitivePath, int64, error)
	Get(ctx context.Context, tenantID uint64, knowledgeBaseID, id string) (*types.CognitivePath, error)
	UpsertBatch(ctx context.Context, paths []*types.CognitivePath) ([]*types.CognitivePath, error)
}

type CognitivePathService interface {
	List(ctx context.Context, tenantID uint64, filter types.CognitivePathFilter) ([]*types.CognitivePath, int64, error)
	Get(ctx context.Context, tenantID uint64, knowledgeBaseID, id string) (*types.CognitivePath, error)
	UpsertBatch(ctx context.Context, tenantID uint64, knowledgeBaseID, actorID string, paths []*types.CognitivePath) ([]*types.CognitivePath, error)
}
