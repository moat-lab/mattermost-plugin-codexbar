package main

import (
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"strings"
	"time"

	"github.com/mattermost/mattermost/server/public/model"
)

const (
	colorGood    = "#2E7D32"
	colorAccent  = "#386FA4"
	colorWarning = "#B7791F"
	colorError   = "#C53030"
)

type tokenTotals struct {
	InputTokens         int64   `json:"inputTokens"`
	OutputTokens        int64   `json:"outputTokens"`
	CacheCreationTokens int64   `json:"cacheCreationTokens"`
	CacheReadTokens     int64   `json:"cacheReadTokens"`
	TotalTokens         int64   `json:"totalTokens"`
	TotalCost           float64 `json:"totalCost"`
}

type costReport struct {
	Provider          string      `json:"provider"`
	Source            string      `json:"source"`
	UpdatedAt         string      `json:"updatedAt"`
	Last30DaysCostUSD float64     `json:"last30DaysCostUSD"`
	Last30DaysTokens  int64       `json:"last30DaysTokens"`
	SessionCostUSD    float64     `json:"sessionCostUSD"`
	SessionTokens     int64       `json:"sessionTokens"`
	HistoryDays       int         `json:"historyDays"`
	Totals            tokenTotals `json:"totals"`
	Daily             []dailyCost `json:"daily"`
}

type dailyCost struct {
	Date                string           `json:"date"`
	InputTokens         int64            `json:"inputTokens"`
	OutputTokens        int64            `json:"outputTokens"`
	CacheCreationTokens int64            `json:"cacheCreationTokens"`
	CacheReadTokens     int64            `json:"cacheReadTokens"`
	TotalCost           float64          `json:"totalCost"`
	TotalTokens         int64            `json:"totalTokens"`
	ModelBreakdowns     []modelBreakdown `json:"modelBreakdowns"`
}

type modelBreakdown struct {
	ModelName   string  `json:"modelName"`
	TotalTokens int64   `json:"totalTokens"`
	Cost        float64 `json:"cost"`
}

type usageReport struct {
	Provider string          `json:"provider"`
	Source   string          `json:"source"`
	Version  string          `json:"version"`
	Status   *providerStatus `json:"status"`
	Usage    *providerUsage  `json:"usage"`
	Credits  *creditsInfo    `json:"credits"`
	Error    *providerError  `json:"error"`
}

type providerStatus struct {
	Description string `json:"description"`
	Indicator   string `json:"indicator"`
	UpdatedAt   string `json:"updatedAt"`
	URL         string `json:"url"`
}

type providerUsage struct {
	AccountEmail        string            `json:"accountEmail"`
	AccountOrganization string            `json:"accountOrganization"`
	LoginMethod         string            `json:"loginMethod"`
	Primary             *usageWindow      `json:"primary"`
	Secondary           *usageWindow      `json:"secondary"`
	Tertiary            *usageWindow      `json:"tertiary"`
	ExtraRateWindows    []extraRateWindow `json:"extraRateWindows"`
	UpdatedAt           string            `json:"updatedAt"`
}

type usageWindow struct {
	UsedPercent      *float64 `json:"usedPercent"`
	WindowMinutes    int      `json:"windowMinutes"`
	ResetDescription string   `json:"resetDescription"`
	ResetsAt         string   `json:"resetsAt"`
}

type extraRateWindow struct {
	ID     string      `json:"id"`
	Title  string      `json:"title"`
	Window usageWindow `json:"window"`
}

type creditsInfo struct {
	Remaining float64 `json:"remaining"`
	UpdatedAt string  `json:"updatedAt"`
}

type providerError struct {
	Kind    string `json:"kind"`
	Code    string `json:"code"`
	Message string `json:"message"`
}

