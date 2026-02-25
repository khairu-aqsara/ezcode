package components

import (
	"context"
	"fmt"
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/khairu-aqsara/ezcode/internal/config"
	"github.com/khairu-aqsara/ezcode/internal/docker"
	"github.com/khairu-aqsara/ezcode/internal/mcp"
)

type DashboardState int

const (
	DashboardCheckingMCP DashboardState = iota
	DashboardStartingDocker
	DashboardConnectingMCP
	DashboardIndexingCodebase
	DashboardIndexingGit
	DashboardReady
	DashboardError
)

type DashboardModel struct {
	cfg           *config.Config
	dockerMgr     *docker.Manager
	mcpMgr        *mcp.Manager
	state         DashboardState
	spinner       spinner.Model
	statusMsg     string
	err           error
	composePath   string
	projectPath   string // host-side project path (used for stdio mode)
	startedDocker bool
}

// StartedDocker reports whether this session launched Docker via compose up.
func (m *DashboardModel) StartedDocker() bool { return m.startedDocker }

func NewDashboard(cfg *config.Config, composePath string, projectPath string) *DashboardModel {
	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = lipgloss.NewStyle().Foreground(lipgloss.Color("205"))

	return &DashboardModel{
		cfg:         cfg,
		dockerMgr:   docker.NewManager(nil),
		mcpMgr:      nil,
		state:       DashboardCheckingMCP,
		spinner:     s,
		statusMsg:   "Checking MCP Server...",
		composePath: composePath,
		projectPath: projectPath,
	}
}

func (m *DashboardModel) Init() tea.Cmd {
	return tea.Batch(
		m.spinner.Tick,
		m.pingMCPFirstCmd,
	)
}

func (m *DashboardModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd

	case needsDockerMsg:
		if m.cfg.IsStdioMode() {
			// In stdio mode we launch the Node.js process directly — no Docker needed.
			m.state = DashboardConnectingMCP
			m.statusMsg = "Launching MCP Server (stdio)..."
			return m, m.connectMCPCmd
		}
		m.state = DashboardStartingDocker
		m.statusMsg = "Starting Qdrant MCP Server..."
		return m, m.startDockerCmd

	case dockerStartedMsg:
		m.startedDocker = true
		m.state = DashboardConnectingMCP
		m.statusMsg = "Connecting to MCP Server..."
		return m, m.connectMCPCmd

	case mcpConnectedMsg:
		m.mcpMgr = msg.mgr
		m.state = DashboardReady
		m.statusMsg = "Connected to MCP! Ready to chat and index."
		return m, nil

	case errorMsg:
		m.state = DashboardError
		m.err = msg.err
		m.statusMsg = fmt.Sprintf("Error: %v", m.err)
		return m, nil
	}
	return m, nil
}

func (m *DashboardModel) View() string {
	var content string
	if m.state == DashboardError {
		content = lipgloss.NewStyle().Foreground(lipgloss.Color("196")).Render(m.statusMsg)
	} else if m.state == DashboardReady {
		content = lipgloss.NewStyle().Foreground(lipgloss.Color("42")).Render(m.statusMsg)
	} else {
		content = fmt.Sprintf("%s %s", m.spinner.View(), m.statusMsg)
	}

	return content
}

// Commands

type needsDockerMsg struct{}
type dockerStartedMsg struct{}
type mcpConnectedMsg struct{ mgr *mcp.Manager }
type codebaseIndexedMsg struct{}
type gitIndexedMsg struct{}
type errorMsg struct{ err error }

// pingMCPFirstCmd tries to reach an already-running MCP server with a short
// timeout. In stdio mode, there is never a pre-existing server, so we proceed
// directly to launching the subprocess. In HTTP/Docker mode, a successful ping
// means Docker is already up and we can skip compose up.
func (m *DashboardModel) pingMCPFirstCmd() tea.Msg {
	// Stdio mode: no HTTP server to pre-check — go straight to connect.
	if m.cfg.IsStdioMode() {
		return needsDockerMsg{}
	}

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	c, err := mcp.GetDefaultClient()
	if err != nil {
		return needsDockerMsg{}
	}

	mgr := mcp.NewManager(c)
	if err := mgr.Initialize(ctx); err != nil {
		return needsDockerMsg{}
	}
	if err := mgr.Ping(ctx); err != nil {
		return needsDockerMsg{}
	}

	return mcpConnectedMsg{mgr: mgr}
}

func (m *DashboardModel) startDockerCmd() tea.Msg {
	// Attempt to compose up
	err := m.dockerMgr.ComposeUp(context.Background(), m.composePath, m.cfg.Env)
	if err != nil {
		return errorMsg{fmt.Errorf("failed to start docker: %w", err)}
	}
	return dockerStartedMsg{}
}

func (m *DashboardModel) connectMCPCmd() tea.Msg {
	if m.cfg.IsStdioMode() {
		return m.connectStdioCmd()
	}
	return m.connectHTTPCmd()
}

// connectStdioCmd spawns the MCP server as a local Node.js subprocess.
func (m *DashboardModel) connectStdioCmd() tea.Msg {
	serverPath := m.cfg.ServerPath
	if serverPath == "" {
		return errorMsg{fmt.Errorf("stdio mode requires server_path in config (path to build/index.js)")}
	}
	projectPath := m.projectPath
	if projectPath == "" {
		projectPath = m.cfg.Env["PROJECT_PATH"]
	}
	c, err := mcp.GetStdioClient(serverPath, m.cfg.Env, projectPath)
	if err != nil {
		return errorMsg{fmt.Errorf("failed to launch stdio MCP server: %w", err)}
	}
	mgr := mcp.NewManager(c)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if err = mgr.Initialize(ctx); err != nil {
		return errorMsg{fmt.Errorf("stdio MCP initialize failed: %w", err)}
	}
	if err = mgr.Ping(ctx); err != nil {
		return errorMsg{fmt.Errorf("stdio MCP ping failed: %w", err)}
	}
	return mcpConnectedMsg{mgr: mgr}
}

// connectHTTPCmd connects to an already-running HTTP MCP server (Docker mode).
func (m *DashboardModel) connectHTTPCmd() tea.Msg {
	var err error
	httpClient, cerr := mcp.GetDefaultClient()
	if cerr != nil {
		return errorMsg{fmt.Errorf("failed to create MCP client: %w", cerr)}
	}

	mgr := mcp.NewManager(httpClient)
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	// Initial immediate attempt
	if err = mgr.Initialize(ctx); err == nil {
		if err = mgr.Ping(ctx); err == nil {
			return mcpConnectedMsg{mgr: mgr}
		}
	}

	for {
		select {
		case <-ctx.Done():
			return errorMsg{fmt.Errorf("timeout waiting for MCP server. last err: %v", err)}
		case <-ticker.C:
			err = mgr.Initialize(context.Background())
			if err != nil {
				continue
			}
			err = mgr.Ping(context.Background())
			if err == nil {
				return mcpConnectedMsg{mgr: mgr}
			}
		}
	}
}

func (m *DashboardModel) indexCodebaseCmd() tea.Msg {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	_, err := m.mcpMgr.IndexCodebase(ctx, "/app/project")
	if err != nil {
		return errorMsg{fmt.Errorf("failed to index codebase: %w", err)}
	}
	return codebaseIndexedMsg{}
}

func (m *DashboardModel) indexGitCmd() tea.Msg {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	_, err := m.mcpMgr.IndexGitHistory(ctx, "/app/project")
	if err != nil {
		return errorMsg{fmt.Errorf("failed to index git history: %w", err)}
	}
	return gitIndexedMsg{}
}
