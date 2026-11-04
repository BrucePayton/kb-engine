-- Domain-neutral cognitive paths, isolated by tenant and knowledge space.
-- Domain-specific semantics live in ordered node/edge records rather than
-- fixed columns, so quality inspection is only one possible path schema.
CREATE TABLE IF NOT EXISTS cognitive_paths (
    id VARCHAR(36) PRIMARY KEY,
    tenant_id BIGINT NOT NULL,
    knowledge_base_id VARCHAR(36) NOT NULL,
    domain VARCHAR(128) NOT NULL DEFAULT 'general',
    path_type VARCHAR(128) NOT NULL DEFAULT 'judgement',
    schema_name VARCHAR(128) NOT NULL DEFAULT '',
    schema_version VARCHAR(32) NOT NULL DEFAULT '1',
    subject_type VARCHAR(128) NOT NULL DEFAULT '',
    subject_key VARCHAR(255) NOT NULL DEFAULT '',
    subject_name TEXT NOT NULL DEFAULT '',
    scope_conditions JSONB NOT NULL DEFAULT '{}',
    outcome JSONB NOT NULL DEFAULT '{}',
    required_node_types JSONB NOT NULL DEFAULT '[]',
    missing_node_types JSONB NOT NULL DEFAULT '[]',
    path_key VARCHAR(64) NOT NULL,
    fact_hash VARCHAR(64) NOT NULL,
    completeness_status VARCHAR(16) NOT NULL DEFAULT 'unassessed',
    status VARCHAR(16) NOT NULL DEFAULT 'active',
    metadata JSONB NOT NULL DEFAULT '{}',
    revision INT NOT NULL DEFAULT 1,
    created_by VARCHAR(64) NOT NULL DEFAULT '',
    updated_by VARCHAR(64) NOT NULL DEFAULT '',
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP,
    deleted_at TIMESTAMP WITH TIME ZONE
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_cognitive_paths_active_fact
    ON cognitive_paths (tenant_id, knowledge_base_id, path_key, fact_hash)
    WHERE deleted_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_cognitive_paths_kb_scope
    ON cognitive_paths (tenant_id, knowledge_base_id, domain, path_type, status);
CREATE INDEX IF NOT EXISTS idx_cognitive_paths_subject
    ON cognitive_paths (tenant_id, knowledge_base_id, subject_type, subject_key);
CREATE INDEX IF NOT EXISTS idx_cognitive_paths_path_key
    ON cognitive_paths (tenant_id, knowledge_base_id, path_key);
CREATE INDEX IF NOT EXISTS idx_cognitive_paths_completeness
    ON cognitive_paths (tenant_id, knowledge_base_id, completeness_status);

CREATE TABLE IF NOT EXISTS cognitive_path_nodes (
    id VARCHAR(36) PRIMARY KEY,
    tenant_id BIGINT NOT NULL,
    knowledge_base_id VARCHAR(36) NOT NULL,
    cognitive_path_id VARCHAR(36) NOT NULL,
    position INT NOT NULL,
    node_type VARCHAR(128) NOT NULL,
    node_key VARCHAR(255) NOT NULL,
    node_value TEXT NOT NULL DEFAULT '',
    attributes JSONB NOT NULL DEFAULT '{}',
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT fk_cognitive_path_nodes_path
        FOREIGN KEY (cognitive_path_id) REFERENCES cognitive_paths(id) ON DELETE CASCADE
);
CREATE UNIQUE INDEX IF NOT EXISTS idx_cognitive_path_nodes_position
    ON cognitive_path_nodes (cognitive_path_id, position);
CREATE INDEX IF NOT EXISTS idx_cognitive_path_nodes_lookup
    ON cognitive_path_nodes (tenant_id, knowledge_base_id, node_type, node_key);

CREATE TABLE IF NOT EXISTS cognitive_path_edges (
    id VARCHAR(36) PRIMARY KEY,
    tenant_id BIGINT NOT NULL,
    knowledge_base_id VARCHAR(36) NOT NULL,
    cognitive_path_id VARCHAR(36) NOT NULL,
    from_node_key VARCHAR(255) NOT NULL,
    to_node_key VARCHAR(255) NOT NULL,
    relation_type VARCHAR(128) NOT NULL,
    conditions JSONB NOT NULL DEFAULT '{}',
    attributes JSONB NOT NULL DEFAULT '{}',
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT fk_cognitive_path_edges_path
        FOREIGN KEY (cognitive_path_id) REFERENCES cognitive_paths(id) ON DELETE CASCADE
);
CREATE INDEX IF NOT EXISTS idx_cognitive_path_edges_lookup
    ON cognitive_path_edges (tenant_id, knowledge_base_id, relation_type, from_node_key, to_node_key);

CREATE TABLE IF NOT EXISTS cognitive_path_sources (
    id VARCHAR(36) PRIMARY KEY,
    tenant_id BIGINT NOT NULL,
    knowledge_base_id VARCHAR(36) NOT NULL,
    cognitive_path_id VARCHAR(36) NOT NULL,
    knowledge_id VARCHAR(36) NOT NULL DEFAULT '',
    chunk_id VARCHAR(36) NOT NULL DEFAULT '',
    wiki_slug VARCHAR(255) NOT NULL DEFAULT '',
    document_title TEXT NOT NULL DEFAULT '',
    source_location TEXT NOT NULL DEFAULT '',
    evidence_text TEXT NOT NULL DEFAULT '',
    metadata JSONB NOT NULL DEFAULT '{}',
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT fk_cognitive_path_sources_path
        FOREIGN KEY (cognitive_path_id) REFERENCES cognitive_paths(id) ON DELETE CASCADE
);
CREATE INDEX IF NOT EXISTS idx_cognitive_path_sources_path
    ON cognitive_path_sources (tenant_id, knowledge_base_id, cognitive_path_id);
CREATE INDEX IF NOT EXISTS idx_cognitive_path_sources_document
    ON cognitive_path_sources (tenant_id, knowledge_base_id, knowledge_id);
