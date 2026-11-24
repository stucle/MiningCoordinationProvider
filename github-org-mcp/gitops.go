package main

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"os/exec"
)

// needsNetworkAuth is the set of subcommands that talk to a remote and thus
// benefit from the organ's token; local-only subcommands (add, commit, rebase,
// tag) never get it.
var needsNetworkAuth = map[string]bool{"clone": true, "fetch": true, "push": true}

// authHeaderArgs builds the `-c http.extraHeader=...` global git flag that
// injects the organ's token as a Basic auth header for this invocation only.
// This keeps the token entirely inside the Go process: it never has to be
// embedded in a repoUrl/remote argument coming from the MCP caller, and it is
// never passed to guardCheck/alert logging (see runGit).
func authHeaderArgs(subcommand, token string) []string {
	if token == "" || !needsNetworkAuth[subcommand] {
		return nil
	}
	cred := base64.StdEncoding.EncodeToString([]byte("x-access-token:" + token))
	return []string{"-c", "http.extraHeader=Authorization: Basic " + cred}
}

// runGit executes `git <subcommand> <args...>` in workdir, guarding against the organ's safeCommands allowlist first (alert-only, never blocks).
// token (if non-empty and subcommand needs network auth) is injected as a global -c flag ahead of the subcommand — it is never part of args, so it never reaches guardCheck's alert logging or the tool_response text.
// The bool return reports whether an alert fired, so callers can also surface the MCP_ALERT marker in the tool_response text itself.
func runGit(organName string, safe []string, workdir, subcommand string, args []string, token string) (string, bool, error) {
	alerted := guardCheck(organName, safe, subcommand, args)

	full := append(authHeaderArgs(subcommand, token), append([]string{subcommand}, args...)...)
	cmd := exec.Command("git", full...)
	if workdir != "" {
		cmd.Dir = workdir
	}
	var out, stderr bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return "", alerted, fmt.Errorf("git %s failed: %v: %s", subcommand, err, stderr.String())
	}
	return out.String(), alerted, nil
}

func gitClone(organName string, safe []string, token, repoURL, dest string) (string, bool, error) {
	return runGit(organName, safe, "", "clone", []string{repoURL, dest}, token)
}

func gitFetch(organName string, safe []string, token, workdir, remote string) (string, bool, error) {
	args := []string{}
	if remote != "" {
		args = append(args, remote)
	}
	return runGit(organName, safe, workdir, "fetch", args, token)
}

func gitCommit(organName string, safe []string, workdir, message string, paths []string) (string, bool, error) {
	alerted := false
	if len(paths) > 0 {
		_, a, err := runGit(organName, safe, workdir, "add", paths, "")
		alerted = alerted || a
		if err != nil {
			return "", alerted, err
		}
	}
	out, a, err := runGit(organName, safe, workdir, "commit", []string{"-m", message}, "")
	return out, alerted || a, err
}

func gitPush(organName string, safe []string, token, workdir, remote, branch string, force bool) (string, bool, error) {
	args := []string{remote, branch}
	if force {
		args = append([]string{"--force"}, args...)
	}
	return runGit(organName, safe, workdir, "push", args, token)
}

func gitRebase(organName string, safe []string, workdir, onto string) (string, bool, error) {
	return runGit(organName, safe, workdir, "rebase", []string{onto}, "")
}

func gitTag(organName string, safe []string, workdir, name, message string) (string, bool, error) {
	args := []string{name}
	if message != "" {
		args = []string{"-a", name, "-m", message}
	}
	return runGit(organName, safe, workdir, "tag", args, "")
}
