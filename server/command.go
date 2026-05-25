package main

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"sync"
	"time"

	rexec "github.com/Mouriya-Emma/rexec-go"
	"github.com/mattermost/mattermost/server/public/model"
	"github.com/mattermost/mattermost/server/public/plugin"
)

const (
	costTimeout   = 45 * time.Second
	usageTimeout  = 75 * time.Second
	configTimeout = 15 * time.Second

	usageWebTimeoutSeconds = "20"
)

var (
	allowedProviders = map[string]bool{
		"all":    true,
		"codex":  true,
		"claude": true,
		"gemini": true,
	}
	allowedUsageSources = map[string]bool{
		"auto":  true,
		"web":   true,
		"cli":   true,
		"oauth": true,
		"api":   true,
	}
	safeTokenPattern = regexp.MustCompile(`^[a-z0-9][a-z0-9-]*$`)
)

type commandMode string

const (
	modeSummary commandMode = "summary"
	modeCost    commandMode = "cost"
	modeUsage   commandMode = "usage"
	modeConfig  commandMode = "config"
	modeHelp    commandMode = "help"
)

type codexbarRequest struct {
	Mode        commandMode
	Invocations []codexbarInvocation
}

type codexbarInvocation struct {
	Label      string
	Argv       []string
	Cwd        string
	Timeout    time.Duration
	UsageHints usageRenderHints
}

type codexbarOutput struct {
	Label      string
	Result     *rexec.Result
	Err        error
	UsageHints usageRenderHints
}

type codexbarRunner interface {
	Run(ctx context.Context, argv []string, opts ...rexec.RunOption) (*rexec.Result, error)
}

type usageRenderHints struct {
	Provider string
	Source   string
}

func buildAutocompleteTree() *model.AutocompleteData {
	root := model.NewAutocompleteData(slashTrigger, "[command]", "Private CodexBar cards.")

	root.AddCommand(model.NewAutocompleteData("summary", "", "Usage limits plus local cost summary."))

	cost := model.NewAutocompleteData("cost", "[provider]", "Local token cost from CodexBar logs.")
	cost.AddStaticListArgument("Provider", false, providerAutocompleteItems())
	cost.AddNamedStaticListArgument("provider", "Provider", false, providerAutocompleteItems())
	root.AddCommand(cost)

	usage := model.NewAutocompleteData("usage", "[provider]", "Live usage limits and provider status.")
	usage.AddStaticListArgument("Provider", false, providerAutocompleteItems())
	usage.AddNamedStaticListArgument("provider", "Provider", false, providerAutocompleteItems())
	usage.AddNamedStaticListArgument("source", "CodexBar source", false, []model.AutocompleteListItem{
		{Item: "auto"}, {Item: "web"}, {Item: "cli"}, {Item: "oauth"}, {Item: "api"},
	})
	root.AddCommand(usage)

	root.AddCommand(model.NewAutocompleteData("config", "", "Validate CodexBar config."))
	root.AddCommand(model.NewAutocompleteData("help", "", "Show the Mattermost command surface."))

	return root
}

func providerAutocompleteItems() []model.AutocompleteListItem {
	return []model.AutocompleteListItem{
		{Item: "all", Hint: "all enabled providers"},
		{Item: "codex"},
		{Item: "claude"},
		{Item: "gemini"},
	}
}

