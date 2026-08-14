package router

import (
	"github.com/Tencent/WeKnora/internal/handler"
	"github.com/gin-gonic/gin"
)

func RegisterCognitivePathRoutes(r *gin.RouterGroup, h *handler.CognitivePathHandler, g *rbacGuards) {
	paths := g.apiKeyGroup(r.Group("/knowledge-bases/:id/cognitive-paths"), apiKeyIngest(apiKeyFullAccess()))
	read := paths.With(apiKeyRetrieve(apiKeyIngest(apiKeyFullAccess())))
	{
		read.GET("", g.Viewer(), g.KBAccessRead("id"), h.List)
		paths.POST("/batch-upsert", g.Admin(), g.KBAccessWrite("id"), h.BatchUpsert)
	}
}
