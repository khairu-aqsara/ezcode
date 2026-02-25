package components

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/glamour"
	"github.com/charmbracelet/lipgloss"
	"github.com/khairu-aqsara/ezcode/internal/config"
	"github.com/khairu-aqsara/ezcode/internal/mcp"
)

// Adaptive palette — each color has a light-terminal and dark-terminal variant.
// Semantic colors (cyan, purple, green, yellow) are saturated enough to work on both backgrounds.
var (
	colorMuted  = lipgloss.AdaptiveColor{Light: "240", Dark: "244"} // timestamps, hints
	colorSubtle = lipgloss.AdaptiveColor{Light: "248", Dark: "238"} // borders
	colorLabel  = lipgloss.AdaptiveColor{Light: "237", Dark: "245"} // dim labels
	colorText   = lipgloss.AdaptiveColor{Light: "235", Dark: "252"} // body text

	tabStyle = lipgloss.NewStyle().
			Border(lipgloss.NormalBorder(), true).
			BorderForeground(colorSubtle).
			Padding(0, 1)

	activeTabStyle = tabStyle.
			BorderForeground(lipgloss.Color("36")).
			Foreground(lipgloss.Color("36")).
			Bold(true)
)

const (
	TabIndexSetup = iota
	TabIndexIndexCodebase
	TabIndexIndexGit
	TabIndexChat
	TabCount
)

// ChatMessage is a single turn in the conversation (user question or AI answer).
type ChatMessage struct {
	Role      string // "user" | "ai" | "system"
	Content   string // raw text for user; pre-rendered markdown for ai
	Timestamp time.Time
}

type ChatModel struct {
	cfg         *config.Config
	composePath string
	projectPath string // host-side project path (from CWD at launch)
	isFirstRun  bool

	configForm *ConfigForm
	dashboard  *DashboardModel
	mcpMgr     *mcp.Manager

	viewports   []viewport.Model
	input       textinput.Model
	spinner     spinner.Model
	isLoading   bool
	messages    [][]string     // plain-text log for non-chat tabs (index tabs etc.)
	chatMsgs    []*ChatMessage // structured conversation for the Chat tab
	activeTab   int
	err         error
	initialized bool

	codeStatus *mcp.IndexStatus
	gitStatus  *mcp.IndexStatus

	// Prompts support
	prompts       []mcp.PromptInfo // list fetched from server after connect
	showPrompts   bool             // whether the prompts picker overlay is visible
	promptsFilter string           // current filter string (typed in picker)
	promptsCursor int              // selected index in the filtered list
}

func NewChat(cfg *config.Config, composePath string, projectPath string, isFirstRun bool) *ChatModel {
	in := textinput.New()
	in.Placeholder = "Ask a question about the code..."
	in.Focus()
	in.CharLimit = 256
	in.Width = 50

	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = lipgloss.NewStyle().Foreground(lipgloss.Color("205"))

	viewports := make([]viewport.Model, TabCount)
	messages := make([][]string, TabCount)

	for i := 0; i < TabCount; i++ {
		vp := viewport.New(80, 20)
		msg := "Welcome! Type your question below."
		if i == TabIndexIndexCodebase {
			msg = "Checking index status…"
		} else if i == TabIndexIndexGit {
			msg = "Checking git index status…"
		} else if i == TabIndexSetup {
			msg = "Configuration Setup"
		}
		vp.SetContent(msg)
		viewports[i] = vp
		messages[i] = make([]string, 0)
	}

	activeTab := TabIndexChat
	if isFirstRun {
		activeTab = TabIndexSetup
	}

	return &ChatModel{
		cfg:         cfg,
		composePath: composePath,
		projectPath: projectPath,
		isFirstRun:  isFirstRun,
		configForm:  NewConfigForm(cfg, !isFirstRun),
		dashboard:   NewDashboard(cfg, composePath, projectPath),
		viewports:   viewports,
		input:       in,
		spinner:     s,
		messages:    messages,
		activeTab:   activeTab,
	}
}

func (m *ChatModel) Init() tea.Cmd {
	m.initialized = true
	var cmds []tea.Cmd
	cmds = append(cmds, textinput.Blink, m.spinner.Tick, m.configForm.Init())

	if !m.isFirstRun {
		cmds = append(cmds, m.dashboard.Init())
	}
	return tea.Batch(cmds...)
}