func renderOutputs(mode commandMode, outputs []codexbarOutput) []*model.SlackAttachment {
	var attachments []*model.SlackAttachment
	for _, out := range outputs {
		if out.Err != nil {
			attachments = append(attachments, renderInvocationError(out))
			continue
		}
		if out.Result == nil {
			attachments = append(attachments, renderInvocationError(codexbarOutput{Label: out.Label, Err: fmt.Errorf("empty result")}))
			continue
		}
		if out.Result.ExitCode != 0 {
			if len(out.Result.Stdout) > 0 {
				attachments = append(attachments, renderStdoutByLabel(reqModeTitle(mode), out.Label, out.Result.Stdout)...)
				continue
			}
			attachments = append(attachments, renderExitError(out))
			continue
		}

		attachments = append(attachments, renderStdoutByLabel(reqModeTitle(mode), out.Label, out.Result.Stdout)...)
	}
	if len(attachments) == 0 {
		attachments = append(attachments, &model.SlackAttachment{
			Title: "CodexBar",
			Text:  "No output was returned.",
			Color: colorWarning,
		})
	}
	return attachments
}

func reqModeTitle(mode commandMode) string {
	return "CodexBar " + string(mode)
}

func renderStdoutByLabel(title, label string, stdout []byte) []*model.SlackAttachment {
	switch label {
	case "cost":
		return renderCostStdout(stdout)
	case "usage":
		return renderUsageStdout(stdout)
	case "config":
		return []*model.SlackAttachment{renderConfigStdout(stdout)}
	default:
		return []*model.SlackAttachment{renderGenericJSON(title, stdout)}
	}
}

func renderCostStdout(stdout []byte) []*model.SlackAttachment {
	var reports []costReport
	if err := json.Unmarshal(stdout, &reports); err != nil {
		return []*model.SlackAttachment{renderJSONError("CodexBar cost", err, stdout)}
	}
	if len(reports) == 0 {
		return []*model.SlackAttachment{{
			Title:  "CodexBar cost",
			Text:   "No local cost data was returned.",
			Color:  colorWarning,
			Footer: "codexbar cost --format json",
		}}
	}
	sort.SliceStable(reports, func(i, j int) bool {
		return reports[i].Provider < reports[j].Provider
	})
	out := make([]*model.SlackAttachment, 0, len(reports))
	for _, report := range reports {
		out = append(out, renderCostReport(report))
	}
	return out
}

func renderCostReport(report costReport) *model.SlackAttachment {
	provider := displayProvider(report.Provider)
	historyDays := report.HistoryDays
	if historyDays == 0 {
		historyDays = 30
	}
	fields := []*model.SlackAttachmentField{
		shortField("Provider", provider),
		shortField("Source", emptyAs(report.Source, "unknown")),
		shortField(fmt.Sprintf("Last %dd", historyDays), fmt.Sprintf("%s / %s tokens", money(report.Last30DaysCostUSD), compactInt(report.Last30DaysTokens))),
		shortField("Current session", fmt.Sprintf("%s / %s tokens", money(report.SessionCostUSD), compactInt(report.SessionTokens))),
		shortField("Input / output", fmt.Sprintf("%s / %s", compactInt(report.Totals.InputTokens), compactInt(report.Totals.OutputTokens))),
		shortField("Cache", cacheSummary(report.Totals)),
	}
	text := latestCostLine(report)
	if text == "" {
		text = "Local token-cost scan completed."
	}
	return &model.SlackAttachment{
		Title:  "CodexBar cost - " + provider,
		Text:   text,
		Color:  colorAccent,
		Fields: fields,
		Footer: "codexbar cost --format json" + updatedSuffix(report.UpdatedAt),
	}
}

func renderUsageStdout(stdout []byte) []*model.SlackAttachment {
	var reports []usageReport
	if err := json.Unmarshal(stdout, &reports); err != nil {
		return []*model.SlackAttachment{renderJSONError("CodexBar usage", err, stdout)}
	}
	if len(reports) == 0 {
		return []*model.SlackAttachment{{
			Title:  "CodexBar usage",
			Text:   "No usage providers were returned.",
			Color:  colorWarning,
			Footer: "codexbar usage --format json --status",
		}}
	}
	sort.SliceStable(reports, func(i, j int) bool {
		return reports[i].Provider < reports[j].Provider
	})
	out := make([]*model.SlackAttachment, 0, len(reports))
	for _, report := range reports {
		out = append(out, renderUsageReport(report))
	}
	return out
}

