package main

import (
	"encoding/json"
	"fmt"
	"os"
	"regexp"
	"time"
)

// credentialInURL matches the userinfo part of a URL, e.g. https://TOKEN@host/...
var credentialInURL = regexp.MustCompile(`://[^/@\s]+@`)

// redactArgs scrubs embedded basic-auth/token credentials out of URL-shaped  args before they are ever written to stderr, the footprint log,
// or tool_response — those sinks are not secret stores.
func redactArgs(args []string) []string {
	out := make([]string, len(args))
	for i, a := range args {
		out[i] = credentialInURL.ReplaceAllString(a, "://***@")
	}
	return out
}

// alertLogPath is where MCP_ALERT lines are additionally mirrored as structured JSON, independent of stderr, so a hook footprint script has a stable file to tail.
func alertLogPath() string {
	if p := os.Getenv("ALERT_LOG_PATH"); p != "" {
		return p
	}
	return "/Volumes/RAMDisk/log/Go/alerts-footprint.jsonl"
}

type alertRecord struct {
	Timestamp string   `json:"ts"`
	OrganName string   `json:"organName"`
	Command   string   `json:"command"`
	Args      []string `json:"args"`
	PID       int      `json:"pid"`
}

func isSafeCommand(safe []string, subcommand string) bool {
	for _, s := range safe {
		if s == subcommand {
			return true
		}
	}
	return false
}

// guardCheck logs an alert (stderr marker + footprint file) when subcommand is outside the organ's safeCommands allowlist.
// It never blocks execution: the alert is observability for the user's own monitoring purpose, not an ACL.
// Returns true when an alert fired, so callers can also surface the marker in the tool_response text — stderr alone is invisible to a Claude Code hook.
func guardCheck(organName string, safe []string, subcommand string, args []string) bool {
	if isSafeCommand(safe, subcommand) {
		return false
	}
	rec := alertRecord{
		Timestamp: time.Now().UTC().Format(time.RFC3339),
		OrganName: organName,
		Command:   subcommand,
		Args:      redactArgs(args),
		PID:       os.Getpid(),
	}
	line, err := json.Marshal(rec)
	if err != nil {
		fmt.Fprintf(os.Stderr, "MCP_ALERT: marshal error for command %q: %v\n", subcommand, err)
		return true
	}
	fmt.Fprintf(os.Stderr, "MCP_ALERT: %s\n", line)

	f, err := os.OpenFile(alertLogPath(), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err == nil {
		defer f.Close()
		f.Write(line)
		f.Write([]byte("\n"))
	} else {
		fmt.Fprintf(os.Stderr, "MCP_ALERT: could not open footprint log: %v\n", err)
	}
	return true
}