func (m *ChatModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd
	var cmd tea.Cmd

	if !m.initialized {
		cmds = append(cmds, m.Init())
	}

	switch msg := msg.(type) {
	case promptsResultMsg:
		m.prompts = msg.prompts

	case tea.KeyMsg:
		if m.isLoading {
			break
		}

		// Prompts picker overlay intercepts all keys when open.
		if m.showPrompts {
			cmds = append(cmds, m.handlePromptsKey(msg)...)
			return m, tea.Batch(cmds...)
		}

		if m.activeTab == TabIndexSetup {
			if msg.Type == tea.KeyEsc && !m.isFirstRun {
				m.activeTab = TabIndexChat
				return m, nil
			}
			if msg.Type == tea.KeyEnter {
				if err := m.configForm.Validate(); err == nil {
					m.configForm.SaveToConfig()

					homeDir, _ := os.UserHomeDir()
					configPath := filepath.Join(homeDir, ".ezcode", "config.json")
					_ = config.SaveConfig(m.cfg, configPath)

					// Trigger restart of docker & dashboard
					m.isFirstRun = false       // Mark first run as complete if it was true
					m.isLoading = true         // prevent other input while restarting
					m.activeTab = TabIndexChat // Move to chat automatically
					cmds = append(cmds, m.restartDockerCmd())
					return m, tea.Batch(cmds...)
				}
			}
			fModel, fCmd := m.configForm.Update(msg)
			m.configForm = fModel.(*ConfigForm)
			cmds = append(cmds, fCmd)
			break
		}

		switch msg.Type {
		case tea.KeyEnter:
			if m.activeTab == TabIndexIndexCodebase || m.activeTab == TabIndexIndexGit {
				// Don't index if MCP isn't ready
				if m.dashboard.state != DashboardReady {
					m.addMessage(m.activeTab, "MCP Server not ready yet. Please wait.")
					return m, nil
				}

				m.isLoading = true
				if m.activeTab == TabIndexIndexCodebase {
					m.addMessage(m.activeTab, "Triggering Smart Indexing...")
					cmds = append(cmds, m.indexCmd("codebase", m.activeTab))
				} else {
					m.addMessage(m.activeTab, "Starting git history indexing...")
					cmds = append(cmds, m.indexCmd("git", m.activeTab))
				}
				return m, tea.Batch(cmds...)
			}

			if m.activeTab == TabIndexChat {
				val := m.input.Value()
				if val == "" {
					return m, nil
				}

				if m.dashboard.state != DashboardReady {
					m.addChatMessage("system", "MCP Server not ready yet. Please wait.")
					m.input.SetValue("")
					return m, nil
				}

				// Handle /prompt <name> [arg=value ...] command.
				if strings.HasPrefix(val, "/prompt ") {
					cmds = append(cmds, m.executePromptCmd(val))
					m.input.SetValue("")
					return m, tea.Batch(cmds...)
				}

				m.addChatMessage("user", val)
				m.input.SetValue("")
				m.isLoading = true

				cmds = append(cmds, m.searchCmd(val, "context", m.activeTab))
			}

		case tea.KeyCtrlP:
			// Ctrl+P opens the prompts picker when on Chat tab.
			if m.activeTab == TabIndexChat && len(m.prompts) > 0 {
				m.showPrompts = true
				m.promptsFilter = ""
				m.promptsCursor = 0
				return m, nil
			}

		case tea.KeyTab, tea.KeyRight:
			m.activeTab = (m.activeTab + 1) % TabCount
			return m, nil

		case tea.KeyShiftTab, tea.KeyLeft:
			m.activeTab = (m.activeTab - 1 + TabCount) % TabCount
			return m, nil

		case tea.KeyUp, tea.KeyDown, tea.KeyPgUp, tea.KeyPgDown:
			m.viewports[m.activeTab], cmd = m.viewports[m.activeTab].Update(msg)
			cmds = append(cmds, cmd)
			return m, tea.Batch(cmds...)
		}

	case tea.WindowSizeMsg:
		for i := range m.viewports {
			m.viewports[i].Width = msg.Width
			m.viewports[i].Height = msg.Height - 10
		}
		m.input.Width = msg.Width - 4
		// Re-render chat messages at the new width
		if len(m.chatMsgs) > 0 {
			m.refreshChatViewport()
		}

	case searchResultMsg:
		m.isLoading = false
		if msg.tabIndex == TabIndexChat {
			rendered, err := glamour.Render(msg.content, "auto")
			if err != nil {
				m.addChatMessage("ai", msg.content)
			} else {
				m.addChatMessage("ai", rendered)
			}
		} else {
			m.addMessage(msg.tabIndex, msg.content)
		}
		// Refresh collection status after any indexing operation
		if msg.tabIndex == TabIndexIndexCodebase || msg.tabIndex == TabIndexIndexGit {
			cmds = append(cmds, m.fetchStatusCmd())
		}

	case errorMsg:
		m.isLoading = false
		if m.activeTab == TabIndexChat {
			m.addChatMessage("system", "Error: "+msg.err.Error())
		} else {
			m.addMessage(m.activeTab, "Error: "+msg.err.Error())
		}

	case spinner.TickMsg:
		m.spinner, cmd = m.spinner.Update(msg)
		cmds = append(cmds, cmd)
		// Also forward to dashboard spinner if not ready
		if m.dashboard.state != DashboardReady {
			dModel, dCmd := m.dashboard.Update(msg)
			m.dashboard = dModel.(*DashboardModel)
			cmds = append(cmds, dCmd)
		}

	case restartDockerMsg:
		m.isLoading = false
		// Reset dashboard to trigger docker up and MCP connect sequence again
		m.dashboard = NewDashboard(m.cfg, m.composePath, m.projectPath)
		m.mcpMgr = nil // Clear current MCP info connection explicitly
		cmds = append(cmds, m.dashboard.Init())

	case indexStatusResultMsg:
		m.codeStatus = msg.codeStatus
		m.gitStatus = msg.gitStatus
		m.viewports[TabIndexIndexCodebase].SetContent(m.buildIndexTabContent(false))
		m.viewports[TabIndexIndexGit].SetContent(m.buildIndexTabContent(true))

	// Catch dashboard specific messages
	case needsDockerMsg, dockerStartedMsg, mcpConnectedMsg:
		dModel, dCmd := m.dashboard.Update(msg)
		m.dashboard = dModel.(*DashboardModel)
		cmds = append(cmds, dCmd)

		if cMsg, ok := msg.(mcpConnectedMsg); ok {
			m.mcpMgr = cMsg.mgr
			// Fetch index status and prompts list as soon as MCP is connected
			cmds = append(cmds, m.fetchStatusCmd(), m.fetchPromptsCmd())
		}
	}

	// Always update input if we are in chat tab and not loading
	if !m.isLoading && m.activeTab == TabIndexChat {
		m.input, cmd = m.input.Update(msg)
		cmds = append(cmds, cmd)
	}

	return m, tea.Batch(cmds...)
}