func (p *Plugin) ExecuteCommand(_ *plugin.Context, args *model.CommandArgs) (*model.CommandResponse, *model.AppError) {
	client := p.getClient()
	rc := p.getRexec()
	botID := p.getBotUserID()
	bin := p.getCodexbarBin()
	cwd := p.getCodexbarCwd()
	if client == nil || rc == nil || botID == "" || bin == "" {
		return ephemeral("CodexBar plugin is not fully activated"), nil
	}

	ok, err := p.isCodexbarBotDM(args.ChannelId)
	if err != nil {
		return ephemeral(fmt.Sprintf("CodexBar could not inspect this channel: %v", err)), nil
	}
	if !ok {
		return ephemeral("CodexBar only responds in its bot direct message. Open a DM with CodexBar and run `/codexbar` there."), nil
	}

	req, err := buildCodexbarRequest(args.Command, bin, cwd)
	if err != nil {
		return ephemeral(err.Error()), nil
	}

	p.clearBotMessages(args.ChannelId, botID)

	loading := loadingPost(args.ChannelId, botID)
	_ = client.Post.CreatePost(loading)

	if req.Mode == modeHelp {
		if loading.Id != "" {
			_ = client.Post.DeletePost(loading.Id)
		}
		post := botPost(args.ChannelId, botID, renderHelp()...)
		if err := client.Post.CreatePost(post); err != nil {
			return ephemeral(fmt.Sprintf("create post failed: %v", err)), nil
		}
		return &model.CommandResponse{}, nil
	}

	outputs := runCodexbarInvocations(rc, req.Invocations)

	if loading.Id != "" {
		_ = client.Post.DeletePost(loading.Id)
	}

	attachments := renderOutputsWithOptions(req.Mode, outputs, renderOptions{
		HideAccountValues: p.getHideAccountValues(),
	})
	post := botPost(args.ChannelId, botID, attachments...)
	if err := client.Post.CreatePost(post); err != nil {
		return ephemeral(fmt.Sprintf("create post failed: %v", err)), nil
	}

	return &model.CommandResponse{}, nil
}

func runCodexbarInvocations(runner codexbarRunner, invocations []codexbarInvocation) []codexbarOutput {
	outputs := make([]codexbarOutput, len(invocations))
	var wg sync.WaitGroup
	for i, inv := range invocations {
		i, inv := i, inv
		outputs[i] = codexbarOutput{
			Label:      inv.Label,
			UsageHints: inv.UsageHints,
		}
		wg.Add(1)
		go func() {
			defer wg.Done()
			ctx, cancel := context.WithTimeout(context.Background(), inv.Timeout)
			defer cancel()
			res, runErr := runner.Run(ctx, inv.Argv, rexec.WithTimeout(inv.Timeout), rexec.WithCwd(inv.Cwd))
			outputs[i].Result = res
			outputs[i].Err = runErr
		}()
	}
	wg.Wait()
	return outputs
}

func loadingPost(channelID, botID string) *model.Post {
	post := &model.Post{
		ChannelId: channelID,
		UserId:    botID,
	}
	model.ParseSlackAttachment(post, []*model.SlackAttachment{{
		Text:  "Loading…",
		Color: colorAccent,
	}})
	return post
}

func (p *Plugin) clearBotMessages(channelID, botID string) {
	client := p.getClient()
	if client == nil {
		return
	}
	var toDelete []string
	const perPage = 200
	for page := 0; ; page++ {
		postList, err := client.Post.GetPostsForChannel(channelID, page, perPage)
		if err != nil || postList == nil {
			break
		}
		for _, postID := range postList.Order {
			if post := postList.Posts[postID]; post != nil && post.UserId == botID {
				toDelete = append(toDelete, post.Id)
			}
		}
		if len(postList.Order) < perPage {
			break
		}
	}
	for _, id := range toDelete {
		_ = client.Post.DeletePost(id)
	}
}

func botPost(channelID, botID string, attachments ...*model.SlackAttachment) *model.Post {
	post := &model.Post{
		ChannelId: channelID,
		UserId:    botID,
	}
	model.ParseSlackAttachment(post, attachments)
	return post
}

func (p *Plugin) isCodexbarBotDM(channelID string) (bool, error) {
	client := p.getClient()
	if client == nil {
		return false, errors.New("plugin API client is unavailable")
	}
	channel, err := client.Channel.Get(channelID)
	if err != nil {
		return false, err
	}
	return isCodexbarBotDM(channel, p.getBotUserID()), nil
}

