package main

import (
	"encoding/json"
	"fmt"
	"os"
)

// OrgConfig mirrors one element of env.json.
type OrgConfig struct {
	OrganName    string   `json:"organName"`
	AccessToken  string   `json:"accessToken"`
	SafeCommands []string `json:"safeCommands"`
}

func envJSONPath() string {
	if p := os.Getenv("ENV_JSON_PATH"); p != "" {
		return p
	}
	return "/Volumes/RAMDisk/space/Go/env.json"
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
//  1. paramToken (passed in via ACCESS_TOKEN env/flag at process start) wins if non-empty,
//     and is upserted back into env.json so the file stays in sync with the latest param value.
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

// SafeCommandsFor returns the allowlisted git subcommands for organName.
func SafeCommandsFor(cfgs []OrgConfig, organName string) []string {
	cfg, _ := findOrgConfig(cfgs, organName)
	if cfg == nil {
		return nil
	}
	return cfg.SafeCommands
}
