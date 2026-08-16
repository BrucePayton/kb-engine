package langfuse

import (
	"context"
	"encoding/base64"
	"fmt"
	"strings"

	"github.com/Tencent/WeKnora/internal/logger"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
)

// loggingSpanExporter makes asynchronous BatchSpanProcessor failures visible
// in the normal application logs. Without this wrapper a broken live exporter
// can look enabled at startup while silently delivering no traces.
type loggingSpanExporter struct {
	delegate sdktrace.SpanExporter
	debug    bool
}

func (e *loggingSpanExporter) ExportSpans(ctx context.Context, spans []sdktrace.ReadOnlySpan) error {
	err := e.delegate.ExportSpans(ctx, spans)
	if err != nil {
		logger.Errorf(ctx, "[Langfuse] export failed spans=%d: %v", len(spans), err)
		return err
	}
	if e.debug {
		logger.Debugf(ctx, "[Langfuse] export completed spans=%d", len(spans))
	}
	return nil
}

func (e *loggingSpanExporter) Shutdown(ctx context.Context) error {
	err := e.delegate.Shutdown(ctx)
	if err != nil {
		logger.Errorf(ctx, "[Langfuse] exporter shutdown failed: %v", err)
	}
	return err
}

// newExporter builds an OTLP/HTTP trace exporter pointed at the
// Langfuse v3+ / LiteFuse OTel endpoint (POST /api/public/otel/v1/traces).
//
// Auth is HTTP Basic (public_key:secret_key). The x-langfuse-ingestion-version
// header is the gate opt-in that LiteFuse/Langfuse v3 require for the OTel
// direct-write path (verified against directWriteHelpers.ts — without it the
// server returns 400 "requires Python SDK >= 4.0.0"). The x-langfuse-sdk-*
// markers are sent for parity; WeKnora is a Go client, not the Python SDK.
func newExporter(ctx context.Context, cfg Config) (sdktrace.SpanExporter, error) {
	endpoint := strings.TrimRight(cfg.Host, "/") + "/api/public/otel/v1/traces"
	creds := cfg.PublicKey + ":" + cfg.SecretKey
	opts := []otlptracehttp.Option{
		otlptracehttp.WithEndpointURL(endpoint),
		otlptracehttp.WithHeaders(map[string]string{
			"Authorization":                "Basic " + base64.StdEncoding.EncodeToString([]byte(creds)),
			"x-langfuse-ingestion-version": "4",
			"x-langfuse-sdk-name":          "python",
			"x-langfuse-sdk-version":       langfuseScopeVersion,
		}),
	}
	if cfg.RequestTimeout > 0 {
		opts = append(opts, otlptracehttp.WithTimeout(cfg.RequestTimeout))
	}
	exp, err := otlptracehttp.New(ctx, opts...)
	if err != nil {
		return nil, fmt.Errorf("langfuse: build otlp exporter: %w", err)
	}
	return &loggingSpanExporter{delegate: exp, debug: cfg.Debug}, nil
}
