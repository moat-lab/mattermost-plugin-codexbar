package main

import (
	"reflect"
	"testing"

	"github.com/mattermost/mattermost/server/public/model"
)

func TestBuildCodexbarRequestSummary(t *testing.T) {
	req, err := buildCodexbarRequest("/codexbar", "/opt/homebrew/bin/codexbar", "/Applications/CodexBar.app/Contents/Helpers")
	if err != nil {
		t.Fatalf("buildCodexbarRequest: %v", err)
	}
	if req.Mode != modeSummary {
		t.Fatalf("mode = %s, want %s", req.Mode, modeSummary)
	}
	if len(req.Invocations) != 3 {
		t.Fatalf("invocations = %d, want 3", len(req.Invocations))
	}
	wantUsage := []string{"/opt/homebrew/bin/codexbar", "usage", "--format", "json", "--status", "--provider", "codex", "--source", "web", "--web-timeout", usageWebTimeoutSeconds}
	if !reflect.DeepEqual(req.Invocations[0].Argv, wantUsage) {
		t.Fatalf("usage argv = %#v, want %#v", req.Invocations[0].Argv, wantUsage)
	}
	if req.Invocations[0].Cwd != "/Applications/CodexBar.app/Contents/Helpers" {
		t.Fatalf("usage cwd = %q", req.Invocations[0].Cwd)
	}
	wantClaudeUsage := []string{"/opt/homebrew/bin/codexbar", "usage", "--format", "json", "--status", "--provider", "claude", "--source", "web", "--web-timeout", usageWebTimeoutSeconds}
	if !reflect.DeepEqual(req.Invocations[1].Argv, wantClaudeUsage) {
		t.Fatalf("claude usage argv = %#v, want %#v", req.Invocations[1].Argv, wantClaudeUsage)
	}
	wantCost := []string{"/opt/homebrew/bin/codexbar", "cost", "--format", "json", "--provider", "all"}
	if !reflect.DeepEqual(req.Invocations[2].Argv, wantCost) {
		t.Fatalf("cost argv = %#v, want %#v", req.Invocations[2].Argv, wantCost)
	}
}

func TestBuildCodexbarRequestBareAndSummaryShareOrderingPath(t *testing.T) {
	bare, err := buildCodexbarRequest("/codexbar", "codexbar", "/helpers")
	if err != nil {
		t.Fatalf("build bare request: %v", err)
	}
	explicit, err := buildCodexbarRequest("/codexbar summary", "codexbar", "/helpers")
	if err != nil {
		t.Fatalf("build summary request: %v", err)
	}
	if bare.Mode != modeSummary || explicit.Mode != modeSummary {
		t.Fatalf("modes = %q/%q, want summary/summary", bare.Mode, explicit.Mode)
	}
	if !reflect.DeepEqual(bare.Invocations, explicit.Invocations) {
		t.Fatalf("invocations differ: bare=%#v summary=%#v", bare.Invocations, explicit.Invocations)
	}
}

func TestBuildCodexbarRequestUsageSource(t *testing.T) {
	req, err := buildCodexbarRequest("/codexbar usage --provider claude --source cli", "codexbar", "")
	if err != nil {
		t.Fatalf("buildCodexbarRequest: %v", err)
	}
	want := []string{"codexbar", "usage", "--format", "json", "--status", "--provider", "claude", "--source", "cli"}
	if !reflect.DeepEqual(req.Invocations[0].Argv, want) {
		t.Fatalf("argv = %#v, want %#v", req.Invocations[0].Argv, want)
	}
	if req.Invocations[0].UsageHints != (usageRenderHints{Provider: "claude", Source: "cli"}) {
		t.Fatalf("usage hints = %#v", req.Invocations[0].UsageHints)
	}
}

func TestBuildCodexbarRequestUsageProviderHint(t *testing.T) {
	req, err := buildCodexbarRequest("/codexbar usage gemini", "codexbar", "")
	if err != nil {
		t.Fatalf("buildCodexbarRequest: %v", err)
	}
	if req.Invocations[0].UsageHints != (usageRenderHints{Provider: "gemini"}) {
		t.Fatalf("usage hints = %#v", req.Invocations[0].UsageHints)
	}
}

