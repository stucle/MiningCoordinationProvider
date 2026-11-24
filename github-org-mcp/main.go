package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
)

func toolDefs() []toolDef {
	str := map[string]any{"type": "string"}
	strArr := map[string]any{"type": "array", "items": map[string]any{"type": "string"}}
	boolT := map[string]any{"type": "boolean"}
	num := map[string]any{"type": "integer"}

	withOrgan := func(props map[string]any, required []string) map[string]any {
		props["organName"] = str
		return map[string]any{
			"type":       "object",
			"properties": props,
			"required":   append([]string{"organName"}, required...),
		}
	}

	return []toolDef{
		{Name: "git_clone", Description: "Clone a git repository into dest", InputSchema: withOrgan(map[string]any{"repoUrl": str, "dest": str}, []string{"repoUrl", "dest"})},
		{Name: "git_fetch", Description: "Fetch from a remote in workdir", InputSchema: withOrgan(map[string]any{"workdir": str, "remote": str}, []string{"workdir"})},
		{Name: "git_commit", Description: "Stage paths and commit in workdir", InputSchema: withOrgan(map[string]any{"workdir": str, "message": str, "paths": strArr}, []string{"workdir", "message"})},
		{Name: "git_push", Description: "Push a branch to a remote from workdir", InputSchema: withOrgan(map[string]any{"workdir": str, "remote": str, "branch": str, "force": boolT}, []string{"workdir", "remote", "branch"})},
		{Name: "git_rebase", Description: "Rebase current branch onto ref in workdir", InputSchema: withOrgan(map[string]any{"workdir": str, "onto": str}, []string{"workdir", "onto"})},
		{Name: "git_tag", Description: "Create a tag in workdir", InputSchema: withOrgan(map[string]any{"workdir": str, "name": str, "message": str}, []string{"workdir", "name"})},
		{Name: "list_pr_comments", Description: "List review + issue (team) comments on a pull request", InputSchema: withOrgan(map[string]any{"owner": str, "repo": str, "prNumber": num}, []string{"owner", "repo", "prNumber"})},
		{Name: "add_pr_comment", Description: "Add a comment to a pull request", InputSchema: withOrgan(map[string]any{"owner": str, "repo": str, "prNumber": num, "body": str}, []string{"owner", "repo", "prNumber", "body"})},
		{Name: "create_pull_request", Description: "Create a pull request", InputSchema: withOrgan(map[string]any{"owner": str, "repo": str, "head": str, "base": str, "title": str, "body": str}, []string{"owner", "repo", "head", "base", "title"})},
	}
}

