package main

import (
	"strings"
	"testing"

	rexec "github.com/Mouriya-Emma/rexec-go"
	"github.com/mattermost/mattermost/server/public/model"
)

func TestRenderCostStdout(t *testing.T) {
	stdout := []byte(`[
	  {
	    "provider": "codex",
	    "source": "local",
	    "updatedAt": "2026-05-18T23:25:09Z",
	    "last30DaysCostUSD": 3038.857692,
	    "last30DaysTokens": 3927623011,
	    "sessionCostUSD": 561.673818,
	    "sessionTokens": 774171057,
	    "totals": {
	      "inputTokens": 3915238830,
	      "outputTokens": 12384181,
	      "totalCost": 3038.857692,
	      "totalTokens": 3927623011
	    },
	    "daily": [
	      {
	        "date": "2026-05-19",
	        "totalCost": 561.673818,
	        "totalTokens": 774171057,
	        "modelBreakdowns": [
	          {"modelName": "gpt-5.5", "totalTokens": 774171057, "cost": 561.673818}
	        ]
	      }
	    ]
	  }
	]`)

	attachments := renderCostStdout(stdout)
	if len(attachments) != 1 {
		t.Fatalf("attachments = %d, want 1", len(attachments))
	}
	att := attachments[0]
	if att.Title != "CodexBar cost - Codex" {
		t.Fatalf("title = %q", att.Title)
	}
	if !strings.Contains(att.Text, "Latest day 2026-05-19") {
		t.Fatalf("text missing latest day: %q", att.Text)
	}
	if !fieldContains(att.Fields, "Last 30d", "$3039 / 3.93B tokens") {
		t.Fatalf("missing last 30d field: %#v", att.Fields)
	}
	assertNoInternalCommandFooter(t, att)
}

func TestRenderUsageStdout(t *testing.T) {
	stdout := []byte(`[
	  {
	    "provider": "claude",
	    "source": "web",
	    "status": {
	      "description": "All Systems Operational",
	      "indicator": "none",
	      "updatedAt": "2026-05-18T22:58:36Z"
	    },
	    "usage": {
	      "accountEmail": "ccc88@cornell.edu",
	      "loginMethod": "Claude Max",
	      "primary": {"usedPercent": 0, "resetDescription": "May 19 at 1:20PM", "windowMinutes": 300},
	      "secondary": {"usedPercent": 70, "resetDescription": "May 20 at 5:59AM", "windowMinutes": 10080},
	      "extraRateWindows": [
	        {"id": "claude-design", "title": "Designs", "window": {"usedPercent": 0, "windowMinutes": 10080}}
	      ],
	      "updatedAt": "2026-05-18T23:25:17Z"
	    },
	    "version": "2.1.143"
	  }
	]`)

	attachments := renderUsageStdout(stdout)
	if len(attachments) != 1 {
		t.Fatalf("attachments = %d, want 1", len(attachments))
	}
	att := attachments[0]
	if att.Title != "CodexBar usage - Claude" {
		t.Fatalf("title = %q", att.Title)
	}
	if att.Color != colorWarning {
		t.Fatalf("color = %q, want warning for 70%% secondary", att.Color)
	}
	if !strings.Contains(att.Text, "Designs: 0% used") {
		t.Fatalf("extra window text missing: %q", att.Text)
	}
	if !fieldContains(att.Fields, "Plan", "Claude Max") {
		t.Fatalf("missing plan field: %#v", att.Fields)
	}
	assertNoInternalUsageLimitFieldNames(t, att)
	assertNoInternalCommandFooter(t, att)
}

