package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/mark3labs/mcp-go/client"
	"github.com/mark3labs/mcp-go/mcp"
)

// qdrantCollectionNameRe restricts collection names to alphanumeric and underscore (Qdrant-safe).
var qdrantCollectionNameRe = regexp.MustCompile(`[^a-zA-Z0-9_]`)

// ProjectNameFromPath returns the directory name for display purposes (e.g. "code" from "/Users/…/code").
// Sanitized for Qdrant naming rules: only [a-zA-Z0-9_]. Falls back to "project".
func ProjectNameFromPath(projectPath string) string {
	name := strings.TrimSpace(filepath.Base(filepath.Clean(projectPath)))
	if name == "" || name == "." {
		return "project"
	}
	sanitized := qdrantCollectionNameRe.ReplaceAllString(name, "_")
	if sanitized == "" {
		return "project"
	}
	return sanitized
}

// IndexStatus holds the parsed response from get_index_status or get_git_index_status.
// The server assigns the collectionName internally (e.g. "code_ff555f18") — we discover it here.
type IndexStatus struct {
	Status         string `json:"status"`         // "not_indexed" | "indexing" | "indexed"
	CollectionName string `json:"collectionName"` // actual Qdrant collection name assigned by server
	ChunksCount    int    `json:"chunksCount"`
	LastUpdated    string `json:"lastUpdated"`   // code index
	LastIndexedAt  string `json:"lastIndexedAt"` // git index
}

// IsIndexed returns true when the codebase/git history is fully indexed.
func (s *IndexStatus) IsIndexed() bool { return s.Status == "indexed" }

// LastIndexedTime returns whichever timestamp is present (code or git).
func (s *IndexStatus) LastIndexedTime() string {
	if s.LastUpdated != "" {
		return s.LastUpdated
	}
	return s.LastIndexedAt
}

// ParseIndexStatus extracts IndexStatus from the MCP tool text result.
// get_index_status and get_git_index_status both return JSON inside a "text" content block.
func ParseIndexStatus(result *mcp.CallToolResult) (*IndexStatus, error) {
	if result == nil {
		return nil, fmt.Errorf("nil result")
	}

	// Extract the text payload from the first content block
	raw, err := json.Marshal(result)
	if err != nil {
		return nil, err
	}

	var wrapper struct {
		Content []struct {
			Type string `json:"type"`
			Text string `json:"text"`
		} `json:"content"`
	}
	if err := json.Unmarshal(raw, &wrapper); err != nil {
		return nil, err
	}

	var text string
	for _, c := range wrapper.Content {
		if c.Type == "text" {
			text = c.Text
			break
		}
	}
	if text == "" {
		return nil, fmt.Errorf("no text content in result")
	}

	// "not indexed" responses are plain text, not JSON
	if !strings.HasPrefix(strings.TrimSpace(text), "{") {
		return &IndexStatus{Status: "not_indexed"}, nil
	}

	var status IndexStatus
	if err := json.Unmarshal([]byte(text), &status); err != nil {
		return nil, fmt.Errorf("failed to parse status JSON: %w", err)
	}
	return &status, nil
}

// MCPClient is an interface to allow mocking of the mcp-go client
type MCPClient interface {
	Ping(ctx context.Context) error
	CallTool(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)
	GetClient() *client.Client // Helper to let us run Start/Init, since the interface methods are complicated
}

// Manager wraps the mcp-go client
type Manager struct {
	client *client.Client // we'll use the concrete type to make it easier to initialize
}

// NewManager creates a new MCP Manager
func NewManager(c *client.Client) *Manager {
	return &Manager{client: c}
}

// GetDefaultClient returns a real HTTP client configured to connect to localhost:3000
func GetDefaultClient() (*client.Client, error) {
	// Using 127.0.0.1 over localhost to prevent IPv6 [::1] connection refused errors
	mcpClient, err := client.NewStreamableHttpClient("http://127.0.0.1:3000/mcp")
	if err != nil {
		return nil, err
	}
	return mcpClient, nil
}

