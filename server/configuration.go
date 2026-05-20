package main

import (
	"errors"
	"fmt"
	"os"
	"strings"
)

const (
	rexecdAddrEnv      = "CODEXBAR_REXECD_ADDR"
	codexbarBinEnv     = "CODEXBAR_BIN"
	codexbarCwdEnv     = "CODEXBAR_CWD"
	defaultCodexbarBin = "codexbar"
)

// configuration mirrors plugin.json's settings_schema. Field names must match
// setting keys so Mattermost LoadPluginConfiguration binds them.
type configuration struct {
	HideAccountValues bool `json:"HideAccountValues"`
}

func resolveRexecdAddr() (string, error) {
	raw := strings.TrimSpace(os.Getenv(rexecdAddrEnv))
	if raw == "" {
		return "", errors.New(rexecdAddrEnv + " is unset; set it on the Mattermost server process")
	}
	return raw, nil
}

func resolveCodexbarBin() string {
	raw := strings.TrimSpace(os.Getenv(codexbarBinEnv))
	if raw == "" {
		return defaultCodexbarBin
	}
	return raw
}

func resolveCodexbarCwd() string {
	return strings.TrimSpace(os.Getenv(codexbarCwdEnv))
}

func (p *Plugin) loadPluginConfiguration() (configuration, error) {
	var cfg configuration
	if err := p.API.LoadPluginConfiguration(&cfg); err != nil {
		return configuration{}, fmt.Errorf("LoadPluginConfiguration: %w", err)
	}
	return cfg, nil
}
