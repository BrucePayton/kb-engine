package repository

import (
	"context"
	"errors"
	"strings"

	"github.com/Tencent/WeKnora/internal/types"
	"github.com/Tencent/WeKnora/internal/types/interfaces"
	"gorm.io/gorm"
)

var ErrCognitivePathNotFound = errors.New("cognitive path not found")

type cognitivePathRepository struct{ db *gorm.DB }

func NewCognitivePathRepository(db *gorm.DB) interfaces.CognitivePathRepository {
	return &cognitivePathRepository{db: db}
}

func cognitiveContains(q *gorm.DB, column, value string) *gorm.DB {
	value = strings.TrimSpace(value)
	if value == "" {
		return q
	}
	return q.Where("LOWER("+column+") LIKE ?", "%"+strings.ToLower(value)+"%")
}

func (r *cognitivePathRepository) List(
	ctx context.Context, tenantID uint64, filter types.CognitivePathFilter,
) ([]*types.CognitivePath, int64, error) {
	q := r.db.WithContext(ctx).Model(&types.CognitivePath{}).
		Where("cognitive_paths.tenant_id = ? AND cognitive_paths.deleted_at IS NULL", tenantID)
	if len(filter.KnowledgeBaseIDs) > 0 {
		q = q.Where("cognitive_paths.knowledge_base_id IN ?", filter.KnowledgeBaseIDs)
	}
	if !filter.IncludeDeprecated {
		q = q.Where("cognitive_paths.status <> ?", types.CognitivePathStatusDeprecated)
	}
	if filter.Status != "" {
		q = q.Where("cognitive_paths.status = ?", filter.Status)
	}
	if filter.Completeness != "" {
		q = q.Where("cognitive_paths.completeness_status = ?", filter.Completeness)
	}
	if filter.PathKey != "" {
		q = q.Where("cognitive_paths.path_key = ?", strings.TrimSpace(filter.PathKey))
	}
	q = cognitiveContains(q, "cognitive_paths.domain", filter.Domain)
	q = cognitiveContains(q, "cognitive_paths.path_type", filter.PathType)
	q = cognitiveContains(q, "cognitive_paths.schema_name", filter.SchemaName)
	q = cognitiveContains(q, "cognitive_paths.subject_type", filter.SubjectType)
	if strings.TrimSpace(filter.Subject) != "" {
		needle := "%" + strings.ToLower(strings.TrimSpace(filter.Subject)) + "%"
		q = q.Where("LOWER(cognitive_paths.subject_key) LIKE ? OR LOWER(cognitive_paths.subject_name) LIKE ?", needle, needle)
	}
	if strings.TrimSpace(filter.NodeType) != "" || strings.TrimSpace(filter.NodeValue) != "" {
		q = q.Joins("JOIN cognitive_path_nodes cpn ON cpn.cognitive_path_id = cognitive_paths.id")
		q = cognitiveContains(q, "cpn.node_type", filter.NodeType)
		q = cognitiveContains(q, "cpn.node_value", filter.NodeValue)
	}
	if strings.TrimSpace(filter.RelationType) != "" {
		q = q.Joins("JOIN cognitive_path_edges cpe ON cpe.cognitive_path_id = cognitive_paths.id")
		q = cognitiveContains(q, "cpe.relation_type", filter.RelationType)
	}
	if strings.TrimSpace(filter.SourceTitle) != "" {
		q = q.Joins("JOIN cognitive_path_sources cps ON cps.cognitive_path_id = cognitive_paths.id").
			Where("LOWER(cps.document_title) LIKE ?", "%"+strings.ToLower(strings.TrimSpace(filter.SourceTitle))+"%")
	}

	var total int64
	if err := q.Session(&gorm.Session{}).Distinct("cognitive_paths.id").Count(&total).Error; err != nil {
		return nil, 0, err
	}
	limit := filter.Limit
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	offset := filter.Offset
	if offset < 0 {
		offset = 0
	}
	var paths []*types.CognitivePath
	err := q.Select("DISTINCT cognitive_paths.*").Preload("Nodes", func(db *gorm.DB) *gorm.DB { return db.Order("position ASC") }).
		Preload("Edges").Preload("Sources").
		Order("cognitive_paths.updated_at DESC, cognitive_paths.id ASC").
		Limit(limit).Offset(offset).Find(&paths).Error
	return paths, total, err
}