// adaptEnvForStdio rewrites environment variables from the ezcode config format
// to what the qdrant-mcp-server process actually expects when running on the host:
//
//   - QDRANT_URL: replace "host.docker.internal" with "localhost" — that Docker
//     DNS alias only resolves inside containers; on the host Qdrant listens on localhost.
//   - OPENAI_BASE_URL → EMBEDDING_BASE_URL: the server's embedding factory reads
//     EMBEDDING_BASE_URL, not OPENAI_BASE_URL.
//   - TRANSPORT_MODE is forced to "stdio" regardless of what the config says.
func adaptEnvForStdio(env map[string]string, projectPath string) []string {
	// Start from the current process environment so Node.js (and its native
	// modules) can find all required runtime paths.
	base := os.Environ()
	overrides := make([]string, 0, len(env)+4)

	for k, v := range env {
		switch k {
		case "QDRANT_URL":
			// host.docker.internal resolves inside Docker only; rewrite to localhost.
			v = strings.ReplaceAll(v, "host.docker.internal", "localhost")
			overrides = append(overrides, "QDRANT_URL="+v)
		case "OPENAI_BASE_URL":
			// Server reads EMBEDDING_BASE_URL, not OPENAI_BASE_URL.
			overrides = append(overrides, "EMBEDDING_BASE_URL="+v)
			// Keep OPENAI_BASE_URL too in case future versions use it.
			overrides = append(overrides, "OPENAI_BASE_URL="+v)
		default:
			overrides = append(overrides, k+"="+v)
		}
	}

	// Hard overrides — must come after the loop so they always win.
	overrides = append(overrides,
		"PROJECT_PATH="+projectPath,
		"TRANSPORT_MODE=stdio",
	)

	return append(base, overrides...)
}

// GetStdioClient launches the MCP server as a local Node.js subprocess and
// returns a stdio-transport client connected to it.
// serverPath is the path to the built index.js (e.g. /home/user/qdrant-mcp-server/build/index.js).
// env is the environment variables from the ezcode config (will be adapted for host use).
// projectPath is the local filesystem path that will be served as the project root.
func GetStdioClient(serverPath string, env map[string]string, projectPath string) (*client.Client, error) {
	envSlice := adaptEnvForStdio(env, projectPath)

	c, err := client.NewStdioMCPClient("node", envSlice, serverPath)
	if err != nil {
		return nil, fmt.Errorf("failed to start stdio MCP client: %w", err)
	}
	return c, nil
}

// Initialize attempts to start and initialize the connection
func (m *Manager) Initialize(ctx context.Context) error {
	if m.client == nil {
		return fmt.Errorf("client is nil")
	}

	if err := m.client.Start(ctx); err != nil {
		return fmt.Errorf("failed to start sse client: %w", err)
	}

	req := mcp.InitializeRequest{}
	req.Params.ProtocolVersion = mcp.LATEST_PROTOCOL_VERSION
	req.Params.ClientInfo = mcp.Implementation{
		Name:    "ezcode",
		Version: "1.0",
	}
	req.Params.Capabilities = mcp.ClientCapabilities{}

	_, err := m.client.Initialize(ctx, req)
	return err
}

// Ping checks if the MCP server is alive
func (m *Manager) Ping(ctx context.Context) error {
	if m.client == nil {
		return context.DeadlineExceeded
	}
	return m.client.Ping(ctx)
}

// callToolWithRetry handles graceful degradation and connection retries (3x max)
func (m *Manager) callToolWithRetry(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	if m.client == nil {
		return nil, context.DeadlineExceeded
	}

	maxRetries := 3
	var lastErr error

	for i := 0; i < maxRetries; i++ {
		res, err := m.client.CallTool(ctx, req)
		if err == nil {
			return res, nil
		}

		lastErr = err

		if ctx.Err() != nil {
			return nil, ctx.Err()
		}

		// Simple backoff: 1s, 2s, 4s…
		time.Sleep(time.Duration(1<<i) * time.Second)
	}

	return nil, fmt.Errorf("tool call failed after %d retries. Last error: %w", maxRetries, lastErr)
}

// GetIndexStatus calls get_index_status and returns a parsed IndexStatus.
// The server assigns collectionName internally (e.g. "code_ff555f18") — use this to discover it.
func (m *Manager) GetIndexStatus(ctx context.Context, directoryPath string) (*IndexStatus, error) {
	req := mcp.CallToolRequest{}
	req.Params.Name = "get_index_status"
	req.Params.Arguments = map[string]interface{}{"path": directoryPath}
	result, err := m.callToolWithRetry(ctx, req)
	if err != nil {
		return nil, err
	}
	return ParseIndexStatus(result)
}

