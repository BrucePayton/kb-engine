package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/Tencent/WeKnora/internal/types"
	"github.com/Tencent/WeKnora/internal/types/interfaces"
)

var (
	ErrCognitivePathEmptyBatch = errors.New("at least one cognitive path is required")
	ErrCognitivePathBatchLimit = errors.New("a batch may contain at most 500 cognitive paths")
	ErrCognitivePathNilEntry   = errors.New("cognitive path must not be null")
	ErrCognitivePathNoNodes    = errors.New("cognitive path must contain at least one node")
	ErrCognitivePathNodeType   = errors.New("every cognitive path node must have a node_type")
	ErrCognitivePathNodeKey    = errors.New("cognitive path node keys and positions must be unique")
	ErrCognitivePathEdgeRef    = errors.New("cognitive path edge must reference existing node keys")
)

type cognitivePathService struct {
	repo interfaces.CognitivePathRepository
}

func NewCognitivePathService(repo interfaces.CognitivePathRepository) interfaces.CognitivePathService {
	return &cognitivePathService{repo: repo}
}

func (s *cognitivePathService) List(ctx context.Context, tenantID uint64, filter types.CognitivePathFilter) ([]*types.CognitivePath, int64, error) {
	return s.repo.List(ctx, tenantID, filter)
}

func (s *cognitivePathService) Get(ctx context.Context, tenantID uint64, knowledgeBaseID, id string) (*types.CognitivePath, error) {
	return s.repo.Get(ctx, tenantID, knowledgeBaseID, id)
}

func normalizeCognitiveValue(value string) string {
	return strings.ToLower(strings.Join(strings.Fields(strings.TrimSpace(value)), " "))
}

func cognitiveHash(parts ...any) string {
	raw, _ := json.Marshal(parts)
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:])
}

