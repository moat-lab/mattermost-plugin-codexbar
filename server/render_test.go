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
}

func TestRenderConfigStdout(t *testing.T) {
	att := renderConfigStdout([]byte(`[]`))
	if att.Color != colorGood {
		t.Fatalf("color = %q, want good", att.Color)
	}
	if !strings.Contains(att.Text, "validates cleanly") {
		t.Fatalf("text = %q", att.Text)
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
}

func fieldContains(fields []*model.SlackAttachmentField, title, want string) bool {
	for _, field := range fields {
		if field.Title == title && strings.Contains(field.Value.(string), want) {
			return true
		}
	}
	return false
}