func TestBuildCodexbarRequestUsageAllSplitsProviders(t *testing.T) {
	req, err := buildCodexbarRequest("/codexbar usage", "codexbar", "/helpers")
	if err != nil {
		t.Fatalf("buildCodexbarRequest: %v", err)
	}
	if len(req.Invocations) != 3 {
		t.Fatalf("invocations = %d, want 3", len(req.Invocations))
	}
	want := [][]string{
		{"codexbar", "usage", "--format", "json", "--status", "--provider", "codex", "--source", "web", "--web-timeout", usageWebTimeoutSeconds},
		{"codexbar", "usage", "--format", "json", "--status", "--provider", "claude", "--source", "web", "--web-timeout", usageWebTimeoutSeconds},
		{"codexbar", "usage", "--format", "json", "--status", "--provider", "gemini", "--source", "api"},
	}
	for i := range want {
		if !reflect.DeepEqual(req.Invocations[i].Argv, want[i]) {
			t.Fatalf("invocation %d argv = %#v, want %#v", i, req.Invocations[i].Argv, want[i])
		}
		if req.Invocations[i].Cwd != "/helpers" {
			t.Fatalf("invocation %d cwd = %q, want /helpers", i, req.Invocations[i].Cwd)
		}
	}
	if req.Invocations[2].UsageHints != (usageRenderHints{Provider: "gemini", Source: "api"}) {
		t.Fatalf("gemini usage hints = %#v", req.Invocations[2].UsageHints)
	}
}

func TestBuildCodexbarRequestCostRefresh(t *testing.T) {
	req, err := buildCodexbarRequest("/codexbar cost --provider=codex --refresh", "codexbar", "")
	if err != nil {
		t.Fatalf("buildCodexbarRequest: %v", err)
	}
	want := []string{"codexbar", "cost", "--format", "json", "--provider", "codex", "--refresh"}
	if !reflect.DeepEqual(req.Invocations[0].Argv, want) {
		t.Fatalf("argv = %#v, want %#v", req.Invocations[0].Argv, want)
	}
}

func TestBuildCodexbarRequestAcceptsCommandTail(t *testing.T) {
	req, err := buildCodexbarRequest("cost codex", "codexbar", "")
	if err != nil {
		t.Fatalf("buildCodexbarRequest tail: %v", err)
	}
	want := []string{"codexbar", "cost", "--format", "json", "--provider", "codex"}
	if !reflect.DeepEqual(req.Invocations[0].Argv, want) {
		t.Fatalf("tail argv = %#v, want %#v", req.Invocations[0].Argv, want)
	}

	req, err = buildCodexbarRequest("config", "codexbar", "")
	if err != nil {
		t.Fatalf("buildCodexbarRequest config tail: %v", err)
	}
	if req.Mode != modeConfig {
		t.Fatalf("tail mode = %q, want %q", req.Mode, modeConfig)
	}

	req, err = buildCodexbarRequest("", "codexbar", "")
	if err != nil {
		t.Fatalf("buildCodexbarRequest empty tail: %v", err)
	}
	if req.Mode != modeSummary {
		t.Fatalf("empty mode = %q, want %q", req.Mode, modeSummary)
	}
}

func TestBuildCodexbarRequestRejectsPassthrough(t *testing.T) {
	_, err := buildCodexbarRequest("/codexbar cache clear", "codexbar", "")
	if err == nil {
		t.Fatal("expected unsupported command error")
	}
}

func TestIsCodexbarBotDM(t *testing.T) {
	botID := "botUserID"
	dm := &model.Channel{
		Type: model.ChannelTypeDirect,
		Name: model.GetDMNameFromIds("userID", botID),
	}
	if !isCodexbarBotDM(dm, botID) {
		t.Fatal("bot DM was rejected")
	}

	humanDM := &model.Channel{
		Type: model.ChannelTypeDirect,
		Name: model.GetDMNameFromIds("userID", "otherID"),
	}
	if isCodexbarBotDM(humanDM, botID) {
		t.Fatal("human DM was accepted")
	}

	public := &model.Channel{Type: model.ChannelTypeOpen}
	if isCodexbarBotDM(public, botID) {
		t.Fatal("public channel was accepted")
	}
}