func (m *ChatModel) View() string {
	docStyle := lipgloss.NewStyle().Margin(1, 2)

	tabs := make([]string, TabCount)
	for i, title := range []string{"Setup", "Index Codebase", "Index Git", "Chat"} {
		if i == m.activeTab {
			tabs[i] = activeTabStyle.Render(title)
		} else {
			tabs[i] = tabStyle.Render(title)
		}
	}

	tabRow := lipgloss.JoinHorizontal(lipgloss.Top, tabs...)

	var contentArea string
	if m.activeTab == TabIndexSetup {
		contentArea = m.configForm.View()
	} else {
		contentArea = m.viewports[m.activeTab].View()
	}

	// Prompts overlay replaces the bottom row when active.
	if m.showPrompts {
		promptsOverlay := m.renderPromptsOverlay()
		ui := fmt.Sprintf("%s\n\n%s\n\n%s", tabRow, contentArea, promptsOverlay)
		return docStyle.Render(ui)
	}

	var bottomRow string
	if m.dashboard.state != DashboardReady {
		// If MCP isn't ready or docker is starting up, display its state at the bottom
		bottomRow = m.dashboard.View()
	} else if m.isLoading {
		if m.activeTab == TabIndexChat {
			bottomRow = lipgloss.JoinHorizontal(lipgloss.Center,
				fmt.Sprintf("%s Thinking...", m.spinner.View()),
			)
		} else {
			bottomRow = lipgloss.JoinHorizontal(lipgloss.Center,
				fmt.Sprintf("%s Indexing...", m.spinner.View()),
			)
		}
	} else if m.activeTab == TabIndexChat {
		// Show prompts hint if prompts are available.
		promptHint := ""
		if len(m.prompts) > 0 {
			promptHint = lipgloss.NewStyle().Foreground(colorMuted).Italic(true).
				Render(fmt.Sprintf("  Ctrl+P: prompts (%d)", len(m.prompts)))
		}
		bottomRow = m.input.View() + promptHint
	}

	ui := fmt.Sprintf("%s\n\n%s\n\n%s", tabRow, contentArea, bottomRow)
	return docStyle.Render(ui)
}