func (r *cognitivePathRepository) Get(
	ctx context.Context, tenantID uint64, knowledgeBaseID, id string,
) (*types.CognitivePath, error) {
	var path types.CognitivePath
	err := r.db.WithContext(ctx).Preload("Nodes", func(db *gorm.DB) *gorm.DB { return db.Order("position ASC") }).Preload("Edges").Preload("Sources").
		Where("tenant_id = ? AND knowledge_base_id = ? AND id = ? AND deleted_at IS NULL", tenantID, knowledgeBaseID, id).
		First(&path).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrCognitivePathNotFound
	}
	return &path, err
}

func (r *cognitivePathRepository) UpsertBatch(
	ctx context.Context, paths []*types.CognitivePath,
) ([]*types.CognitivePath, error) {
	result := make([]*types.CognitivePath, 0, len(paths))
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		for _, incoming := range paths {
			var existing types.CognitivePath
			err := tx.Where(
				"tenant_id = ? AND knowledge_base_id = ? AND path_key = ? AND fact_hash = ? AND deleted_at IS NULL",
				incoming.TenantID, incoming.KnowledgeBaseID, incoming.PathKey, incoming.FactHash,
			).First(&existing).Error
			nodes, edges, sources := incoming.Nodes, incoming.Edges, incoming.Sources
			if err == nil {
				incoming.ID = existing.ID
				incoming.CreatedAt = existing.CreatedAt
				incoming.CreatedBy = existing.CreatedBy
				incoming.Revision = existing.Revision + 1
				if err := tx.Model(&types.CognitivePath{}).Where("id = ?", existing.ID).Updates(map[string]any{
					"domain": incoming.Domain, "path_type": incoming.PathType, "schema_name": incoming.SchemaName,
					"schema_version": incoming.SchemaVersion, "subject_type": incoming.SubjectType,
					"subject_key": incoming.SubjectKey, "subject_name": incoming.SubjectName,
					"scope_conditions": incoming.ScopeConditions, "outcome": incoming.Outcome,
					"required_node_types": incoming.RequiredNodeTypes, "missing_node_types": incoming.MissingNodeTypes,
					"completeness_status": incoming.CompletenessStatus, "status": incoming.Status, "metadata": incoming.Metadata,
					"revision": incoming.Revision, "updated_by": incoming.UpdatedBy, "updated_at": incoming.UpdatedAt,
				}).Error; err != nil {
					return err
				}
				for _, child := range []any{&types.CognitivePathNode{}, &types.CognitivePathEdge{}, &types.CognitivePathSource{}} {
					if err := tx.Where("cognitive_path_id = ?", existing.ID).Delete(child).Error; err != nil {
						return err
					}
				}
			} else if errors.Is(err, gorm.ErrRecordNotFound) {
				if err := tx.Omit("Nodes", "Edges", "Sources").Create(incoming).Error; err != nil {
					return err
				}
			} else {
				return err
			}
			for i := range nodes {
				nodes[i].CognitivePathID = incoming.ID
				nodes[i].TenantID = incoming.TenantID
				nodes[i].KnowledgeBaseID = incoming.KnowledgeBaseID
			}
			for i := range edges {
				edges[i].CognitivePathID = incoming.ID
				edges[i].TenantID = incoming.TenantID
				edges[i].KnowledgeBaseID = incoming.KnowledgeBaseID
			}
			for i := range sources {
				sources[i].CognitivePathID = incoming.ID
				sources[i].TenantID = incoming.TenantID
				sources[i].KnowledgeBaseID = incoming.KnowledgeBaseID
			}
			if len(nodes) > 0 {
				if err := tx.Create(&nodes).Error; err != nil {
					return err
				}
			}
			if len(edges) > 0 {
				if err := tx.Create(&edges).Error; err != nil {
					return err
				}
			}
			if len(sources) > 0 {
				if err := tx.Create(&sources).Error; err != nil {
					return err
				}
			}
			incoming.Nodes, incoming.Edges, incoming.Sources = nodes, edges, sources
			result = append(result, incoming)
		}
		return nil
	})
	return result, err
}
