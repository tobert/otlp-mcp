package main

import (
	"context"
	"fmt"
	"os"
	"time"

	collectortrace "go.opentelemetry.io/proto/otlp/collector/trace/v1"
	commonpb "go.opentelemetry.io/proto/otlp/common/v1"
	resourcepb "go.opentelemetry.io/proto/otlp/resource/v1"
	tracepb "go.opentelemetry.io/proto/otlp/trace/v1"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// Simple program to send test traces to a running OTLP server.
// Usage: go run send_trace.go <endpoint>
// Example: go run send_trace.go 127.0.0.1:38279
func main() {
	if len(os.Args) < 2 {
		fmt.Fprintf(os.Stderr, "Usage: %s <endpoint>\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "Example: %s 127.0.0.1:38279\n", os.Args[0])
		os.Exit(1)
	}

	endpoint := os.Args[1]
	fmt.Printf("📡 Connecting to OTLP endpoint: %s\n", endpoint)

	// Create gRPC connection
	conn, err := grpc.NewClient(endpoint, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		fmt.Fprintf(os.Stderr, "❌ Failed to create grpc client: %v\n", err)
		os.Exit(1)
	}
	defer conn.Close()

	client := collectortrace.NewTraceServiceClient(conn)

	// Create test trace with multiple spans
	now := time.Now()
	testTraceID := []byte{0xde, 0xad, 0xbe, 0xef, 0xca, 0xfe, 0xba, 0xbe, 0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08}

	spans := []*tracepb.Span{
		{
			TraceId:           testTraceID,
			SpanId:            []byte{0x11, 0x11, 0x11, 0x11, 0x11, 0x11, 0x11, 0x11},
			Name:              "http.request",
			Kind:              tracepb.Span_SPAN_KIND_SERVER,
			StartTimeUnixNano: uint64(now.UnixNano()),
			EndTimeUnixNano:   uint64(now.Add(150 * time.Millisecond).UnixNano()),
			Attributes: []*commonpb.KeyValue{
				{
					Key:   "http.method",
					Value: &commonpb.AnyValue{Value: &commonpb.AnyValue_StringValue{StringValue: "GET"}},
				},
				{
					Key:   "http.url",
					Value: &commonpb.AnyValue{Value: &commonpb.AnyValue_StringValue{StringValue: "/api/users"}},
				},
				{
					Key:   "http.status_code",
					Value: &commonpb.AnyValue{Value: &commonpb.AnyValue_IntValue{IntValue: 200}},
				},
			},
			Status: &tracepb.Status{Code: tracepb.Status_STATUS_CODE_OK},
		},
		{
			TraceId:           testTraceID,
			SpanId:            []byte{0x22, 0x22, 0x22, 0x22, 0x22, 0x22, 0x22, 0x22},
			ParentSpanId:      []byte{0x11, 0x11, 0x11, 0x11, 0x11, 0x11, 0x11, 0x11},
			Name:              "db.query",
			Kind:              tracepb.Span_SPAN_KIND_CLIENT,
			StartTimeUnixNano: uint64(now.Add(10 * time.Millisecond).UnixNano()),
			EndTimeUnixNano:   uint64(now.Add(100 * time.Millisecond).UnixNano()),
			Attributes: []*commonpb.KeyValue{
				{
					Key:   "db.system",
					Value: &commonpb.AnyValue{Value: &commonpb.AnyValue_StringValue{StringValue: "postgresql"}},
				},
				{
					Key:   "db.statement",
					Value: &commonpb.AnyValue{Value: &commonpb.AnyValue_StringValue{StringValue: "SELECT * FROM users WHERE id = $1"}},
				},
			},
			Status: &tracepb.Status{Code: tracepb.Status_STATUS_CODE_OK},
		},
	}

	// Send the trace
	fmt.Printf("🚀 Sending trace with %d spans...\n", len(spans))
	_, err = client.Export(context.Background(), &collectortrace.ExportTraceServiceRequest{
		ResourceSpans: []*tracepb.ResourceSpans{
			{
				Resource: &resourcepb.Resource{
					Attributes: []*commonpb.KeyValue{
						{
							Key:   "service.name",
							Value: &commonpb.AnyValue{Value: &commonpb.AnyValue_StringValue{StringValue: "demo-web-service"}},
						},
						{
							Key:   "deployment.environment",
							Value: &commonpb.AnyValue{Value: &commonpb.AnyValue_StringValue{StringValue: "development"}},
						},
						{
							Key:   "service.version",
							Value: &commonpb.AnyValue{Value: &commonpb.AnyValue_StringValue{StringValue: "1.0.0"}},
						},
					},
				},
				ScopeSpans: []*tracepb.ScopeSpans{
					{
						Spans: spans,
					},
				},
			},
		},
	})

	if err != nil {
		fmt.Fprintf(os.Stderr, "❌ Failed to export spans: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("✅ Trace exported successfully!")
	fmt.Printf("📊 Trace ID: deadbeefcafebabe0102030405060708\n")
	fmt.Printf("   - http.request (150ms) → GET /api/users → 200 OK\n")
	fmt.Printf("   - db.query (90ms) → SELECT FROM users\n")
}
