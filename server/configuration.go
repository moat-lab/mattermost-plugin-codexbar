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
	defaultCodexbarBin = "codexbar"
)

type configuration struct{}

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

func (p *Plugin) loadPluginConfiguration() (configuration, error) {
	var cfg configuration
	if err := p.API.LoadPluginConfiguration(&cfg); err != nil {
		return configuration{}, fmt.Errorf("LoadPluginConfiguration: %w", err)
	}
	return cfg, nil
}