// GetGitIndexStatus calls get_git_index_status and returns a parsed IndexStatus.
// The server assigns collectionName internally (e.g. "git_ff555f18") — use this to discover it.
func (m *Manager) GetGitIndexStatus(ctx context.Context, directoryPath string) (*IndexStatus, error) {
	req := mcp.CallToolRequest{}
	req.Params.Name = "get_git_index_status"
	req.Params.Arguments = map[string]interface{}{"path": directoryPath}
	result, err := m.callToolWithRetry(ctx, req)
	if err != nil {
		return nil, err
	}
	return ParseIndexStatus(result)
}

// IndexCodebase calls the index_codebase tool.
func (m *Manager) IndexCodebase(ctx context.Context, directoryPath string) (*mcp.CallToolResult, error) {
	req := mcp.CallToolRequest{}
	req.Params.Name = "index_codebase"
	req.Params.Arguments = map[string]interface{}{"path": directoryPath}
	return m.callToolWithRetry(ctx, req)
}

// IndexGitHistory calls the index_git_history tool.
func (m *Manager) IndexGitHistory(ctx context.Context, directoryPath string) (*mcp.CallToolResult, error) {
	req := mcp.CallToolRequest{}
	req.Params.Name = "index_git_history"
	req.Params.Arguments = map[string]interface{}{"path": directoryPath}
	return m.callToolWithRetry(ctx, req)
}

// ContextualSearch calls the contextual_search tool.
func (m *Manager) ContextualSearch(ctx context.Context, directoryPath string, query string, codeLimit int, gitLimit int) (*mcp.CallToolResult, error) {
	req := mcp.CallToolRequest{}
	req.Params.Name = "contextual_search"
	req.Params.Arguments = map[string]interface{}{
		"path":      directoryPath,
		"query":     query,
		"codeLimit": codeLimit,
		"gitLimit":  gitLimit,
		"correlate": true,
	}
	return m.callToolWithRetry(ctx, req)
}

// ReindexChanges calls the reindex_changes tool for incremental codebase indexing.
func (m *Manager) ReindexChanges(ctx context.Context, directoryPath string) (*mcp.CallToolResult, error) {
	req := mcp.CallToolRequest{}
	req.Params.Name = "reindex_changes"
	req.Params.Arguments = map[string]interface{}{"path": directoryPath}
	return m.callToolWithRetry(ctx, req)
}

// PromptInfo is a slim summary of a server-side MCP prompt (name + description).
type PromptInfo struct {
	Name        string
	Description string
}

// ListPrompts fetches the list of prompts exposed by the MCP server.
// Returns an empty slice (not an error) when the server has no prompts configured.
func (m *Manager) ListPrompts(ctx context.Context) ([]PromptInfo, error) {
	if m.client == nil {
		return nil, fmt.Errorf("client is nil")
	}

	req := mcp.ListPromptsRequest{}
	result, err := m.client.ListPrompts(ctx, req)
	if err != nil {
		// Some servers return an error when no prompts file exists — treat as empty.
		return nil, nil //nolint:nilerr
	}

	infos := make([]PromptInfo, 0, len(result.Prompts))
	for _, p := range result.Prompts {
		infos = append(infos, PromptInfo{
			Name:        p.Name,
			Description: p.Description,
		})
	}
	return infos, nil
}

// GetPrompt retrieves an expanded prompt from the server with the given name and arguments.
// The returned string is the concatenated text content of all PromptMessages.
func (m *Manager) GetPrompt(ctx context.Context, name string, arguments map[string]string) (string, error) {
	if m.client == nil {
		return "", fmt.Errorf("client is nil")
	}

	req := mcp.GetPromptRequest{}
	req.Params.Name = name
	req.Params.Arguments = arguments

	result, err := m.client.GetPrompt(ctx, req)
	if err != nil {
		return "", fmt.Errorf("failed to get prompt %q: %w", name, err)
	}

	var sb strings.Builder
	for _, msg := range result.Messages {
		// Each message content is an interface — extract text blocks.
		b, merr := json.Marshal(msg.Content)
		if merr != nil {
			continue
		}
		var raw map[string]interface{}
		if json.Unmarshal(b, &raw) != nil {
			continue
		}
		if t, ok := raw["type"].(string); ok && t == "text" {
			if text, ok := raw["text"].(string); ok {
				if sb.Len() > 0 {
					sb.WriteString("\n")
				}
				sb.WriteString(text)
			}
		}
	}
	return sb.String(), nil
}
