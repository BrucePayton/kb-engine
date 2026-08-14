package types

import "time"

const (
	CognitivePathStatusActive     = "active"
	CognitivePathStatusDraft      = "draft"
	CognitivePathStatusDeprecated = "deprecated"

	CognitivePathCompletenessComplete   = "complete"
	CognitivePathCompletenessIncomplete = "incomplete"
	CognitivePathCompletenessUnassessed = "unassessed"
)

// CognitivePath is deliberately domain-neutral. A quality-inspection chain,
// a compliance decision, a troubleshooting flow, or a legal eligibility rule
// are all represented by their own ordered Nodes and typed Edges.
type CognitivePath struct {
	ID                 string                `json:"id" gorm:"type:varchar(36);primaryKey"`
	TenantID           uint64                `json:"tenant_id" gorm:"index"`
	KnowledgeBaseID    string                `json:"knowledge_base_id" gorm:"type:varchar(36);index"`
	Domain             string                `json:"domain" gorm:"type:varchar(128);index"`
	PathType           string                `json:"path_type" gorm:"type:varchar(128);index"`
	SchemaName         string                `json:"schema_name"`
	SchemaVersion      string                `json:"schema_version"`
	SubjectType        string                `json:"subject_type" gorm:"type:varchar(128);index"`
	SubjectKey         string                `json:"subject_key" gorm:"type:varchar(255);index"`
	SubjectName        string                `json:"subject_name"`
	ScopeConditions    JSONMap               `json:"scope_conditions" gorm:"type:jsonb"`
	Outcome            JSONMap               `json:"outcome" gorm:"type:jsonb"`
	RequiredNodeTypes  []string              `json:"required_node_types" gorm:"serializer:json;type:jsonb"`
	MissingNodeTypes   []string              `json:"missing_node_types" gorm:"serializer:json;type:jsonb"`
	PathKey            string                `json:"path_key" gorm:"type:varchar(64);index"`
	FactHash           string                `json:"fact_hash" gorm:"type:varchar(64)"`
	CompletenessStatus string                `json:"completeness_status" gorm:"type:varchar(16);index"`
	Status             string                `json:"status" gorm:"type:varchar(16);index"`
	Metadata           JSONMap               `json:"metadata" gorm:"type:jsonb"`
	Revision           int                   `json:"revision"`
	CreatedBy          string                `json:"created_by"`
	UpdatedBy          string                `json:"updated_by"`
	CreatedAt          time.Time             `json:"created_at"`
	UpdatedAt          time.Time             `json:"updated_at"`
	DeletedAt          *time.Time            `json:"-"`
	Nodes              []CognitivePathNode   `json:"nodes" gorm:"foreignKey:CognitivePathID"`
	Edges              []CognitivePathEdge   `json:"edges" gorm:"foreignKey:CognitivePathID"`
	Sources            []CognitivePathSource `json:"sources" gorm:"foreignKey:CognitivePathID"`
}

func (CognitivePath) TableName() string { return "cognitive_paths" }

type CognitivePathNode struct {
	ID              string    `json:"id" gorm:"type:varchar(36);primaryKey"`
	TenantID        uint64    `json:"tenant_id"`
	KnowledgeBaseID string    `json:"knowledge_base_id"`
	CognitivePathID string    `json:"cognitive_path_id"`
	Position        int       `json:"position"`
	NodeType        string    `json:"node_type"`
	NodeKey         string    `json:"node_key"`
	NodeValue       string    `json:"node_value"`
	Attributes      JSONMap   `json:"attributes" gorm:"type:jsonb"`
	CreatedAt       time.Time `json:"created_at"`
	UpdatedAt       time.Time `json:"updated_at"`
}

func (CognitivePathNode) TableName() string { return "cognitive_path_nodes" }

type CognitivePathEdge struct {
	ID              string    `json:"id" gorm:"type:varchar(36);primaryKey"`
	TenantID        uint64    `json:"tenant_id"`
	KnowledgeBaseID string    `json:"knowledge_base_id"`
	CognitivePathID string    `json:"cognitive_path_id"`
	FromNodeKey     string    `json:"from_node_key"`
	ToNodeKey       string    `json:"to_node_key"`
	RelationType    string    `json:"relation_type"`
	Conditions      JSONMap   `json:"conditions" gorm:"type:jsonb"`
	Attributes      JSONMap   `json:"attributes" gorm:"type:jsonb"`
	CreatedAt       time.Time `json:"created_at"`
	UpdatedAt       time.Time `json:"updated_at"`
}

func (CognitivePathEdge) TableName() string { return "cognitive_path_edges" }

type CognitivePathSource struct {
	ID              string    `json:"id" gorm:"type:varchar(36);primaryKey"`
	TenantID        uint64    `json:"tenant_id"`
	KnowledgeBaseID string    `json:"knowledge_base_id"`
	CognitivePathID string    `json:"cognitive_path_id"`
	KnowledgeID     string    `json:"knowledge_id"`
	ChunkID         string    `json:"chunk_id"`
	WikiSlug        string    `json:"wiki_slug"`
	DocumentTitle   string    `json:"document_title"`
	SourceLocation  string    `json:"source_location"`
	EvidenceText    string    `json:"evidence_text"`
	Metadata        JSONMap   `json:"metadata" gorm:"type:jsonb"`
	CreatedAt       time.Time `json:"created_at"`
	UpdatedAt       time.Time `json:"updated_at"`
}

func (CognitivePathSource) TableName() string { return "cognitive_path_sources" }

type CognitivePathFilter struct {
	KnowledgeBaseIDs  []string
	Domain            string
	PathType          string
	SchemaName        string
	SubjectType       string
	Subject           string
	NodeType          string
	NodeValue         string
	RelationType      string
	PathKey           string
	Status            string
	Completeness      string
	SourceTitle       string
	IncludeDeprecated bool
	Limit             int
	Offset            int
}