// addMessage appends a plain-text entry to a non-chat tab (index tabs etc.) and refreshes it.
func (m *ChatModel) addMessage(tabIndex int, msg string) {
	m.messages[tabIndex] = append(m.messages[tabIndex], msg)
	m.viewports[tabIndex].SetContent(strings.Join(m.messages[tabIndex], "\n\n"))
	m.viewports[tabIndex].GotoBottom()
}

// addChatMessage appends a structured message to the Chat tab and refreshes the viewport.
func (m *ChatModel) addChatMessage(role, content string) {
	m.chatMsgs = append(m.chatMsgs, &ChatMessage{
		Role:      role,
		Content:   content,
		Timestamp: time.Now(),
	})
	m.refreshChatViewport()
}

// refreshChatViewport rebuilds the Chat tab viewport from all chatMsgs.
func (m *ChatModel) refreshChatViewport() {
	width := m.viewports[TabIndexChat].Width
	if width <= 0 {
		width = 80
	}
	var b strings.Builder
	for i, msg := range m.chatMsgs {
		if i > 0 {
			b.WriteString("\n\n") // one blank line between turns
		}
		b.WriteString(renderChatMessage(msg, width))
	}
	m.viewports[TabIndexChat].SetContent(b.String())
	m.viewports[TabIndexChat].GotoBottom()
}

// renderChatMessage returns a styled string for a single chat message.
func renderChatMessage(msg *ChatMessage, width int) string {
	// Timestamp: bracketed, muted — readable on both light and dark backgrounds
	ts := lipgloss.NewStyle().
		Foreground(colorMuted).
		Render("[" + msg.Timestamp.Format("15:04:05") + "]")

	innerWidth := width - 8
	if innerWidth < 20 {
		innerWidth = 20
	}

	switch msg.Role {
	case "user":
		label := lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("39")).
			Render("You")

		header := lipgloss.NewStyle().
			Width(innerWidth).
			Render(label + "  " + ts)

		contentStyle := lipgloss.NewStyle().
			BorderLeft(true).
			BorderStyle(lipgloss.ThickBorder()).
			BorderForeground(lipgloss.Color("39")).
			PaddingLeft(2).
			Foreground(colorText).
			Width(innerWidth)

		return header + "\n" + contentStyle.Render(msg.Content)

	case "ai":
		label := lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("141")).
			Render("✦ AI")

		header := lipgloss.NewStyle().
			Width(innerWidth).
			Render(label + "  " + ts)

		contentStyle := lipgloss.NewStyle().
			BorderLeft(true).
			BorderStyle(lipgloss.ThickBorder()).
			BorderForeground(lipgloss.Color("141")).
			PaddingLeft(2).
			Width(innerWidth)

		return header + "\n" + contentStyle.Render(strings.TrimRight(msg.Content, "\n"))

	default: // "system" — muted notices (errors, warnings)
		return lipgloss.NewStyle().
			Foreground(colorMuted).
			Italic(true).
			PaddingLeft(2).
			Render("· " + msg.Content + "  " + ts)
	}
}

// mcpProjectPath returns the path to pass to MCP tool calls.
// In Docker mode the container mounts the project at /app/project.
// In stdio mode the server runs on the host and can access the real path directly.
func (m *ChatModel) mcpProjectPath() string {
	if m.cfg.IsStdioMode() {
		return m.projectPath
	}
	return "/app/project"
}

