package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

const githubAPIBase = "https://api.github.com"

func githubRequest(token, method, path string, body any) ([]byte, error) {
	var reader io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return nil, err
		}
		reader = bytes.NewReader(b)
	}
	req, err := http.NewRequest(method, githubAPIBase+path, reader)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode >= 300 {
		return nil, fmt.Errorf("github api %s %s: status %d: %s", method, path, resp.StatusCode, string(data))
	}
	return data, nil
}

// listPRComments returns review comments + issue (conversation) comments for a PR,
// i.e. "team comments" on the PR.
func listPRComments(token, owner, repo string, prNumber int) (string, error) {
	reviewData, err := githubRequest(token, http.MethodGet,
		fmt.Sprintf("/repos/%s/%s/pulls/%d/comments", owner, repo, prNumber), nil)
	if err != nil {
		return "", err
	}
	issueData, err := githubRequest(token, http.MethodGet,
		fmt.Sprintf("/repos/%s/%s/issues/%d/comments", owner, repo, prNumber), nil)
	if err != nil {
		return "", err
	}
	combined := map[string]json.RawMessage{
		"reviewComments": reviewData,
		"issueComments":  issueData,
	}
	out, err := json.Marshal(combined)
	if err != nil {
		return "", err
	}
	return string(out), nil
}

func addPRComment(token, owner, repo string, prNumber int, body string) (string, error) {
	data, err := githubRequest(token, http.MethodPost,
		fmt.Sprintf("/repos/%s/%s/issues/%d/comments", owner, repo, prNumber),
		map[string]string{"body": body})
	if err != nil {
		return "", err
	}
	return string(data), nil
}

func createPullRequest(token, owner, repo, head, base, title, body string) (string, error) {
	data, err := githubRequest(token, http.MethodPost,
		fmt.Sprintf("/repos/%s/%s/pulls", owner, repo),
		map[string]string{"head": head, "base": base, "title": title, "body": body})
	if err != nil {
		return "", err
	}
	return string(data), nil
}
