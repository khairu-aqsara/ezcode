package components

import (
	"errors"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/khairu-aqsara/ezcode/internal/config"
)

// ConfigForm represents the first-run configuration UI
type ConfigForm struct {
	inputs     []textinput.Model
	focusIndex int
	cfg        *config.Config
	showCancel bool
}

// NewConfigForm initializes a new configuration form.
// Index 0–10: env vars (unchanged).
// Index 11:   server_mode  ("docker" | "stdio").
// Index 12:   server_path  (path to build/index.js, stdio only).
func NewConfigForm(cfg *config.Config, showCancel bool) *ConfigForm {
	inputs := make([]textinput.Model, 13)

	var t textinput.Model
	for i := range inputs {
		t = textinput.New()
		t.CharLimit = 256

		switch i {
		case 0:
			t.Placeholder = "OpenAI API Key"
			t.SetValue(cfg.Env["OPENAI_API_KEY"])
			t.Focus()
			t.PromptStyle = focusedStyle
			t.TextStyle = focusedStyle
		case 1:
			t.Placeholder = "Qdrant API Key (Optional)"
			t.SetValue(cfg.Env["QDRANT_API_KEY"])
		case 2:
			t.Placeholder = "Embedding Model (e.g. models/gemini-embedding-001)"
			t.SetValue(cfg.Env["EMBEDDING_MODEL"])
		case 3:
			t.Placeholder = "LLM Model (e.g. gemini-2.5-flash)"
			t.SetValue(cfg.Env["LLM_MODEL"])
		case 4:
			t.Placeholder = "Transport Mode (e.g. http)"
			t.SetValue(cfg.Env["TRANSPORT_MODE"])
		case 5:
			t.Placeholder = "Embedding Provider (e.g. openai)"
			t.SetValue(cfg.Env["EMBEDDING_PROVIDER"])
		case 6:
			t.Placeholder = "OpenAI Base URL (e.g. https://generativelanguage.googleapis.com/v1beta/openai)"
			t.SetValue(cfg.Env["OPENAI_BASE_URL"])
		case 7:
			t.Placeholder = "Log Level (e.g. info)"
			t.SetValue(cfg.Env["LOG_LEVEL"])
		case 8:
			t.Placeholder = "Qdrant URL (e.g. http://localhost:6333)"
			t.SetValue(cfg.Env["QDRANT_URL"])
		case 9:
			t.Placeholder = "HTTP Port (e.g. 3000)"
			t.SetValue(cfg.Env["HTTP_PORT"])
		case 10:
			t.Placeholder = "Embedding Dimensions (e.g. 3072)"
			t.SetValue(cfg.Env["EMBEDDING_DIMENSIONS"])
		case 11:
			t.Placeholder = "Server Mode: docker (default) or stdio"
			t.SetValue(string(cfg.ServerMode))
		case 12:
			t.Placeholder = "Server Path (stdio only): /path/to/qdrant-mcp-server/build/index.js"
			t.SetValue(cfg.ServerPath)
		}

		inputs[i] = t
	}

	return &ConfigForm{
		inputs:     inputs,
		cfg:        cfg,
		showCancel: showCancel,
	}
}

// Validate checks if the form inputs are correct
func (f *ConfigForm) Validate() error {
	if f.inputs[0].Value() == "" {
		return errors.New("OpenAI API Key is required")
	}
	return nil
}

// SaveToConfig writes the form inputs back to the configuration struct
func (f *ConfigForm) SaveToConfig() {
	f.cfg.Env["OPENAI_API_KEY"] = f.inputs[0].Value()
	f.cfg.Env["QDRANT_API_KEY"] = f.inputs[1].Value()
	f.cfg.Env["EMBEDDING_MODEL"] = f.inputs[2].Value()
	f.cfg.Env["LLM_MODEL"] = f.inputs[3].Value()
	f.cfg.Env["TRANSPORT_MODE"] = f.inputs[4].Value()
	f.cfg.Env["EMBEDDING_PROVIDER"] = f.inputs[5].Value()
	f.cfg.Env["OPENAI_BASE_URL"] = f.inputs[6].Value()
	f.cfg.Env["LOG_LEVEL"] = f.inputs[7].Value()
	f.cfg.Env["QDRANT_URL"] = f.inputs[8].Value()
	f.cfg.Env["HTTP_PORT"] = f.inputs[9].Value()
	f.cfg.Env["EMBEDDING_DIMENSIONS"] = f.inputs[10].Value()

	// Server mode / path (top-level config fields, not env vars)
	mode := config.ServerMode(f.inputs[11].Value())
	if mode == config.ServerModeStdio {
		f.cfg.ServerMode = config.ServerModeStdio
	} else {
		f.cfg.ServerMode = config.ServerModeDocker
	}
	f.cfg.ServerPath = f.inputs[12].Value()
}

var focusedStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("205"))

// Init initialize the text inputs
func (f *ConfigForm) Init() tea.Cmd {
	return textinput.Blink
}

// Update handles msgs for the ConfigForm
func (f *ConfigForm) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.Type {
		case tea.KeyUp, tea.KeyShiftTab:
			if f.focusIndex > 0 {
				f.focusIndex--
			}
		case tea.KeyDown, tea.KeyTab:
			if f.focusIndex < len(f.inputs)-1 {
				f.focusIndex++
			}
		case tea.KeyEnter:
			// Form submission is handled by parent AppModel passing Enter check beforehand, but just in case
		}

		// Update focus
		for i := range f.inputs {
			if i == f.focusIndex {
				cmds = append(cmds, f.inputs[i].Focus())
				f.inputs[i].PromptStyle = focusedStyle
				f.inputs[i].TextStyle = focusedStyle
			} else {
				f.inputs[i].Blur()
				f.inputs[i].PromptStyle = lipgloss.NewStyle()
				f.inputs[i].TextStyle = lipgloss.NewStyle()
			}
		}
	}

	for i := range f.inputs {
		f.inputs[i], cmd = f.inputs[i].Update(msg)
		cmds = append(cmds, cmd)
	}

	return f, tea.Batch(cmds...)
}

// View renders the ConfigForm
func (f *ConfigForm) View() string {
	var b string
	for i := range f.inputs {
		b += f.inputs[i].View() + "\n"
	}
	if f.showCancel {
		b += "\n[Enter] Submit • [Up/Down/Tab] Navigate • [Esc] Cancel\n"
	} else {
		b += "\n[Enter] Submit • [Up/Down/Tab] Navigate\n"
	}
	return b
}