// Commands

type searchResultMsg struct {
	content  string
	tabIndex int
}

type restartDockerMsg struct{}

func (m *ChatModel) restartDockerCmd() tea.Cmd {
	return func() tea.Msg {
		// Run compose down first
		_ = m.dashboard.dockerMgr.ComposeDown(context.Background(), m.composePath, m.cfg.Env)
		// We ignore the error and proceed to tell ChatModel it finished terminating
		return restartDockerMsg{}
	}
}

func (m *ChatModel) searchCmd(query string, reqType string, tabIndex int) tea.Cmd {
	return func() tea.Msg {
		if m.mcpMgr == nil {
			return errorMsg{fmt.Errorf("MCP Manager not initialized")}
		}
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
		defer cancel()

		codeLimit := 5
		gitLimit := 2
		if reqType == "code" {
			gitLimit = 0
		} else if reqType == "git" {
			codeLimit = 0
		}

		result, err := m.mcpMgr.ContextualSearch(ctx, m.mcpProjectPath(), query, codeLimit, gitLimit)
		if err != nil {
			return errorMsg{err}
		}

		// We expect text content back. Let's merge it all.
		var final string
		for _, c := range result.Content {
			b, err := json.Marshal(c)
			if err == nil {
				var raw map[string]interface{}
				if json.Unmarshal(b, &raw) == nil {
					if t, ok := raw["type"].(string); ok && t == "text" {
						if textVal, ok := raw["text"].(string); ok {
							final += textVal + "\n"
						}
					}
				}
			}
		}

		if final == "" {
			final = "No results found or blank response."
		}

		if result.IsError {
			return errorMsg{fmt.Errorf("MCP Tool Error: %s", final)}
		}

		// RAG Step: Call LLM with the context string
		llmModel := m.cfg.Env["LLM_MODEL"]
		if llmModel == "" {
			llmModel = "gemini-2.5-flash"
		}

		baseURL := strings.TrimRight(m.cfg.Env["OPENAI_BASE_URL"], "/")
		apiKey := m.cfg.Env["OPENAI_API_KEY"]

		if baseURL == "" || apiKey == "" {
			return searchResultMsg{content: "LLM Not configured (Missing API Key or Base URL). Returning raw context:\n\n" + final, tabIndex: tabIndex}
		}

		reqBody := map[string]interface{}{
			"model": llmModel,
			"messages": []map[string]interface{}{
				{"role": "system", "content": "You are a strict, expert coding assistant. Answer the user's question using ONLY the provided codebase and git history context. Do not use outside knowledge. **Crucially, when discussing code, you MUST include the actual code snippets from the context in your response to give the user exact details.** If the retrieved context is irrelevant or does not contain the answer to the user's question, you MUST say 'I cannot find the relevant information in the codebase context.' and stop. Do not guess or hallucinate."},
				{"role": "user", "content": fmt.Sprintf("Context:\n%s\n\nQuestion: %s", final, query)},
			},
		}

		jsonBody, err := json.Marshal(reqBody)
		if err != nil {
			return errorMsg{fmt.Errorf("Failed to build LLM request: %v", err)}
		}

		apiURL := baseURL + "/chat/completions"
		httpReq, err := http.NewRequestWithContext(ctx, "POST", apiURL, bytes.NewBuffer(jsonBody))
		if err != nil {
			return errorMsg{fmt.Errorf("Failed to create LLM HTTP request: %v", err)}
		}

		httpReq.Header.Set("Content-Type", "application/json")
		httpReq.Header.Set("Authorization", "Bearer "+apiKey)

		client := &http.Client{Timeout: 60 * time.Second}
		resp, err := client.Do(httpReq)
		if err != nil {
			return errorMsg{fmt.Errorf("LLM API call failed: %v", err)}
		}
		defer resp.Body.Close()

		body, err := io.ReadAll(resp.Body)
		if err != nil {
			return errorMsg{fmt.Errorf("Failed to read LLM response: %v", err)}
		}

		if resp.StatusCode != http.StatusOK {
			return errorMsg{fmt.Errorf("LLM API returned status %d: %s", resp.StatusCode, string(body))}
		}

		var apiResp map[string]interface{}
		if err := json.Unmarshal(body, &apiResp); err != nil {
			return errorMsg{fmt.Errorf("Failed to decode LLM response: %v", err)}
		}

		var answer string
		if choices, ok := apiResp["choices"].([]interface{}); ok && len(choices) > 0 {
			if choice, ok := choices[0].(map[string]interface{}); ok {
				if msgMap, ok := choice["message"].(map[string]interface{}); ok {
					if content, ok := msgMap["content"].(string); ok {
						answer = content
					}
				}
			}
		}

		if answer == "" {
			answer = "The LLM returned an empty response."
		}

		return searchResultMsg{content: answer, tabIndex: tabIndex}
	}
}

