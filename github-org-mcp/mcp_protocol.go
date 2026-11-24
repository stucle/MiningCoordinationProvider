package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os"
)

type rpcRequest struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id,omitempty"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

type rpcResponse struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id,omitempty"`
	Result  any             `json:"result,omitempty"`
	Error   *rpcError       `json:"error,omitempty"`
}

type rpcError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

type toolDef struct {
	Name        string         `json:"name"`
	Description string         `json:"description"`
	InputSchema map[string]any `json:"inputSchema"`
}

type toolCallParams struct {
	Name      string          `json:"name"`
	Arguments json.RawMessage `json:"arguments"`
}

type toolContent struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

type toolCallResult struct {
	Content []toolContent `json:"content"`
	IsError bool          `json:"isError,omitempty"`
}

// server binds this process to exactly one organName, resolved at start.
type server struct {
	organName         string
	token             string
	safe              []string
	tools             map[string]func(args json.RawMessage) toolCallResult
	exitAfterResponse bool // set by reload_config: exit cleanly once the response is flushed
}

func textResult(s string) toolCallResult {
	return toolCallResult{Content: []toolContent{{Type: "text", Text: s}}}
}

func errorResult(format string, a ...any) toolCallResult {
	return toolCallResult{Content: []toolContent{{Type: "text", Text: fmt.Sprintf(format, a...)}}, IsError: true}
}

// annotatedResult surfaces the MCP_ALERT marker inside tool_response text (not
// just stderr) so a Claude Code PostToolUse hook can detect it and mirror it
// into the hooks footprint, without this ever turning into IsError.
func annotatedResult(out string, alerted bool, subcommand string) toolCallResult {
	if !alerted {
		return textResult(out)
	}
	return textResult(fmt.Sprintf("MCP_ALERT: command %q is outside safeCommands allowlist\n%s", subcommand, out))
}

func (s *server) run(in io.Reader, out io.Writer) error {
	scanner := bufio.NewScanner(in)
	scanner.Buffer(make([]byte, 0, 64*1024), 16*1024*1024)
	writer := bufio.NewWriter(out)
	defer writer.Flush()

	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}
		var req rpcRequest
		if err := json.Unmarshal(line, &req); err != nil {
			continue
		}
		resp := s.handle(req)
		if resp == nil {
			continue // notification, no response
		}
		data, err := json.Marshal(resp)
		if err != nil {
			continue
		}
		writer.Write(data)
		writer.WriteByte('\n')
		writer.Flush()
		if s.exitAfterResponse {
			// reload_config already persisted the new config; the actual
			// "restart" is Claude Code respawning this process (per its
			// mcpServers entry) the next time a tool on it is called.
			os.Exit(0)
		}
	}
	return scanner.Err()
}

func (s *server) handle(req rpcRequest) *rpcResponse {
	switch req.Method {
	case "initialize":
		return &rpcResponse{JSONRPC: "2.0", ID: req.ID, Result: map[string]any{
			"protocolVersion": "2024-11-05",
			"serverInfo":      map[string]string{"name": "github-org-mcp", "version": "0.1.0"},
			"capabilities":    map[string]any{"tools": map[string]any{}},
		}}
	case "notifications/initialized", "ping":
		if req.Method == "ping" {
			return &rpcResponse{JSONRPC: "2.0", ID: req.ID, Result: map[string]any{}}
		}
		return nil
	case "tools/list":
		return &rpcResponse{JSONRPC: "2.0", ID: req.ID, Result: map[string]any{"tools": toolDefs()}}
	case "tools/call":
		return s.handleToolCall(req)
	default:
		return &rpcResponse{JSONRPC: "2.0", ID: req.ID, Error: &rpcError{Code: -32601, Message: "method not found: " + req.Method}}
	}
}

func (s *server) handleToolCall(req rpcRequest) *rpcResponse {
	var params toolCallParams
	if err := json.Unmarshal(req.Params, &params); err != nil {
		return &rpcResponse{JSONRPC: "2.0", ID: req.ID, Error: &rpcError{Code: -32602, Message: "invalid params"}}
	}

	organName, err := extractOrganName(params.Arguments)
	if err != nil {
		return &rpcResponse{JSONRPC: "2.0", ID: req.ID, Result: errorResult("organName missing or invalid: %v", err)}
	}
	// organName must match this bound instance EXACTLY (plain string compare, no pattern).
	if organName != s.organName {
		return &rpcResponse{JSONRPC: "2.0", ID: req.ID, Result: errorResult(
			"organName mismatch: this instance is bound to %q, got %q — rejected", s.organName, organName)}
	}

	fn, ok := s.tools[params.Name]
	if !ok {
		return &rpcResponse{JSONRPC: "2.0", ID: req.ID, Result: errorResult("unknown tool: %s", params.Name)}
	}
	result := fn(params.Arguments)
	return &rpcResponse{JSONRPC: "2.0", ID: req.ID, Result: result}
}

func extractOrganName(args json.RawMessage) (string, error) {
	var v struct {
		OrganName string `json:"organName"`
	}
	if err := json.Unmarshal(args, &v); err != nil {
		return "", err
	}
	if v.OrganName == "" {
		return "", fmt.Errorf("organName is required")
	}
	return v.OrganName, nil
}

func fatalStartupError(format string, a ...any) {
	fmt.Fprintf(os.Stderr, "FATAL: "+format+"\n", a...)
	os.Exit(1)
}
