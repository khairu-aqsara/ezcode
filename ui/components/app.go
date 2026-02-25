package components

import (
	tea "github.com/charmbracelet/bubbletea"
	"github.com/khairu-aqsara/ezcode/internal/config"
)

// AppModel is the top-level Bubble Tea model
type AppModel struct {
	chat *ChatModel
}

// NewApp creates a new top-level application model
func NewApp(cfg *config.Config, composePath string, projectPath string, isFirstRun bool) *AppModel {
	return &AppModel{
		chat: NewChat(cfg, composePath, projectPath, isFirstRun),
	}
}

// Init initializes the application
func (m *AppModel) Init() tea.Cmd {
	return m.chat.Init()
}

// Update handles messages and state transitions
func (m *AppModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if keyMsg, ok := msg.(tea.KeyMsg); ok {
		if keyMsg.Type == tea.KeyCtrlC {
			return m, tea.Quit
		}
	}

	chatModel, cmd := m.chat.Update(msg)
	if cM, ok := chatModel.(*ChatModel); ok {
		m.chat = cM
	}

	return m, cmd
}

// View outputs the UI strings
func (m *AppModel) View() string {
	return m.chat.View()
}

// WasDockerStarted reports whether this session launched Docker via compose up,
// so the caller can decide whether to run compose down on exit.
func (m *AppModel) WasDockerStarted() bool {
	return m.chat.dashboard.StartedDocker()
}