func renderUsageReport(report usageReport) *model.SlackAttachment {
	provider := displayProvider(report.Provider)
	if report.Error != nil {
		return renderProviderError("CodexBar usage - "+provider, report.Provider, report.Source, report.Error)
	}

	fields := []*model.SlackAttachmentField{
		shortField("Provider", provider),
		shortField("Source", emptyAs(report.Source, "unknown")),
	}
	if report.Usage != nil {
		fields = append(fields,
			shortField("Account", firstNonEmpty(report.Usage.AccountEmail, report.Usage.AccountOrganization, "unknown")),
			shortField("Plan", emptyAs(report.Usage.LoginMethod, "unknown")),
		)
		fields = append(fields, usageWindowFields(report.Usage)...)
	}
	if report.Status != nil {
		fields = append(fields, shortField("Status", statusText(report.Status)))
	}
	if report.Version != "" {
		fields = append(fields, shortField("Version", report.Version))
	}
	if report.Credits != nil {
		fields = append(fields, shortField("Credits", fmt.Sprintf("%s remaining", trimFloat(report.Credits.Remaining))))
	}

	text := extraWindowsText(report.Usage)
	if text == "" {
		text = "Live provider usage fetched from CodexBar."
	}
	return &model.SlackAttachment{
		Title:  "CodexBar usage - " + provider,
		Text:   text,
		Color:  usageColor(report.Usage, report.Status),
		Fields: fields,
		Footer: "codexbar usage --format json --status" + usageUpdatedSuffix(report),
	}
}

func renderConfigStdout(stdout []byte) *model.SlackAttachment {
	var entries []json.RawMessage
	if err := json.Unmarshal(stdout, &entries); err != nil {
		return renderJSONError("CodexBar config", err, stdout)
	}
	if len(entries) == 0 {
		return &model.SlackAttachment{
			Title:  "CodexBar config",
			Text:   "Configuration validates cleanly.",
			Color:  colorGood,
			Footer: "codexbar config validate --format json",
		}
	}
	pretty, err := prettyJSON(stdout)
	if err != nil {
		pretty = truncate(string(stdout), 2000)
	}
	return &model.SlackAttachment{
		Title:  "CodexBar config - validation findings",
		Text:   codeBlock(pretty),
		Color:  colorWarning,
		Footer: "codexbar config validate --format json",
	}
}

func renderHelp() []*model.SlackAttachment {
	return []*model.SlackAttachment{{
		Title: "CodexBar commands",
		Text: strings.Join([]string{
			"`/codexbar` or `/codexbar summary` - usage limits plus local cost cards.",
			"`/codexbar usage [codex|claude|gemini|all] [--source=auto|web|cli|oauth|api]` - live usage/status cards.",
			"`/codexbar cost [codex|claude|gemini|all] [--refresh]` - local token-cost cards.",
			"`/codexbar config` - validate CodexBar config.",
		}, "\n"),
		Color:  colorAccent,
		Footer: "Private bot DM only. CodexBar CLI remains the source of truth.",
	}}
}

func renderInvocationError(out codexbarOutput) *model.SlackAttachment {
	return &model.SlackAttachment{
		Title:  "CodexBar " + out.Label + " - remote error",
		Text:   fmt.Sprintf("`%v`", out.Err),
		Color:  colorError,
		Footer: "rexec-go",
	}
}

func renderExitError(out codexbarOutput) *model.SlackAttachment {
	stderr := strings.TrimSpace(string(out.Result.Stderr))
	stdout := strings.TrimSpace(string(out.Result.Stdout))
	text := stderr
	if text == "" {
		text = stdout
	}
	if text == "" {
		text = fmt.Sprintf("process exited with code %d", out.Result.ExitCode)
	}
	return &model.SlackAttachment{
		Title:  fmt.Sprintf("CodexBar %s - exit %d", out.Label, out.Result.ExitCode),
		Text:   truncate(text, 2000),
		Color:  colorError,
		Footer: "codexbar CLI",
	}
}

