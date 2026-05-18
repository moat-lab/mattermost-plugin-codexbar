package main

import (
	"reflect"
	"testing"

	"github.com/mattermost/mattermost/server/public/model"
)

func TestBuildCodexbarRequestSummary(t *testing.T) {
	req, err := buildCodexbarRequest("/codexbar", "/opt/homebrew/bin/codexbar")
	if err != nil {
		t.Fatalf("buildCodexbarRequest: %v", err)
	}
	if req.Mode != modeSummary {
		t.Fatalf("mode = %s, want %s", req.Mode, modeSummary)
	}
	if len(req.Invocations) != 2 {
		t.Fatalf("invocations = %d, want 2", len(req.Invocations))
	}
	wantUsage := []string{"/opt/homebrew/bin/codexbar", "usage", "--format", "json", "--status", "--provider", "all"}
	if !reflect.DeepEqual(req.Invocations[0].Argv, wantUsage) {
		t.Fatalf("usage argv = %#v, want %#v", req.Invocations[0].Argv, wantUsage)
	}
	wantCost := []string{"/opt/homebrew/bin/codexbar", "cost", "--format", "json", "--provider", "all"}
	if !reflect.DeepEqual(req.Invocations[1].Argv, wantCost) {
		t.Fatalf("cost argv = %#v, want %#v", req.Invocations[1].Argv, wantCost)
	}
}

func TestBuildCodexbarRequestUsageSource(t *testing.T) {
	req, err := buildCodexbarRequest("/codexbar usage --provider claude --source cli", "codexbar")
	if err != nil {
		t.Fatalf("buildCodexbarRequest: %v", err)
	}
	want := []string{"codexbar", "usage", "--format", "json", "--status", "--provider", "claude", "--source", "cli"}
	if !reflect.DeepEqual(req.Invocations[0].Argv, want) {
		t.Fatalf("argv = %#v, want %#v", req.Invocations[0].Argv, want)
	}
}

func TestBuildCodexbarRequestCostRefresh(t *testing.T) {
	req, err := buildCodexbarRequest("/codexbar cost --provider=codex --refresh", "codexbar")
	if err != nil {
		t.Fatalf("buildCodexbarRequest: %v", err)
	}
	want := []string{"codexbar", "cost", "--format", "json", "--provider", "codex", "--refresh"}
	if !reflect.DeepEqual(req.Invocations[0].Argv, want) {
		t.Fatalf("argv = %#v, want %#v", req.Invocations[0].Argv, want)
	}
}

func TestBuildCodexbarRequestRejectsPassthrough(t *testing.T) {
	_, err := buildCodexbarRequest("/codexbar cache clear", "codexbar")
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