func isCodexbarBotDM(channel *model.Channel, botID string) bool {
	if channel == nil || botID == "" {
		return false
	}
	return model.IsBotDMChannel(channel, botID)
}

func buildCodexbarRequest(raw, bin, cwd string) (codexbarRequest, error) {
	args := commandArgs(raw)
	if len(args) == 0 {
		args = []string{"summary"}
	}

	switch args[0] {
	case "summary", "s":
		if len(args) > 1 {
			return codexbarRequest{}, fmt.Errorf("summary does not accept extra arguments: %s", strings.Join(args[1:], " "))
		}
		invocations := append(summaryUsageInvocations(bin, cwd), codexbarInvocation{
			Label:   "cost",
			Argv:    []string{bin, "cost", "--format", "json", "--provider", "all"},
			Cwd:     cwd,
			Timeout: costTimeout,
		})
		return codexbarRequest{
			Mode:        modeSummary,
			Invocations: invocations,
		}, nil
	case "cost", "c":
		argv, err := buildCostArgv(bin, args[1:])
		if err != nil {
			return codexbarRequest{}, err
		}
		return codexbarRequest{
			Mode:        modeCost,
			Invocations: []codexbarInvocation{{Label: "cost", Argv: argv, Cwd: cwd, Timeout: costTimeout}},
		}, nil
	case "usage", "u", "status":
		invocations, err := buildUsageInvocations(bin, cwd, args[1:])
		if err != nil {
			return codexbarRequest{}, err
		}
		return codexbarRequest{
			Mode:        modeUsage,
			Invocations: invocations,
		}, nil
	case "config", "health":
		if len(args) > 1 {
			return codexbarRequest{}, fmt.Errorf("%s does not accept extra arguments: %s", args[0], strings.Join(args[1:], " "))
		}
		return codexbarRequest{
			Mode: modeConfig,
			Invocations: []codexbarInvocation{{
				Label:   "config",
				Argv:    []string{bin, "config", "validate", "--format", "json"},
				Cwd:     cwd,
				Timeout: configTimeout,
			}},
		}, nil
	case "help", "h":
		return codexbarRequest{Mode: modeHelp}, nil
	default:
		return codexbarRequest{}, fmt.Errorf("unknown CodexBar command %q; use `/codexbar help`", args[0])
	}
}

func commandArgs(raw string) []string {
	fields := strings.Fields(strings.TrimSpace(raw))
	if len(fields) == 0 {
		return nil
	}
	trigger := strings.TrimPrefix(fields[0], "/")
	if trigger == slashTrigger {
		return fields[1:]
	}
	return fields
}

func buildCostArgv(bin string, args []string) ([]string, error) {
	provider := "all"
	refresh := false
	for i := 0; i < len(args); i++ {
		arg := args[i]
		switch {
		case arg == "--refresh":
			refresh = true
		case arg == "--provider":
			if i+1 >= len(args) {
				return nil, errors.New("--provider requires codex|claude|gemini|all")
			}
			i++
			value := args[i]
			if err := validateProvider(value); err != nil {
				return nil, err
			}
			provider = value
		case strings.HasPrefix(arg, "--provider="):
			value := strings.TrimPrefix(arg, "--provider=")
			if err := validateProvider(value); err != nil {
				return nil, err
			}
			provider = value
		case safeTokenPattern.MatchString(arg):
			if err := validateProvider(arg); err != nil {
				return nil, err
			}
			provider = arg
		default:
			return nil, fmt.Errorf("unsupported cost argument %q; use provider codex|claude|gemini|all and optional --refresh", arg)
		}
	}
	argv := []string{bin, "cost", "--format", "json", "--provider", provider}
	if refresh {
		argv = append(argv, "--refresh")
	}
	return argv, nil
}

type usageOptions struct {
	provider string
	source   string
}