func renderProviderError(title, provider, source string, err *providerError) *model.SlackAttachment {
	fields := []*model.SlackAttachmentField{
		shortField("Provider", displayProvider(provider)),
		shortField("Source", emptyAs(source, "unknown")),
	}
	if err.Kind != "" {
		fields = append(fields, shortField("Kind", err.Kind))
	}
	if err.Code != "" {
		fields = append(fields, shortField("Code", err.Code))
	}
	msg := firstNonEmpty(err.Message, "CodexBar returned an error for this provider.")
	return &model.SlackAttachment{
		Title:  title,
		Text:   msg,
		Color:  colorError,
		Fields: fields,
		Footer: "codexbar usage --format json --status",
	}
}

func renderJSONError(title string, err error, stdout []byte) *model.SlackAttachment {
	return &model.SlackAttachment{
		Title: title + " - JSON parse error",
		Text:  fmt.Sprintf("%v\n\n%s", err, codeBlock(truncate(string(stdout), 1500))),
		Color: colorError,
	}
}

func renderGenericJSON(title string, stdout []byte) *model.SlackAttachment {
	pretty, err := prettyJSON(stdout)
	if err != nil {
		return renderJSONError(title, err, stdout)
	}
	return &model.SlackAttachment{
		Title: title,
		Text:  codeBlock(pretty),
		Color: colorAccent,
	}
}

func usageWindowFields(usage *providerUsage) []*model.SlackAttachmentField {
	if usage == nil {
		return nil
	}
	fields := []*model.SlackAttachmentField{}
	if usage.Primary != nil {
		fields = append(fields, shortField("Primary", formatWindow(usage.Primary)))
	}
	if usage.Secondary != nil {
		fields = append(fields, shortField("Secondary", formatWindow(usage.Secondary)))
	}
	if usage.Tertiary != nil {
		fields = append(fields, shortField("Tertiary", formatWindow(usage.Tertiary)))
	}
	return fields
}

func formatWindow(w *usageWindow) string {
	if w == nil {
		return "n/a"
	}
	parts := []string{}
	if w.UsedPercent != nil {
		parts = append(parts, percent(*w.UsedPercent)+" used")
	}
	if w.ResetDescription != "" {
		parts = append(parts, w.ResetDescription)
	} else if w.ResetsAt != "" {
		parts = append(parts, "resets "+formatTime(w.ResetsAt))
	}
	if w.WindowMinutes > 0 {
		parts = append(parts, windowLength(w.WindowMinutes))
	}
	if len(parts) == 0 {
		return "n/a"
	}
	return strings.Join(parts, " · ")
}

func latestCostLine(report costReport) string {
	if len(report.Daily) == 0 {
		return ""
	}
	latest := report.Daily[0]
	for _, day := range report.Daily[1:] {
		if day.Date > latest.Date {
			latest = day
		}
	}
	modelText := ""
	if len(latest.ModelBreakdowns) > 0 {
		top := latest.ModelBreakdowns[0]
		for _, mb := range latest.ModelBreakdowns[1:] {
			if mb.TotalTokens > top.TotalTokens {
				top = mb
			}
		}
		modelText = fmt.Sprintf(" · top model %s (%s tokens)", top.ModelName, compactInt(top.TotalTokens))
	}
	return fmt.Sprintf("Latest day %s: %s / %s tokens%s.", latest.Date, money(latest.TotalCost), compactInt(latest.TotalTokens), modelText)
}

func extraWindowsText(usage *providerUsage) string {
	if usage == nil || len(usage.ExtraRateWindows) == 0 {
		return ""
	}
	lines := make([]string, 0, len(usage.ExtraRateWindows))
	for _, item := range usage.ExtraRateWindows {
		name := firstNonEmpty(item.Title, item.ID, "extra")
		win := item.Window
		lines = append(lines, fmt.Sprintf("%s: %s", name, formatWindow(&win)))
	}
	return strings.Join(lines, "\n")
}

func statusText(status *providerStatus) string {
	if status == nil {
		return "unknown"
	}
	desc := emptyAs(status.Description, "unknown")
	if status.Indicator == "" || status.Indicator == "none" {
		return desc
	}
	return desc + " (" + status.Indicator + ")"
}

