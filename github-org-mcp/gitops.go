package main

import (
	"bytes"
	"fmt"
	"os/exec"
)

// runGit executes `git <subcommand> <args...>` in workdir, guarding against
// the organ's safeCommands allowlist first (alert-only, never blocks).
// The bool return reports whether an alert fired, so callers can also surface
// the MCP_ALERT marker in the tool_response text itself.
func runGit(organName string, safe []string, workdir, subcommand string, args []string) (string, bool, error) {
	alerted := guardCheck(organName, safe, subcommand, args)

	full := append([]string{subcommand}, args...)
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

func gitClone(organName string, safe []string, repoURL, dest string) (string, bool, error) {
	return runGit(organName, safe, "", "clone", []string{repoURL, dest})
}

func gitFetch(organName string, safe []string, workdir, remote string) (string, bool, error) {
	args := []string{}
	if remote != "" {
		args = append(args, remote)
	}
	return runGit(organName, safe, workdir, "fetch", args)
}

func gitCommit(organName string, safe []string, workdir, message string, paths []string) (string, bool, error) {
	alerted := false
	if len(paths) > 0 {
		_, a, err := runGit(organName, safe, workdir, "add", paths)
		alerted = alerted || a
		if err != nil {
			return "", alerted, err
		}
	}
	out, a, err := runGit(organName, safe, workdir, "commit", []string{"-m", message})
	return out, alerted || a, err
}

func gitPush(organName string, safe []string, workdir, remote, branch string, force bool) (string, bool, error) {
	args := []string{remote, branch}
	if force {
		args = append([]string{"--force"}, args...)
	}
	return runGit(organName, safe, workdir, "push", args)
}

func gitRebase(organName string, safe []string, workdir, onto string) (string, bool, error) {
	return runGit(organName, safe, workdir, "rebase", []string{onto})
}

func gitTag(organName string, safe []string, workdir, name, message string) (string, bool, error) {
	args := []string{name}
	if message != "" {
		args = []string{"-a", name, "-m", message}
	}
	return runGit(organName, safe, workdir, "tag", args)
}
