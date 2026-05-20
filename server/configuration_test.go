package main

import (
	"encoding/json"
	"os"
	"testing"
)

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

func TestResolveCodexbarCwd(t *testing.T) {
	t.Setenv(codexbarCwdEnv, "")
	if got := resolveCodexbarCwd(); got != "" {
		t.Fatalf("resolveCodexbarCwd default = %q, want empty", got)
	}

	t.Setenv(codexbarCwdEnv, " /Applications/CodexBar.app/Contents/Helpers \n")
	if got := resolveCodexbarCwd(); got != "/Applications/CodexBar.app/Contents/Helpers" {
		t.Fatalf("resolveCodexbarCwd env = %q", got)
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

func TestConfigurationDefaultsKeepAccountValuesVisible(t *testing.T) {
	var cfg configuration
	if cfg.HideAccountValues {
		t.Fatal("HideAccountValues default = true, want false")
	}
}

func TestManifestExposesDisabledAccountHidingSetting(t *testing.T) {
	raw, err := os.ReadFile("../plugin.json")
	if err != nil {
		t.Fatalf("read plugin.json: %v", err)
	}

	var manifest struct {
		SettingsSchema struct {
			Settings []struct {
				Key     string      `json:"key"`
				Type    string      `json:"type"`
				Default interface{} `json:"default"`
			} `json:"settings"`
		} `json:"settings_schema"`
	}
	if err := json.Unmarshal(raw, &manifest); err != nil {
		t.Fatalf("parse plugin.json: %v", err)
	}

	for _, setting := range manifest.SettingsSchema.Settings {
		if setting.Key != "HideAccountValues" {
			continue
		}
		if setting.Type != "bool" {
			t.Fatalf("HideAccountValues type = %q, want bool", setting.Type)
		}
		if setting.Default != "false" {
			t.Fatalf("HideAccountValues default = %#v, want \"false\"", setting.Default)
		}
		return
	}
	t.Fatal("missing HideAccountValues setting")
}
