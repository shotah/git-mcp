package server

import "testing"

func TestNew(t *testing.T) {
	t.Parallel()
	if New() == nil {
		t.Fatal("nil server")
	}
	if ServerName != "git" {
		t.Fatalf("ServerName = %q", ServerName)
	}
}
