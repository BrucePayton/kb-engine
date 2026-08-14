package router

import (
	"net/http"
	"testing"

	"github.com/gin-gonic/gin"

	"github.com/Tencent/WeKnora/internal/handler"
	"github.com/Tencent/WeKnora/internal/types"
)

func TestCognitivePathRoutesSeparateReadAndEngineeringWriteCapabilities(t *testing.T) {
	gin.SetMode(gin.TestMode)
	g := &rbacGuards{}
	v1 := gin.New().Group("/api/v1")
	RegisterCognitivePathRoutes(v1, &handler.CognitivePathHandler{}, g)

	read := mustLookupAPIKeyPolicy(t, g, http.MethodGet, "/api/v1/knowledge-bases/:id/cognitive-paths")
	if !policyHasCapability(read, types.APIKeyCapabilityRetrieve) {
		t.Fatalf("read capabilities = %#v", read.Capabilities)
	}

	write := mustLookupAPIKeyPolicy(t, g, http.MethodPost, "/api/v1/knowledge-bases/:id/cognitive-paths/batch-upsert")
	if !policyHasCapability(write, types.APIKeyCapabilityIngest) {
		t.Fatalf("write capabilities = %#v", write.Capabilities)
	}
	if policyHasCapability(write, types.APIKeyCapabilityRetrieve) {
		t.Fatalf("engineering write must not be granted by retrieve: %#v", write.Capabilities)
	}
}
