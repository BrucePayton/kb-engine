package handler

import (
	"errors"
	"net/http"
	"strconv"
	"strings"

	appservice "github.com/Tencent/WeKnora/internal/application/service"
	apperrors "github.com/Tencent/WeKnora/internal/errors"
	"github.com/Tencent/WeKnora/internal/types"
	"github.com/Tencent/WeKnora/internal/types/interfaces"
	"github.com/gin-gonic/gin"
)

type CognitivePathHandler struct {
	service interfaces.CognitivePathService
}

func NewCognitivePathHandler(service interfaces.CognitivePathService) *CognitivePathHandler {
	return &CognitivePathHandler{service: service}
}

type UpsertCognitivePathsRequest struct {
	Paths []*types.CognitivePath `json:"paths" binding:"required"`
}

func cognitiveIntQuery(c *gin.Context, key string, fallback int) int {
	value, err := strconv.Atoi(c.Query(key))
	if err != nil {
		return fallback
	}
	return value
}

// List returns structured paths for exactly one knowledge space. Tenant and KB
// access are enforced by middleware; neither value is accepted from the body.
func (h *CognitivePathHandler) List(c *gin.Context) {
	tenantID := c.GetUint64(types.TenantIDContextKey.String())
	filter := types.CognitivePathFilter{
		KnowledgeBaseIDs:  []string{c.Param("id")},
		Domain:            c.Query("domain"),
		PathType:          c.Query("path_type"),
		SchemaName:        c.Query("schema_name"),
		SubjectType:       c.Query("subject_type"),
		Subject:           c.Query("subject"),
		NodeType:          c.Query("node_type"),
		NodeValue:         c.Query("node_value"),
		RelationType:      c.Query("relation_type"),
		PathKey:           c.Query("path_key"),
		Status:            c.Query("status"),
		Completeness:      c.Query("completeness_status"),
		SourceTitle:       c.Query("source_title"),
		IncludeDeprecated: strings.EqualFold(c.Query("include_deprecated"), "true"),
		Limit:             cognitiveIntQuery(c, "limit", 50),
		Offset:            cognitiveIntQuery(c, "offset", 0),
	}
	paths, total, err := h.service.List(c.Request.Context(), tenantID, filter)
	if err != nil {
		c.Error(apperrors.NewInternalServerError(err.Error()))
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": paths, "total": total})
}

// BatchUpsert is the engineering write boundary. Agent tools never expose
// this operation; the route requires ingest/full API authority plus Admin and
// KB-write authorization.
func (h *CognitivePathHandler) BatchUpsert(c *gin.Context) {
	var req UpsertCognitivePathsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.Error(apperrors.NewBadRequestError("invalid request body").WithDetails(err.Error()))
		return
	}
	tenantID := c.GetUint64(types.TenantIDContextKey.String())
	actorID, _ := types.UserIDFromContext(c.Request.Context())
	paths, err := h.service.UpsertBatch(c.Request.Context(), tenantID, c.Param("id"), actorID, req.Paths)
	if err != nil {
		if errors.Is(err, appservice.ErrCognitivePathEmptyBatch) ||
			errors.Is(err, appservice.ErrCognitivePathBatchLimit) ||
			errors.Is(err, appservice.ErrCognitivePathNilEntry) ||
			errors.Is(err, appservice.ErrCognitivePathNoNodes) ||
			errors.Is(err, appservice.ErrCognitivePathNodeType) ||
			errors.Is(err, appservice.ErrCognitivePathNodeKey) ||
			errors.Is(err, appservice.ErrCognitivePathEdgeRef) {
			c.Error(apperrors.NewBadRequestError(err.Error()))
			return
		}
		c.Error(apperrors.NewInternalServerError(err.Error()))
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": paths, "count": len(paths)})
}