func buildUsageInvocations(bin, cwd string, args []string) ([]codexbarInvocation, error) {
	opts, err := parseUsageOptions(args)
	if err != nil {
		return nil, err
	}
	if opts.provider == "all" && opts.source == "" {
		return allUsageInvocations(bin, cwd), nil
	}
	return []codexbarInvocation{{
		Label:      "usage",
		Argv:       buildUsageArgv(bin, opts.provider, opts.source),
		Cwd:        cwd,
		Timeout:    usageTimeout,
		UsageHints: usageRenderHints{Provider: opts.provider, Source: opts.source},
	}}, nil
}

func summaryUsageInvocations(bin, cwd string) []codexbarInvocation {
	return usageInvocationsFor(bin, cwd, []usageProviderSource{
		{provider: "codex", source: "web"},
		{provider: "claude", source: "web"},
	})
}

func allUsageInvocations(bin, cwd string) []codexbarInvocation {
	return usageInvocationsFor(bin, cwd, []usageProviderSource{
		{provider: "codex", source: "web"},
		{provider: "claude", source: "web"},
		{provider: "gemini", source: "api"},
	})
}

type usageProviderSource struct {
	provider string
	source   string
}

func usageInvocationsFor(bin, cwd string, specs []usageProviderSource) []codexbarInvocation {
	invocations := make([]codexbarInvocation, 0, len(specs))
	for _, spec := range specs {
		invocations = append(invocations, codexbarInvocation{
			Label:      "usage",
			Argv:       buildUsageArgv(bin, spec.provider, spec.source),
			Cwd:        cwd,
			Timeout:    usageTimeout,
			UsageHints: usageRenderHints{Provider: spec.provider, Source: spec.source},
		})
	}
	return invocations
}

func parseUsageOptions(args []string) (usageOptions, error) {
	provider := "all"
	source := ""
	for i := 0; i < len(args); i++ {
		arg := args[i]
		switch {
		case arg == "--provider":
			if i+1 >= len(args) {
				return usageOptions{}, errors.New("--provider requires codex|claude|gemini|all")
			}
			i++
			value := args[i]
			if err := validateProvider(value); err != nil {
				return usageOptions{}, err
			}
			provider = value
		case strings.HasPrefix(arg, "--provider="):
			value := strings.TrimPrefix(arg, "--provider=")
			if err := validateProvider(value); err != nil {
				return usageOptions{}, err
			}
			provider = value
		case arg == "--source":
			if i+1 >= len(args) {
				return usageOptions{}, errors.New("--source requires auto|web|cli|oauth|api")
			}
			i++
			value := args[i]
			if !allowedUsageSources[value] {
				return usageOptions{}, fmt.Errorf("unsupported usage source %q; use auto|web|cli|oauth|api", value)
			}
			source = value
		case strings.HasPrefix(arg, "--source="):
			value := strings.TrimPrefix(arg, "--source=")
			if !allowedUsageSources[value] {
				return usageOptions{}, fmt.Errorf("unsupported usage source %q; use auto|web|cli|oauth|api", value)
			}
			source = value
		case safeTokenPattern.MatchString(arg):
			if err := validateProvider(arg); err != nil {
				return usageOptions{}, err
			}
			provider = arg
		default:
			return usageOptions{}, fmt.Errorf("unsupported usage argument %q; use provider codex|claude|gemini|all and optional --source=<source>", arg)
		}
	}
	return usageOptions{provider: provider, source: source}, nil
}

func buildUsageArgv(bin, provider, source string) []string {
	argv := []string{bin, "usage", "--format", "json", "--status", "--provider", provider}
	if source != "" {
		argv = append(argv, "--source", source)
	}
	if source == "web" {
		argv = append(argv, "--web-timeout", usageWebTimeoutSeconds)
	}
	return argv
}

func validateProvider(value string) error {
	if !allowedProviders[value] {
		return fmt.Errorf("unsupported provider %q; use codex|claude|gemini|all", value)
	}
	return nil
}

func ephemeral(text string) *model.CommandResponse {
	return &model.CommandResponse{
		ResponseType: model.CommandResponseTypeEphemeral,
		Text:         text,
	}
}
