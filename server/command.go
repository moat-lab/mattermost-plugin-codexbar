package main

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"

	rexec "github.com/Mouriya-Emma/rexec-go"
	"github.com/mattermost/mattermost/server/public/model"
	"github.com/mattermost/mattermost/server/public/plugin"
)

const (
	costTimeout   = 45 * time.Second
	usageTimeout  = 75 * time.Second
	configTimeout = 15 * time.Second
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
	Label   string
	Argv    []string
	Timeout time.Duration
}

type codexbarOutput struct {
	Label  string
	Result *rexec.Result
	Err    error
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

	req, err := buildCodexbarRequest(args.Command, bin)
	if err != nil {
		return ephemeral(err.Error()), nil
	}

	if req.Mode == modeHelp {
		post := botPost(args.ChannelId, botID, renderHelp()...)
		if err := client.Post.CreatePost(post); err != nil {
			return ephemeral(fmt.Sprintf("create post failed: %v", err)), nil
		}
		return &model.CommandResponse{}, nil
	}

	outputs := make([]codexbarOutput, 0, len(req.Invocations))
	for _, inv := range req.Invocations {
		ctx, cancel := context.WithTimeout(context.Background(), inv.Timeout)
		res, runErr := rc.Run(ctx, inv.Argv, rexec.WithTimeout(inv.Timeout))
		cancel()
		outputs = append(outputs, codexbarOutput{
			Label:  inv.Label,
			Result: res,
			Err:    runErr,
		})
	}

	attachments := renderOutputs(req.Mode, outputs)
	post := botPost(args.ChannelId, botID, attachments...)
	if err := client.Post.CreatePost(post); err != nil {
		return ephemeral(fmt.Sprintf("create post failed: %v", err)), nil
	}

	return &model.CommandResponse{}, nil
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

func buildCodexbarRequest(raw, bin string) (codexbarRequest, error) {
	fields := strings.Fields(strings.TrimSpace(raw))
	if len(fields) == 0 {
		return codexbarRequest{}, errors.New("empty command")
	}

	args := fields[1:]
	if len(args) == 0 {
		args = []string{"summary"}
	}

	switch args[0] {
	case "summary", "s":
		if len(args) > 1 {
			return codexbarRequest{}, fmt.Errorf("summary does not accept extra arguments: %s", strings.Join(args[1:], " "))
		}
		return codexbarRequest{
			Mode: modeSummary,
			Invocations: []codexbarInvocation{
				{
					Label:   "usage",
					Argv:    []string{bin, "usage", "--format", "json", "--status", "--provider", "all"},
					Timeout: usageTimeout,
				},
				{
					Label:   "cost",
					Argv:    []string{bin, "cost", "--format", "json", "--provider", "all"},
					Timeout: costTimeout,
				},
			},
		}, nil
	case "cost", "c":
		argv, err := buildCostArgv(bin, args[1:])
		if err != nil {
			return codexbarRequest{}, err
		}
		return codexbarRequest{
			Mode:        modeCost,
			Invocations: []codexbarInvocation{{Label: "cost", Argv: argv, Timeout: costTimeout}},
		}, nil
	case "usage", "u", "status":
		argv, err := buildUsageArgv(bin, args[1:])
		if err != nil {
			return codexbarRequest{}, err
		}
		return codexbarRequest{
			Mode:        modeUsage,
			Invocations: []codexbarInvocation{{Label: "usage", Argv: argv, Timeout: usageTimeout}},
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
				Timeout: configTimeout,
			}},
		}, nil
	case "help", "h":
		return codexbarRequest{Mode: modeHelp}, nil
	default:
		return codexbarRequest{}, fmt.Errorf("unknown CodexBar command %q; use `/codexbar help`", args[0])
	}
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

func buildUsageArgv(bin string, args []string) ([]string, error) {
	provider := "all"
	source := ""
	for i := 0; i < len(args); i++ {
		arg := args[i]
		switch {
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
		case arg == "--source":
			if i+1 >= len(args) {
				return nil, errors.New("--source requires auto|web|cli|oauth|api")
			}
			i++
			value := args[i]
			if !allowedUsageSources[value] {
				return nil, fmt.Errorf("unsupported usage source %q; use auto|web|cli|oauth|api", value)
			}
			source = value
		case strings.HasPrefix(arg, "--source="):
			value := strings.TrimPrefix(arg, "--source=")
			if !allowedUsageSources[value] {
				return nil, fmt.Errorf("unsupported usage source %q; use auto|web|cli|oauth|api", value)
			}
			source = value
		case safeTokenPattern.MatchString(arg):
			if err := validateProvider(arg); err != nil {
				return nil, err
			}
			provider = arg
		default:
			return nil, fmt.Errorf("unsupported usage argument %q; use provider codex|claude|gemini|all and optional --source=<source>", arg)
		}
	}
	argv := []string{bin, "usage", "--format", "json", "--status", "--provider", provider}
	if source != "" {
		argv = append(argv, "--source", source)
	}
	return argv, nil
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
