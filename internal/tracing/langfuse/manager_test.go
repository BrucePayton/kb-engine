package langfuse

import (
	"context"
	"testing"
)

// TestManager_DisabledIsNoop verifies that when the manager is disabled the
// public API is safe to call and produces no side effects (no spans exported,
// no panics).
func TestManager_DisabledIsNoop(t *testing.T) {
	m, err := Init(Config{Enabled: false})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if m.Enabled() {
		t.Fatal("expected disabled")
	}
	ctx, trace := m.StartTrace(context.Background(), TraceOptions{Name: "x"})
	trace.Finish(nil, nil)

	_, gen := m.StartGeneration(ctx, GenerationOptions{Name: "g", Model: "m"})
	gen.Finish(nil, nil, nil)

	if err := m.Shutdown(context.Background()); err != nil {
		t.Fatalf("shutdown: %v", err)
	}
}

// TestManager_ForceFlushExportsCompletedTrace exercises the same durability
// boundary used by live HTTP and async requests.
func TestManager_ForceFlushExportsCompletedTrace(t *testing.T) {
	m, exp := newTestManager(t)
	_, trace := m.StartTrace(context.Background(), TraceOptions{Name: "flush-boundary"})
	trace.Finish(map[string]string{"status": "ok"}, nil)
	if err := m.ForceFlush(context.Background()); err != nil {
		t.Fatalf("force flush: %v", err)
	}
	for _, span := range exp.GetSpans() {
		if span.Name == "flush-boundary" {
			return
		}
	}
	t.Fatal("completed root trace was not exported")
}