func main() {
	organFlag := flag.String("org", "", "organName this instance is bound to")
	tokenFlag := flag.String("token", "", "access token param (highest priority)")
	flag.Parse()

	organName := *organFlag
	if organName == "" {
		organName = os.Getenv("ORG_NAME")
	}
	if organName == "" {
		fatalStartupError("start failed: organName is required (--org or ORG_NAME)")
	}

	paramToken := *tokenFlag
	if paramToken == "" {
		paramToken = os.Getenv("ACCESS_TOKEN")
	}

	token, err := ResolveToken(organName, paramToken)
	if err != nil {
		fatalStartupError("%v", err)
	}

	cfgs, err := loadEnvJSON()
	if err != nil {
		fatalStartupError("%v", err)
	}
	safe := SafeCommandsFor(cfgs, organName)

	s := &server{organName: organName, token: token, safe: safe}
	s.tools = map[string]func(args json.RawMessage) toolCallResult{
		"git_clone": func(args json.RawMessage) toolCallResult {
			var p struct{ RepoURL, Dest string }
			if err := json.Unmarshal(args, &p); err != nil {
				return errorResult("bad args: %v", err)
			}
			out, alerted, err := gitClone(organName, safe, p.RepoURL, p.Dest)
			if err != nil {
				return errorResult("%v", err)
			}
			return annotatedResult(out, alerted, "clone")
		},
		"git_fetch": func(args json.RawMessage) toolCallResult {
			var p struct{ Workdir, Remote string }
			if err := json.Unmarshal(args, &p); err != nil {
				return errorResult("bad args: %v", err)
			}
			out, alerted, err := gitFetch(organName, safe, p.Workdir, p.Remote)
			if err != nil {
				return errorResult("%v", err)
			}
			return annotatedResult(out, alerted, "fetch")
		},
		"git_commit": func(args json.RawMessage) toolCallResult {
			var p struct {
				Workdir, Message string
				Paths             []string
			}
			if err := json.Unmarshal(args, &p); err != nil {
				return errorResult("bad args: %v", err)
			}
			out, alerted, err := gitCommit(organName, safe, p.Workdir, p.Message, p.Paths)
			if err != nil {
				return errorResult("%v", err)
			}
			return annotatedResult(out, alerted, "commit")
		},
		"git_push": func(args json.RawMessage) toolCallResult {
			var p struct {
				Workdir, Remote, Branch string
				Force                   bool
			}
			if err := json.Unmarshal(args, &p); err != nil {
				return errorResult("bad args: %v", err)
			}
			out, alerted, err := gitPush(organName, safe, p.Workdir, p.Remote, p.Branch, p.Force)
			if err != nil {
				return errorResult("%v", err)
			}
			return annotatedResult(out, alerted, "push")
		},
		"git_rebase": func(args json.RawMessage) toolCallResult {
			var p struct{ Workdir, Onto string }
			if err := json.Unmarshal(args, &p); err != nil {
				return errorResult("bad args: %v", err)
			}
			out, alerted, err := gitRebase(organName, safe, p.Workdir, p.Onto)
			if err != nil {
				return errorResult("%v", err)
			}
			return annotatedResult(out, alerted, "rebase")
		},
		"git_tag": func(args json.RawMessage) toolCallResult {
			var p struct{ Workdir, Name, Message string }
			if err := json.Unmarshal(args, &p); err != nil {
				return errorResult("bad args: %v", err)
			}
			out, alerted, err := gitTag(organName, safe, p.Workdir, p.Name, p.Message)
			if err != nil {
				return errorResult("%v", err)
			}
			return annotatedResult(out, alerted, "tag")
		},
		"list_pr_comments": func(args json.RawMessage) toolCallResult {
			var p struct {
				Owner, Repo string
				PrNumber    int
			}
			if err := json.Unmarshal(args, &p); err != nil {
				return errorResult("bad args: %v", err)
			}
			out, err := listPRComments(token, p.Owner, p.Repo, p.PrNumber)
			if err != nil {
				return errorResult("%v", err)
			}
			return textResult(out)
		},
		"add_pr_comment": func(args json.RawMessage) toolCallResult {
			var p struct {
				Owner, Repo, Body string
				PrNumber          int
			}
			if err := json.Unmarshal(args, &p); err != nil {
				return errorResult("bad args: %v", err)
			}
			out, err := addPRComment(token, p.Owner, p.Repo, p.PrNumber, p.Body)
			if err != nil {
				return errorResult("%v", err)
			}
			return textResult(out)
		},
		"create_pull_request": func(args json.RawMessage) toolCallResult {
			var p struct{ Owner, Repo, Head, Base, Title, Body string }
			if err := json.Unmarshal(args, &p); err != nil {
				return errorResult("bad args: %v", err)
			}
			out, err := createPullRequest(token, p.Owner, p.Repo, p.Head, p.Base, p.Title, p.Body)
			if err != nil {
				return errorResult("%v", err)
			}
			return textResult(out)
		},
	}

	fmt.Fprintf(os.Stderr, "github-org-mcp started, bound to organName=%q\n", organName)
	if err := s.run(os.Stdin, os.Stdout); err != nil {
		fatalStartupError("%v", err)
	}
}
