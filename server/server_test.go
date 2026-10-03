package server

import (
	"context"
	"strings"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"
)

func TestNew(t *testing.T) {
	t.Parallel()
	if New() == nil {
		t.Fatal("nil server")
	}
	if ServerName != "git" {
		t.Fatalf("ServerName = %q", ServerName)
	}
}

func TestInstructions(t *testing.T) {
	t.Parallel()
	s := New()
	resp := s.HandleMessage(context.Background(), []byte(`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2024-11-05","capabilities":{},"clientInfo":{"name":"test","version":"0"}}}`))
	result, ok := resp.(mcp.JSONRPCResponse)
	if !ok {
		t.Fatalf("response %T", resp)
	}
	init, ok := result.Result.(mcp.InitializeResult)
	if !ok {
		t.Fatalf("result %T", result.Result)
	}
	if init.Instructions != Instructions {
		t.Fatalf("instructions = %q", init.Instructions)
	}
	if !strings.Contains(init.Instructions, "stage_update") || !strings.Contains(init.Instructions, "only when the user asked") {
		t.Fatalf("instructions = %q", init.Instructions)
	}
}