func (m *ChatModel) indexCmd(reqType string, tabIndex int) tea.Cmd {
	return func() tea.Msg {
		if m.mcpMgr == nil {
			return errorMsg{fmt.Errorf("MCP Manager not initialized")}
		}

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
		defer cancel()

		projectPath := m.mcpProjectPath()

		if reqType == "codebase" {
			// Smart Indexing: attempt incremental reindex, fall back to full index.
			// reindex_changes throws (via the MCP error chain) whenever the codebase
			// has never been indexed or its on-disk snapshot is missing. Both errors
			// surface as Go errors with messages containing one of these substrings:
			//   "Codebase not indexed: <path>"           – Qdrant collection absent
			//   "No previous snapshot found"             – snapshot file missing
			// We treat ANY reindex_changes error as a signal to run a full index,
			// because that tool is only meaningful after a first successful index_codebase.
			result, err := m.mcpMgr.ReindexChanges(ctx, projectPath)

			needsFullIndex := false
			if err != nil {
				needsFullIndex = true
			} else if result != nil && result.IsError {
				needsFullIndex = true
			}

			if needsFullIndex {
				result, err = m.mcpMgr.IndexCodebase(ctx, projectPath)
			}

			return m.processIndexResult(result, err, tabIndex)
		} else {
			result, err := m.mcpMgr.IndexGitHistory(ctx, projectPath)
			return m.processIndexResult(result, err, tabIndex)
		}
	}
}

func (m *ChatModel) processIndexResult(result interface{}, err error, tabIndex int) tea.Msg {
	if err != nil {
		return errorMsg{err}
	}
	b, marshalErr := json.Marshal(result)
	if marshalErr != nil {
		return errorMsg{fmt.Errorf("could not parse result")}
	}

	var raw map[string]interface{}
	json.Unmarshal(b, &raw)

	var final string
	if content, ok := raw["content"].([]interface{}); ok {
		for _, c := range content {
			if cmap, ok := c.(map[string]interface{}); ok {
				if t, ok := cmap["type"].(string); ok && t == "text" {
					if textVal, ok := cmap["text"].(string); ok {
						final += textVal + "\n"
					}
				}
			}
		}
	}

	isErr := false
	if e, ok := raw["isError"].(bool); ok {
		isErr = e
	}

	if isErr {
		if final == "" {
			final = "Unknown MCP tool error"
		}
		return errorMsg{fmt.Errorf("Index Failed: %s", final)}
	}

	if final == "" {
		final = "Indexing completed successfully!"
	}

	return searchResultMsg{content: final, tabIndex: tabIndex}
}

// indexStatusResultMsg carries freshly fetched code and git index statuses.
type indexStatusResultMsg struct {
	codeStatus *mcp.IndexStatus
	gitStatus  *mcp.IndexStatus
}

// fetchStatusCmd calls get_index_status and get_git_index_status in sequence and returns the results.
func (m *ChatModel) fetchStatusCmd() tea.Cmd {
	return func() tea.Msg {
		if m.mcpMgr == nil {
			return nil
		}
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()

		projectPath := m.mcpProjectPath()
		codeStatus, _ := m.mcpMgr.GetIndexStatus(ctx, projectPath)
		gitStatus, _ := m.mcpMgr.GetGitIndexStatus(ctx, projectPath)
		return indexStatusResultMsg{codeStatus: codeStatus, gitStatus: gitStatus}
	}
}

