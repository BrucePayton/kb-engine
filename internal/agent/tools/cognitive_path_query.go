package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/Tencent/WeKnora/internal/types"
	"github.com/Tencent/WeKnora/internal/types/interfaces"
	"github.com/Tencent/WeKnora/internal/utils"
)

type CognitivePathQueryInput struct {
	Domain             string `json:"domain,omitempty" jsonschema:"Business domain, for example quality_inspection, compliance, troubleshooting, or legal"`
	PathType           string `json:"path_type,omitempty" jsonschema:"Path kind, for example judgement, procedure, eligibility, or causality"`
	SchemaName         string `json:"schema_name,omitempty" jsonschema:"Domain schema name"`
	SubjectType        string `json:"subject_type,omitempty" jsonschema:"Subject/entity type"`
	Subject            string `json:"subject,omitempty" jsonschema:"Subject key or display-name filter"`
	NodeType           string `json:"node_type,omitempty" jsonschema:"Any domain-defined node type"`
	NodeValue          string `json:"node_value,omitempty" jsonschema:"Value text of a matching node"`
	RelationType       string `json:"relation_type,omitempty" jsonschema:"Typed edge/relation filter"`
	PathKey            string `json:"path_key,omitempty" jsonschema:"Exact canonical path key, when known"`
	CompletenessStatus string `json:"completeness_status,omitempty" jsonschema:"complete or incomplete"`
	SourceTitle        string `json:"source_title,omitempty" jsonschema:"Source document title filter"`
	IncludeDeprecated  bool   `json:"include_deprecated,omitempty" jsonschema:"Include deprecated path records"`
	Limit              int    `json:"limit,omitempty" jsonschema:"Maximum rows, 1-100"`
}

type cognitivePathQueryTool struct {
	BaseTool
	repo          interfaces.CognitivePathRepository
	searchTargets types.SearchTargets
}

func NewCognitivePathQueryTool(repo interfaces.CognitivePathRepository, targets types.SearchTargets) *cognitivePathQueryTool {
	return &cognitivePathQueryTool{
		BaseTool: BaseTool{
			name: ToolQueryCognitivePaths,
			description: `Read structured cognitive judgement paths from the Agent's bound knowledge spaces.
The storage is engineering-managed and read-only to Agents. Paths use domain-defined ordered nodes
and typed edges with source-document provenance. Quality inspection is one example, not a built-in
schema. Use this tool to compare scoped rules, detect conflicting facts, or locate missing required
node types in any domain. Never attempt INSERT, UPDATE, or DELETE.`,
			schema: utils.GenerateSchema[CognitivePathQueryInput](),
		},
		repo: repo, searchTargets: targets,
	}
}

func cognitivePathKBIDs(targets types.SearchTargets) []string {
	seen := map[string]struct{}{}
	ids := make([]string, 0, len(targets))
	for _, target := range targets {
		if target == nil || strings.TrimSpace(target.KnowledgeBaseID) == "" {
			continue
		}
		if _, ok := seen[target.KnowledgeBaseID]; ok {
			continue
		}
		seen[target.KnowledgeBaseID] = struct{}{}
		ids = append(ids, target.KnowledgeBaseID)
	}
	return ids
}

func (t *cognitivePathQueryTool) Execute(ctx context.Context, args json.RawMessage) (*types.ToolResult, error) {
	var input CognitivePathQueryInput
	if err := json.Unmarshal(args, &input); err != nil {
		return &types.ToolResult{Success: false, Error: "invalid arguments: " + err.Error()}, nil
	}
	tenantID, ok := types.TenantIDFromContext(ctx)
	if !ok || tenantID == 0 {
		return &types.ToolResult{Success: false, Error: "workspace context is required"}, nil
	}
	kbIDs := cognitivePathKBIDs(t.searchTargets)
	if len(kbIDs) == 0 {
		return &types.ToolResult{Success: false, Error: "no effective knowledge-space scope is available"}, nil
	}
	limit := input.Limit
	if limit <= 0 || limit > 100 {
		limit = 50
	}
	paths, total, err := t.repo.List(ctx, tenantID, types.CognitivePathFilter{
		KnowledgeBaseIDs: kbIDs, Domain: input.Domain, PathType: input.PathType, SchemaName: input.SchemaName,
		SubjectType: input.SubjectType, Subject: input.Subject, NodeType: input.NodeType,
		NodeValue: input.NodeValue, RelationType: input.RelationType,
		PathKey: input.PathKey, Completeness: input.CompletenessStatus, SourceTitle: input.SourceTitle,
		IncludeDeprecated: input.IncludeDeprecated, Limit: limit,
	})
	if err != nil {
		return &types.ToolResult{Success: false, Error: err.Error()}, nil
	}
	var output strings.Builder
	fmt.Fprintf(&output, "Retrieved %d of %d cognitive path(s).\n\n", len(paths), total)
	for i, path := range paths {
		fmt.Fprintf(&output, "Path %d [%s, %s]\n", i+1, path.Status, path.CompletenessStatus)
		fmt.Fprintf(&output, "Domain/path: %s / %s (schema %s@%s)\n", path.Domain, path.PathType, path.SchemaName, path.SchemaVersion)
		fmt.Fprintf(&output, "Subject: %s / %s / %s\n", path.SubjectType, path.SubjectKey, path.SubjectName)
		output.WriteString("Nodes:\n")
		for _, node := range path.Nodes {
			fmt.Fprintf(&output, "  %d. [%s] %s = %s\n", node.Position, node.NodeType, node.NodeKey, node.NodeValue)
		}
		output.WriteString("Edges:\n")
		for _, edge := range path.Edges {
			fmt.Fprintf(&output, "  %s -[%s]-> %s\n", edge.FromNodeKey, edge.RelationType, edge.ToNodeKey)
		}
		if len(path.MissingNodeTypes) > 0 {
			fmt.Fprintf(&output, "Missing required node types: %s\n", strings.Join(path.MissingNodeTypes, ", "))
		}
		for _, source := range path.Sources {
			fmt.Fprintf(&output, "Source: %s — %s\n", source.DocumentTitle, source.SourceLocation)
			if source.EvidenceText != "" {
				fmt.Fprintf(&output, "Evidence: %s\n", source.EvidenceText)
			}
		}
		output.WriteString("\n")
	}
	return &types.ToolResult{Success: true, Output: output.String(), Data: map[string]any{
		"display_type": "cognitive_path_query", "paths": paths, "row_count": len(paths), "total": total,
	}}, nil
}