func TestRenderUsageStdoutReadableLimitLabels(t *testing.T) {
	stdout := []byte(`[
	  {
	    "provider": "codex",
	    "source": "web",
	    "usage": {
	      "accountEmail": "codex@example.com",
	      "loginMethod": "Codex Pro",
	      "primary": {"usedPercent": 12, "resetDescription": "resets at noon", "windowMinutes": 300},
	      "secondary": {"usedPercent": 34, "resetDescription": "resets Sunday", "windowMinutes": 10080}
	    }
	  },
	  {
	    "provider": "claude",
	    "source": "web",
	    "usage": {
	      "accountEmail": "claude@example.com",
	      "loginMethod": "Claude Max",
	      "primary": {"usedPercent": 0, "resetDescription": "May 19 at 1:20PM", "windowMinutes": 300},
	      "secondary": {"usedPercent": 70, "resetDescription": "May 20 at 5:59AM", "windowMinutes": 10080}
	    }
	  },
	  {
	    "provider": "gemini",
	    "source": "api",
	    "usage": {
	      "accountEmail": "gemini@example.com",
	      "loginMethod": "Gemini API",
	      "primary": {"usedPercent": 45, "resetDescription": "resets tomorrow", "windowMinutes": 1440},
	      "secondary": {"usedPercent": 89, "resetDescription": "resets next week", "windowMinutes": 10080},
	      "tertiary": {"usedPercent": 67, "resetDescription": "resets soon", "windowMinutes": 60}
	    }
	  }
	]`)

	attachments := renderUsageStdout(stdout)
	if len(attachments) != 3 {
		t.Fatalf("attachments = %d, want 3", len(attachments))
	}

	for _, att := range attachments {
		assertNoInternalUsageLimitFieldNames(t, att)
	}

	codex := attachmentByTitle(t, attachments, "CodexBar usage - Codex")
	claude := attachmentByTitle(t, attachments, "CodexBar usage - Claude")
	gemini := attachmentByTitle(t, attachments, "CodexBar usage - Gemini")

	if !fieldContains(codex.Fields, "5h limit", "12% used") ||
		!fieldContains(codex.Fields, "5h limit", "resets at noon") ||
		!fieldContains(codex.Fields, "5h limit", "5h window") {
		t.Fatalf("codex short window field lost value/reset/window: %s", fieldsDebugString(codex.Fields))
	}
	if !fieldContains(codex.Fields, "Weekly limit", "34% used") ||
		!fieldContains(codex.Fields, "Weekly limit", "resets Sunday") ||
		!fieldContains(codex.Fields, "Weekly limit", "1w window") {
		t.Fatalf("codex weekly field lost value/reset/window: %s", fieldsDebugString(codex.Fields))
	}
	if !fieldContains(claude.Fields, "5h limit", "0% used") ||
		!fieldContains(claude.Fields, "Weekly limit", "70% used") {
		t.Fatalf("claude readable limit fields missing: %s", fieldsDebugString(claude.Fields))
	}
	if !fieldContains(gemini.Fields, "Daily limit", "45% used") ||
		!fieldContains(gemini.Fields, "Daily limit", "resets tomorrow") ||
		!fieldContains(gemini.Fields, "Daily limit", "1d window") {
		t.Fatalf("gemini daily field lost value/reset/window: %s", fieldsDebugString(gemini.Fields))
	}
	if !fieldContains(gemini.Fields, "Weekly limit", "89% used") ||
		!fieldContains(gemini.Fields, "Hourly limit", "67% used") {
		t.Fatalf("gemini readable limit fields missing: %s", fieldsDebugString(gemini.Fields))
	}
}

func TestRenderUsageProviderError(t *testing.T) {
	stdout := []byte(`[
	  {
	    "provider": "codex",
	    "source": "cli",
	    "error": {
	      "kind": "provider_error",
	      "code": "invalid_data",
	      "message": "Codex returned invalid data"
	    }
	  }
	]`)

	attachments := renderUsageStdout(stdout)
	if len(attachments) != 1 {
		t.Fatalf("attachments = %d, want 1", len(attachments))
	}
	att := attachments[0]
	if att.Color != colorError {
		t.Fatalf("color = %q, want error", att.Color)
	}
	if !strings.Contains(att.Text, "Codex returned invalid data") {
		t.Fatalf("error message missing: %q", att.Text)
	}
	assertNoInternalCommandFooter(t, att)
}

func TestRenderUsageProviderErrorNumericCode(t *testing.T) {
	stdout := []byte(`[
	  {
	    "provider": "gemini",
	    "source": "oauth",
	    "error": {
	      "kind": "provider",
	      "code": 1,
	      "message": "Source is not supported."
	    }
	  }
	]`)

	attachments := renderUsageStdout(stdout)
	if len(attachments) != 1 {
		t.Fatalf("attachments = %d, want 1", len(attachments))
	}
	att := attachments[0]
	if att.Color != colorError {
		t.Fatalf("color = %q, want error", att.Color)
	}
	if !fieldContains(att.Fields, "Code", "1") {
		t.Fatalf("numeric code field missing: %#v", att.Fields)
	}
	assertNoInternalCommandFooter(t, att)
}

