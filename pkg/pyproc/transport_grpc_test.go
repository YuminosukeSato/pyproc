package pyproc

import (
	"strings"
	"testing"
)

func TestNewGRPCTransport_NotImplemented(t *testing.T) {
	cfg := TransportConfig{
		Type:    "grpc",
		Address: "localhost:50051",
	}
	logger := NewLogger(LoggingConfig{Level: "debug"})

	transport, err := NewGRPCTransport(cfg, logger)

	if transport != nil {
		t.Error("expected nil transport for unimplemented gRPC")
	}

	if err == nil {
		t.Fatal("expected error for unimplemented gRPC transport")
	}

	if !strings.Contains(err.Error(), "not yet implemented") {
		t.Errorf("expected 'not yet implemented' in error, got: %v", err)
	}
}