func prepareCognitivePath(path *types.CognitivePath, tenantID uint64, kbID, actorID string, now time.Time) error {
	if path == nil {
		return ErrCognitivePathNilEntry
	}
	if len(path.Nodes) == 0 {
		return ErrCognitivePathNoNodes
	}
	path.TenantID = tenantID
	path.KnowledgeBaseID = kbID
	path.ScopeConditions = nonNilJSONMap(path.ScopeConditions)
	path.Outcome = nonNilJSONMap(path.Outcome)
	path.Metadata = nonNilJSONMap(path.Metadata)
	if strings.TrimSpace(path.Domain) == "" {
		path.Domain = "general"
	}
	if strings.TrimSpace(path.PathType) == "" {
		path.PathType = "judgement"
	}
	if strings.TrimSpace(path.SchemaVersion) == "" {
		path.SchemaVersion = "1"
	}
	if path.Status != types.CognitivePathStatusDraft && path.Status != types.CognitivePathStatusDeprecated {
		path.Status = types.CognitivePathStatusActive
	}
	presentTypes := map[string]struct{}{}
	nodeKeys := map[string]struct{}{}
	positions := map[int]struct{}{}
	for i := range path.Nodes {
		node := &path.Nodes[i]
		if node.Position <= 0 {
			node.Position = i + 1
		}
		if strings.TrimSpace(node.NodeKey) == "" {
			node.NodeKey = "n" + strconv.Itoa(node.Position)
		}
		node.NodeType = strings.TrimSpace(node.NodeType)
		if node.NodeType == "" {
			return ErrCognitivePathNodeType
		}
		normalizedKey := normalizeCognitiveValue(node.NodeKey)
		if _, duplicate := nodeKeys[normalizedKey]; duplicate {
			return ErrCognitivePathNodeKey
		}
		if _, duplicate := positions[node.Position]; duplicate {
			return ErrCognitivePathNodeKey
		}
		nodeKeys[normalizedKey] = struct{}{}
		positions[node.Position] = struct{}{}
		presentTypes[normalizeCognitiveValue(node.NodeType)] = struct{}{}
		node.Attributes = nonNilJSONMap(node.Attributes)
	}
	for i := range path.Edges {
		from := normalizeCognitiveValue(path.Edges[i].FromNodeKey)
		to := normalizeCognitiveValue(path.Edges[i].ToNodeKey)
		if _, ok := nodeKeys[from]; !ok {
			return ErrCognitivePathEdgeRef
		}
		if _, ok := nodeKeys[to]; !ok {
			return ErrCognitivePathEdgeRef
		}
	}
	missing := make([]string, 0)
	for _, required := range path.RequiredNodeTypes {
		if _, ok := presentTypes[normalizeCognitiveValue(required)]; !ok {
			missing = append(missing, required)
		}
	}
	path.MissingNodeTypes = missing
	if len(path.RequiredNodeTypes) == 0 {
		path.CompletenessStatus = types.CognitivePathCompletenessUnassessed
	} else if len(missing) == 0 {
		path.CompletenessStatus = types.CognitivePathCompletenessComplete
	} else {
		path.CompletenessStatus = types.CognitivePathCompletenessIncomplete
	}
	scopeJSON, _ := json.Marshal(path.ScopeConditions)
	topology := make([]any, 0, len(path.Nodes)+len(path.Edges))
	for _, node := range path.Nodes {
		topology = append(topology, []any{node.Position, normalizeCognitiveValue(node.NodeType), normalizeCognitiveValue(node.NodeKey)})
	}
	for _, edge := range path.Edges {
		topology = append(topology, []any{normalizeCognitiveValue(edge.FromNodeKey), normalizeCognitiveValue(edge.RelationType), normalizeCognitiveValue(edge.ToNodeKey)})
	}
	scopeParts := []any{normalizeCognitiveValue(path.Domain), normalizeCognitiveValue(path.PathType), normalizeCognitiveValue(path.SchemaName),
		normalizeCognitiveValue(path.SubjectType), normalizeCognitiveValue(path.SubjectKey), string(scopeJSON), topology}
	// Canonical keys are always derived server-side. Accepting caller-provided
	// hashes would let two equivalent paths evade conflict grouping.
	path.PathKey = cognitiveHash(scopeParts...)
	facts := make([]any, 0, len(path.Nodes)+len(path.Edges)+1)
	for _, node := range path.Nodes {
		facts = append(facts, []any{node.NodeKey, node.NodeValue, node.Attributes})
	}
	for _, edge := range path.Edges {
		facts = append(facts, []any{edge.FromNodeKey, edge.RelationType, edge.ToNodeKey, edge.Conditions, edge.Attributes})
	}
	facts = append(facts, path.Outcome)
	path.FactHash = cognitiveHash(append(scopeParts, facts...)...)
	if path.ID == "" {
		path.ID = uuid.NewString()
	}
	if path.Revision <= 0 {
		path.Revision = 1
	}
	path.CreatedBy = actorID
	path.UpdatedBy = actorID
	path.CreatedAt = now
	path.UpdatedAt = now
	for i := range path.Nodes {
		if path.Nodes[i].ID == "" {
			path.Nodes[i].ID = uuid.NewString()
		}
		path.Nodes[i].CreatedAt, path.Nodes[i].UpdatedAt = now, now
	}
	for i := range path.Edges {
		if path.Edges[i].ID == "" {
			path.Edges[i].ID = uuid.NewString()
		}
		path.Edges[i].Conditions, path.Edges[i].Attributes = nonNilJSONMap(path.Edges[i].Conditions), nonNilJSONMap(path.Edges[i].Attributes)
		path.Edges[i].CreatedAt, path.Edges[i].UpdatedAt = now, now
	}
	for i := range path.Sources {
		if path.Sources[i].ID == "" {
			path.Sources[i].ID = uuid.NewString()
		}
		path.Sources[i].Metadata = nonNilJSONMap(path.Sources[i].Metadata)
		path.Sources[i].CreatedAt = now
		path.Sources[i].UpdatedAt = now
	}
	return nil
}

func nonNilJSONMap(value types.JSONMap) types.JSONMap {
	if value == nil {
		return types.JSONMap{}
	}
	return value
}

func (s *cognitivePathService) UpsertBatch(
	ctx context.Context, tenantID uint64, knowledgeBaseID, actorID string, paths []*types.CognitivePath,
) ([]*types.CognitivePath, error) {
	if len(paths) == 0 {
		return nil, ErrCognitivePathEmptyBatch
	}
	if len(paths) > 500 {
		return nil, ErrCognitivePathBatchLimit
	}
	now := time.Now().UTC()
	for _, path := range paths {
		if err := prepareCognitivePath(path, tenantID, knowledgeBaseID, actorID, now); err != nil {
			return nil, err
		}
	}
	return s.repo.UpsertBatch(ctx, paths)
}