// buildIndexTabContent builds a styled status card for the Index Codebase / Index Git tabs.
func (m *ChatModel) buildIndexTabContent(isGit bool) string {
	var status *mcp.IndexStatus
	title := "Codebase Index"
	hint := "Press Enter — smart-index: incremental reindex, or full index if not yet indexed."
	if isGit {
		status = m.gitStatus
		title = "Git History Index"
		hint = "Press Enter — index git commit history."
	} else {
		status = m.codeStatus
	}

	// ── Styles ──────────────────────────────────────────────────────────────
	cardStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(colorSubtle).
		Padding(1, 3).
		Width(60)

	titleStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("39"))

	labelStyle := lipgloss.NewStyle().Foreground(colorLabel).Width(14)
	valueStyle := lipgloss.NewStyle().Foreground(colorText)

	dotGreen := lipgloss.NewStyle().Foreground(lipgloss.Color("82")).Render("●")
	dotYellow := lipgloss.NewStyle().Foreground(lipgloss.Color("220")).Render("●")
	dotGray := lipgloss.NewStyle().Foreground(colorMuted).Render("●")

	hintStyle := lipgloss.NewStyle().Foreground(colorMuted).Italic(true)

	row := func(label, value string) string {
		return labelStyle.Render(label) + valueStyle.Render(value)
	}

	// ── Build card body ──────────────────────────────────────────────────────
	var body string

	if status == nil {
		body = dotGray + "  " + titleStyle.Render(title) + "\n\n" +
			row("Status", "unknown — waiting for MCP…")
	} else {
		switch status.Status {
		case "indexed":
			ts := status.LastIndexedTime()
			if len(ts) > 19 {
				ts = ts[:19]
			}
			ts = strings.ReplaceAll(ts, "T", " ")
			body = dotGreen + "  " + titleStyle.Render(title) + "\n\n" +
				row("Status", "Indexed") + "\n" +
				row("Collection", status.CollectionName) + "\n" +
				row("Chunks", fmt.Sprintf("%d", status.ChunksCount)) + "\n" +
				row("Last indexed", ts)
		case "indexing":
			body = dotYellow + "  " + titleStyle.Render(title) + "\n\n" +
				row("Status", "Indexing in progress…") + "\n" +
				row("Chunks so far", fmt.Sprintf("%d", status.ChunksCount))
		default:
			body = dotGray + "  " + titleStyle.Render(title) + "\n\n" +
				row("Status", "Not indexed yet")
		}
	}

	card := cardStyle.Render(body)
	hint_ := hintStyle.Render("↵  " + hint)
	return card + "\n\n" + hint_
}

// ── Prompts Support ───────────────────────────────────────────────────────────

// promptsResultMsg carries the list of prompts fetched from the MCP server.
type promptsResultMsg struct{ prompts []mcp.PromptInfo }

// fetchPromptsCmd asks the MCP server for its list of prompts.
func (m *ChatModel) fetchPromptsCmd() tea.Cmd {
	return func() tea.Msg {
		if m.mcpMgr == nil {
			return nil
		}
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		prompts, _ := m.mcpMgr.ListPrompts(ctx)
		return promptsResultMsg{prompts: prompts}
	}
}

// filteredPrompts returns the subset of m.prompts whose name or description
// contains the current promptsFilter (case-insensitive).
func (m *ChatModel) filteredPrompts() []mcp.PromptInfo {
	if m.promptsFilter == "" {
		return m.prompts
	}
	filter := strings.ToLower(m.promptsFilter)
	var out []mcp.PromptInfo
	for _, p := range m.prompts {
		if strings.Contains(strings.ToLower(p.Name), filter) ||
			strings.Contains(strings.ToLower(p.Description), filter) {
			out = append(out, p)
		}
	}
	return out
}

