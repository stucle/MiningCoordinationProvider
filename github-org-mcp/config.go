package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// OrgConfig mirrors one element of env.json.
type OrgConfig struct {
	OrganName    string   `json:"organName"`
	AccessToken  string   `json:"accessToken"`
	SafeCommands []string `json:"safeCommands"`
}

// envJSONPath resolves the config source of truth. It must live under the Claude config folder,
// whose exact location varies per machine (most unix systems default to ~/.claude), but is fixed once resolved — no runtime override, otherwise "fixed" would be a lie.
func envJSONPath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		home = "/Volumes/RAMDisk/claude"
	}
	return filepath.Join(home, ".claude", "github-org-mcp", "env.json")
}

func loadEnvJSON() ([]OrgConfig, error) {
	path := envJSONPath()
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read env.json: %w", err)
	}
	var cfgs []OrgConfig
	if err := json.Unmarshal(data, &cfgs); err != nil {
		return nil, fmt.Errorf("parse env.json: %w", err)
	}
	return cfgs, nil
}

func saveEnvJSON(cfgs []OrgConfig) error {
	data, err := json.MarshalIndent(cfgs, "", "    ")
	if err != nil {
		return err
	}
	return os.WriteFile(envJSONPath(), data, 0600)
}

func findOrgConfig(cfgs []OrgConfig, organName string) (*OrgConfig, int) {
	for i := range cfgs {
		if cfgs[i].OrganName == organName {
			return &cfgs[i], i
		}
	}
	return nil, -1
}

// ResolveToken implements the priority rule:
//  1. paramToken (passed in via ACCESS_TOKEN env/flag at process start) wins if non-empty, and is upserted back into env.json so the file stays in sync with the latest param value.
//  2. otherwise fall back to the accessToken already stored in env.json for organName.
//  3. if both are empty, start fails.
func ResolveToken(organName, paramToken string) (string, error) {
	cfgs, err := loadEnvJSON()
	if err != nil {
		return "", err
	}
	cfg, idx := findOrgConfig(cfgs, organName)
	if cfg == nil {
		return "", fmt.Errorf("start failed: organName %q not present in env.json", organName)
	}

	if paramToken != "" {
		cfgs[idx].AccessToken = paramToken
		if err := saveEnvJSON(cfgs); err != nil {
			return "", fmt.Errorf("start failed: could not sync param token into env.json: %w", err)
		}
		return paramToken, nil
	}

	if cfg.AccessToken != "" {
		return cfg.AccessToken, nil
	}

	return "", fmt.Errorf("start failed: no access token for org %q (param empty and env.json empty)", organName)
}

// ReloadToken implements the reload_config semantics:
//  1. newToken non-empty → overwrite accessToken for organName in env.json, return it.
//  2. newToken empty → reuse the accessToken already stored in env.json, unchanged.
//
// Does not restart or exec anything — the caller (tool handler) decides when the process exits after this returns.
func ReloadToken(organName, newToken string) (string, error) {
	cfgs, err := loadEnvJSON()
	if err != nil {
		return "", err
	}
	cfg, idx := findOrgConfig(cfgs, organName)
	if cfg == nil {
		return "", fmt.Errorf("reload failed: organName %q not present in env.json", organName)
	}

	if newToken == "" {
		return cfg.AccessToken, nil
	}

	cfgs[idx].AccessToken = newToken
	if err := saveEnvJSON(cfgs); err != nil {
		return "", fmt.Errorf("reload failed: could not write new token into env.json: %w", err)
	}
	return newToken, nil
}

// SafeCommandsFor returns the allowlisted git subcommands for organName.
func SafeCommandsFor(cfgs []OrgConfig, organName string) []string {
	cfg, _ := findOrgConfig(cfgs, organName)
	if cfg == nil {
		return nil
	}
	return cfg.SafeCommands
}