func usageColor(usage *providerUsage, status *providerStatus) string {
	if status != nil && status.Indicator != "" && status.Indicator != "none" {
		return colorWarning
	}
	maxUsed := 0.0
	if usage != nil {
		for _, w := range []*usageWindow{usage.Primary, usage.Secondary, usage.Tertiary} {
			if w != nil && w.UsedPercent != nil && *w.UsedPercent > maxUsed {
				maxUsed = *w.UsedPercent
			}
		}
	}
	switch {
	case maxUsed >= 90:
		return colorError
	case maxUsed >= 70:
		return colorWarning
	default:
		return colorGood
	}
}

func cacheSummary(t tokenTotals) string {
	if t.CacheCreationTokens == 0 && t.CacheReadTokens == 0 {
		return "n/a"
	}
	return fmt.Sprintf("%s create / %s read", compactInt(t.CacheCreationTokens), compactInt(t.CacheReadTokens))
}

func updatedSuffix(updatedAt string) string {
	if updatedAt == "" {
		return ""
	}
	return " · updated " + formatTime(updatedAt)
}

func usageUpdatedSuffix(report usageReport) string {
	if report.Usage != nil && report.Usage.UpdatedAt != "" {
		return updatedSuffix(report.Usage.UpdatedAt)
	}
	if report.Status != nil && report.Status.UpdatedAt != "" {
		return updatedSuffix(report.Status.UpdatedAt)
	}
	if report.Credits != nil && report.Credits.UpdatedAt != "" {
		return updatedSuffix(report.Credits.UpdatedAt)
	}
	return ""
}

func shortField(title, value string) *model.SlackAttachmentField {
	return &model.SlackAttachmentField{
		Title: title,
		Value: value,
		Short: true,
	}
}

func displayProvider(provider string) string {
	if provider == "" {
		return "Unknown"
	}
	return strings.ToUpper(provider[:1]) + provider[1:]
}

func emptyAs(value, fallback string) string {
	if strings.TrimSpace(value) == "" {
		return fallback
	}
	return value
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

func money(v float64) string {
	return "$" + trimFloat(v)
}

func percent(v float64) string {
	return trimFloat(v) + "%"
}

func trimFloat(v float64) string {
	if math.Abs(v) >= 100 {
		return fmt.Sprintf("%.0f", v)
	}
	if math.Abs(v) >= 10 {
		return strings.TrimRight(strings.TrimRight(fmt.Sprintf("%.1f", v), "0"), ".")
	}
	return strings.TrimRight(strings.TrimRight(fmt.Sprintf("%.2f", v), "0"), ".")
}

func compactInt(v int64) string {
	abs := math.Abs(float64(v))
	switch {
	case abs >= 1_000_000_000:
		return trimFloat(float64(v)/1_000_000_000) + "B"
	case abs >= 1_000_000:
		return trimFloat(float64(v)/1_000_000) + "M"
	case abs >= 1_000:
		return trimFloat(float64(v)/1_000) + "K"
	default:
		return fmt.Sprintf("%d", v)
	}
}

func windowLength(minutes int) string {
	if minutes <= 0 {
		return ""
	}
	if minutes%10080 == 0 {
		weeks := minutes / 10080
		if weeks == 1 {
			return "1w window"
		}
		return fmt.Sprintf("%dw window", weeks)
	}
	if minutes%1440 == 0 {
		days := minutes / 1440
		if days == 1 {
			return "1d window"
		}
		return fmt.Sprintf("%dd window", days)
	}
	if minutes%60 == 0 {
		hours := minutes / 60
		if hours == 1 {
			return "1h window"
		}
		return fmt.Sprintf("%dh window", hours)
	}
	return fmt.Sprintf("%dm window", minutes)
}

func formatTime(value string) string {
	t, err := time.Parse(time.RFC3339, value)
	if err != nil {
		return value
	}
	return t.UTC().Format("2006-01-02 15:04 UTC")
}

func prettyJSON(b []byte) (string, error) {
	var v any
	if err := json.Unmarshal(b, &v); err != nil {
		return "", err
	}
	buf, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return "", err
	}
	return string(buf), nil
}

func codeBlock(text string) string {
	return "```json\n" + text + "\n```"
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}