func TestRenderConfigStdout(t *testing.T) {
	att := renderConfigStdout([]byte(`[]`))
	if att.Color != colorGood {
		t.Fatalf("color = %q, want good", att.Color)
	}
	if !strings.Contains(att.Text, "validates cleanly") {
		t.Fatalf("text = %q", att.Text)
	}
	assertNoInternalCommandFooter(t, att)
}

func TestRenderEmptyCardsDoNotExposeInternalCommandFooters(t *testing.T) {
	for name, attachments := range map[string][]*model.SlackAttachment{
		"cost":   renderCostStdout([]byte(`[]`)),
		"usage":  renderUsageStdout([]byte(`[]`)),
		"config": {renderConfigStdout([]byte(`[{"path":"CODEXBAR_BIN","message":"missing"}]`))},
	} {
		if len(attachments) == 0 {
			t.Fatalf("%s attachments empty", name)
		}
		for _, att := range attachments {
			assertNoInternalCommandFooter(t, att)
		}
	}
}

func TestRenderOutputsExitError(t *testing.T) {
	attachments := renderOutputs(modeCost, []codexbarOutput{{
		Label:  "cost",
		Result: &rexec.Result{ExitCode: 2, Stderr: []byte("bad args\n")},
	}})
	if len(attachments) != 1 {
		t.Fatalf("attachments = %d, want 1", len(attachments))
	}
	if attachments[0].Color != colorError {
		t.Fatalf("color = %q, want error", attachments[0].Color)
	}
	if !strings.Contains(attachments[0].Text, "bad args") {
		t.Fatalf("text = %q", attachments[0].Text)
	}
	assertNoInternalCommandFooter(t, attachments[0])
}

func TestRenderOutputsUsesStructuredStdoutOnNonZeroExit(t *testing.T) {
	stdout := []byte(`[
	  {
	    "provider": "codex",
	    "source": "cli",
	    "error": {
	      "kind": "provider",
	      "code": "1",
	      "message": "Codex RPC timed out waiting for initialize reply."
	    }
	  }
	]`)

	attachments := renderOutputs(modeUsage, []codexbarOutput{{
		Label:  "usage",
		Result: &rexec.Result{ExitCode: 1, Stdout: stdout},
	}})
	if len(attachments) != 1 {
		t.Fatalf("attachments = %d, want 1", len(attachments))
	}
	if attachments[0].Title != "CodexBar usage - Codex" {
		t.Fatalf("title = %q", attachments[0].Title)
	}
	if attachments[0].Color != colorError {
		t.Fatalf("color = %q, want error", attachments[0].Color)
	}
	if !strings.Contains(attachments[0].Text, "Codex RPC timed out") {
		t.Fatalf("text = %q", attachments[0].Text)
	}
	assertNoInternalCommandFooter(t, attachments[0])
}

func fieldContains(fields []*model.SlackAttachmentField, title, want string) bool {
	for _, field := range fields {
		if field.Title == title && strings.Contains(field.Value.(string), want) {
			return true
		}
	}
	return false
}

func attachmentByTitle(t *testing.T, attachments []*model.SlackAttachment, title string) *model.SlackAttachment {
	t.Helper()
	for _, att := range attachments {
		if att.Title == title {
			return att
		}
	}
	t.Fatalf("missing attachment %q", title)
	return nil
}

func fieldsDebugString(fields []*model.SlackAttachmentField) string {
	parts := make([]string, 0, len(fields))
	for _, field := range fields {
		parts = append(parts, field.Title+"="+field.Value.(string))
	}
	return strings.Join(parts, "; ")
}

func assertNoInternalUsageLimitFieldNames(t *testing.T, att *model.SlackAttachment) {
	t.Helper()
	for _, field := range att.Fields {
		switch field.Title {
		case "Primary", "Secondary", "Tertiary":
			t.Fatalf("usage field exposes internal label %q in %#v", field.Title, att.Fields)
		}
	}
}

func assertNoInternalCommandFooter(t *testing.T, att *model.SlackAttachment) {
	t.Helper()
	footer := strings.ToLower(att.Footer)
	for _, disallowed := range []string{
		"--format json",
		"codexbar cost --format json",
		"codexbar usage --format json",
		"codexbar config validate --format json",
		"codexbar cli",
	} {
		if strings.Contains(footer, disallowed) {
			t.Fatalf("footer exposes internal command %q in %q", disallowed, att.Footer)
		}
	}
}