// renderPromptsOverlay renders the prompts picker as a styled overlay.
func (m *ChatModel) renderPromptsOverlay() string {
	filtered := m.filteredPrompts()

	headerStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("141"))
	selectedStyle := lipgloss.NewStyle().
		Background(lipgloss.Color("236")).
		Foreground(lipgloss.Color("39")).
		PaddingLeft(1).PaddingRight(1)
	normalStyle := lipgloss.NewStyle().
		Foreground(colorText).
		PaddingLeft(1).PaddingRight(1)
	descStyle := lipgloss.NewStyle().Foreground(colorMuted).Italic(true)
	filterStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("220"))
	hintStyle := lipgloss.NewStyle().Foreground(colorMuted).Italic(true)
	boxStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("141")).
		Padding(1, 2).
		Width(64)

	var sb strings.Builder
	sb.WriteString(headerStyle.Render("✦ Prompts") + "\n\n")

	filterLine := filterStyle.Render("Filter: ") + m.promptsFilter + "█"
	sb.WriteString(filterLine + "\n\n")

	if len(filtered) == 0 {
		sb.WriteString(descStyle.Render("No prompts match.") + "\n")
	} else {
		for i, p := range filtered {
			name := p.Name
			desc := p.Description
			if len(desc) > 48 {
				desc = desc[:48] + "…"
			}
			line := name
			if desc != "" {
				line += "  " + descStyle.Render(desc)
			}
			if i == m.promptsCursor {
				sb.WriteString(selectedStyle.Render("> "+name) + "  " + descStyle.Render(desc) + "\n")
			} else {
				sb.WriteString(normalStyle.Render("  "+line) + "\n")
			}
		}
	}

	sb.WriteString("\n" + hintStyle.Render("↑↓ navigate  Enter to use  Esc to close"))
	return boxStyle.Render(sb.String())
}

// executePromptCmd parses a "/prompt <name> [key=value ...]" command, fetches
// the expanded prompt from the server, and injects the result as a user message
// that is then sent through the normal RAG pipeline.
func (m *ChatModel) executePromptCmd(raw string) tea.Cmd {
	return func() tea.Msg {
		if m.mcpMgr == nil {
			return errorMsg{fmt.Errorf("MCP Manager not initialized")}
		}

		// Parse: /prompt <name> [key=value ...]
		parts := strings.Fields(strings.TrimPrefix(raw, "/prompt "))
		if len(parts) == 0 {
			return errorMsg{fmt.Errorf("usage: /prompt <name> [key=value ...]")}
		}
		name := parts[0]
		args := make(map[string]string)
		for _, part := range parts[1:] {
			kv := strings.SplitN(part, "=", 2)
			if len(kv) == 2 {
				args[kv[0]] = kv[1]
			}
		}

		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()

		text, err := m.mcpMgr.GetPrompt(ctx, name, args)
		if err != nil {
			return errorMsg{fmt.Errorf("prompt %q: %w", name, err)}
		}
		if text == "" {
			return errorMsg{fmt.Errorf("prompt %q returned empty content", name)}
		}

		// Treat the expanded prompt as a user question fed into the RAG pipeline.
		return searchResultMsg{content: text, tabIndex: TabIndexChat}
	}
}

// handlePromptsKey processes keyboard input when the prompts picker is open.
// Returns the updated commands slice. Closes the picker and injects the prompt
// template into the chat input if Enter is pressed on a valid selection.
func (m *ChatModel) handlePromptsKey(msg tea.KeyMsg) []tea.Cmd {
	filtered := m.filteredPrompts()
	switch msg.Type {
	case tea.KeyEsc:
		m.showPrompts = false
		m.promptsFilter = ""
		m.promptsCursor = 0

	case tea.KeyUp:
		if m.promptsCursor > 0 {
			m.promptsCursor--
		}

	case tea.KeyDown:
		if m.promptsCursor < len(filtered)-1 {
			m.promptsCursor++
		}

	case tea.KeyEnter:
		if len(filtered) > 0 && m.promptsCursor < len(filtered) {
			chosen := filtered[m.promptsCursor]
			m.showPrompts = false
			m.promptsFilter = ""
			m.promptsCursor = 0
			// Pre-fill the chat input with the prompt name so user can add args
			m.input.SetValue("/prompt " + chosen.Name + " ")
			m.input.CursorEnd()
		}

	case tea.KeyBackspace, tea.KeyDelete:
		if len(m.promptsFilter) > 0 {
			m.promptsFilter = m.promptsFilter[:len(m.promptsFilter)-1]
			m.promptsCursor = 0
		}

	case tea.KeyRunes:
		m.promptsFilter += string(msg.Runes)
		m.promptsCursor = 0
	}
	return nil
}
