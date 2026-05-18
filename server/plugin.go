package main

import (
	"fmt"
	"sync"

	rexec "github.com/Mouriya-Emma/rexec-go"
	"github.com/mattermost/mattermost/server/public/model"
	"github.com/mattermost/mattermost/server/public/plugin"
	"github.com/mattermost/mattermost/server/public/pluginapi"
)

const (
	manifestID = "codexbar"

	botUsername    = "codexbar"
	botDisplayName = "CodexBar"
	botDescription = "Private CodexBar usage and cost cards"

	slashTrigger = "codexbar"
)

type Plugin struct {
	plugin.MattermostPlugin

	mu          sync.RWMutex
	client      *pluginapi.Client
	rexec       *rexec.Client
	rexecAddr   string
	codexbarBin string
	botUserID   string
	config      configuration
}

func (p *Plugin) OnActivate() error {
	client := pluginapi.NewClient(p.API, p.Driver)

	botID, err := client.Bot.EnsureBot(&model.Bot{
		Username:    botUsername,
		DisplayName: botDisplayName,
		Description: botDescription,
	})
	if err != nil {
		return fmt.Errorf("ensure bot: %w", err)
	}

	addr, err := resolveRexecdAddr()
	if err != nil {
		return fmt.Errorf("resolve rexecd addr: %w", err)
	}
	rc, err := rexec.New(addr)
	if err != nil {
		return fmt.Errorf("rexec client for %q: %w", addr, err)
	}

	cmd := &model.Command{
		Trigger:          slashTrigger,
		AutoComplete:     true,
		AutoCompleteDesc: "Show private CodexBar usage, limit, cost, and config cards.",
		AutoCompleteHint: "[summary|cost|usage|config|help]",
		DisplayName:      botDisplayName,
		Description:      botDescription,
		AutocompleteData: buildAutocompleteTree(),
	}
	if err := client.SlashCommand.Register(cmd); err != nil {
		_ = rc.Close()
		return fmt.Errorf("register slash /%s: %w", slashTrigger, err)
	}

	p.mu.Lock()
	p.client = client
	p.rexec = rc
	p.rexecAddr = addr
	p.codexbarBin = resolveCodexbarBin()
	p.botUserID = botID
	p.mu.Unlock()

	if err := p.OnConfigurationChange(); err != nil {
		return fmt.Errorf("load configuration: %w", err)
	}

	client.Log.Info("codexbar plugin activated",
		"bot_user_id", botID,
		"rexecd_addr", addr,
		"codexbar_bin", p.getCodexbarBin(),
	)
	return nil
}

func (p *Plugin) OnConfigurationChange() error {
	cfg, err := p.loadPluginConfiguration()
	if err != nil {
		return err
	}
	p.mu.Lock()
	p.config = cfg
	p.mu.Unlock()
	return nil
}

func (p *Plugin) OnDeactivate() error {
	p.mu.Lock()
	rc := p.rexec
	p.rexec = nil
	p.mu.Unlock()
	if rc != nil {
		return rc.Close()
	}
	return nil
}

func (p *Plugin) getClient() *pluginapi.Client {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.client
}

func (p *Plugin) getRexec() *rexec.Client {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.rexec
}

func (p *Plugin) getBotUserID() string {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.botUserID
}

func (p *Plugin) getCodexbarBin() string {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.codexbarBin
}
