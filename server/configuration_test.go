package main

import "testing"

func TestResolveCodexbarBin(t *testing.T) {
	t.Setenv(codexbarBinEnv, "")
	if got := resolveCodexbarBin(); got != defaultCodexbarBin {
		t.Fatalf("resolveCodexbarBin default = %q, want %q", got, defaultCodexbarBin)
	}

	t.Setenv(codexbarBinEnv, " /opt/homebrew/bin/codexbar \n")
	if got := resolveCodexbarBin(); got != "/opt/homebrew/bin/codexbar" {
		t.Fatalf("resolveCodexbarBin env = %q", got)
	}
}

func TestResolveRexecdAddr(t *testing.T) {
	t.Setenv(rexecdAddrEnv, "")
	if _, err := resolveRexecdAddr(); err == nil {
		t.Fatal("expected missing env error")
	}

	t.Setenv(rexecdAddrEnv, " macmini.mouriya.lan:8443 ")
	got, err := resolveRexecdAddr()
	if err != nil {
		t.Fatalf("resolveRexecdAddr: %v", err)
	}
	if got != "macmini.mouriya.lan:8443" {
		t.Fatalf("addr = %q", got)
	}
}
