// ABOUTME: Main Bubble Tea application model
// ABOUTME: Coordinates three-pane layout and navigation

package tui

import (
	"database/sql"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// Pane represents which pane is focused
type Pane int

const (
	TopicsPane Pane = iota
	ThreadsPane
	MessagesPane
)

// Model is the main application state
type Model struct {
	db          *sql.DB
	identity    string
	activePane  Pane
	width       int
	height      int
	topics      TopicsModel
	threads     ThreadsModel
	messages    MessagesModel
	composing   bool
	composeText string
	err         error
}

// NewModel creates a new TUI model
func NewModel(db *sql.DB, identity string) Model {
	return Model{
		db:         db,
		identity:   identity,
		activePane: TopicsPane,
		topics:     NewTopicsModel(db),
		threads:    NewThreadsModel(db),
		messages:   NewMessagesModel(db),
	}
}

// Init initializes the model
func (m Model) Init() tea.Cmd {
	return m.topics.LoadTopics()
}

// Update handles messages
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		if m.composing {
			return m.updateCompose(msg)
		}
		return m.updateNavigation(msg)

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil

	case TopicsLoadedMsg:
		m.topics.SetTopics(msg.Topics)
		return m, nil

	case ThreadsLoadedMsg:
		m.threads.SetThreads(msg.Threads)
		return m, nil

	case MessagesLoadedMsg:
		m.messages.SetMessages(msg.Messages)
		return m, nil

	case error:
		m.err = msg
		return m, nil
	}

	return m, nil
}

func (m Model) updateNavigation(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "q", "ctrl+c":
		return m, tea.Quit

	case "tab":
		m.activePane = (m.activePane + 1) % 3
		return m, nil

	case "shift+tab":
		m.activePane = (m.activePane + 2) % 3
		return m, nil

	case "j", "down":
		switch m.activePane {
		case TopicsPane:
			m.topics.MoveDown()
		case ThreadsPane:
			m.threads.MoveDown()
		case MessagesPane:
			m.messages.MoveDown()
		}
		return m, nil

	case "k", "up":
		switch m.activePane {
		case TopicsPane:
			m.topics.MoveUp()
		case ThreadsPane:
			m.threads.MoveUp()
		case MessagesPane:
			m.messages.MoveUp()
		}
		return m, nil

	case "enter":
		switch m.activePane {
		case TopicsPane:
			if topic := m.topics.Selected(); topic != nil {
				m.activePane = ThreadsPane
				return m, m.threads.LoadThreads(topic.ID.String())
			}
		case ThreadsPane:
			if thread := m.threads.Selected(); thread != nil {
				m.activePane = MessagesPane
				return m, m.messages.LoadMessages(thread.ID.String())
			}
		}
		return m, nil

	case "n":
		m.composing = true
		m.composeText = ""
		return m, nil

	case "r":
		return m, m.topics.LoadTopics()
	}

	return m, nil
}

func (m Model) updateCompose(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.composing = false
		return m, nil
	case "enter":
		// Submit compose
		m.composing = false
		return m, nil
	case "backspace":
		if len(m.composeText) > 0 {
			m.composeText = m.composeText[:len(m.composeText)-1]
		}
		return m, nil
	default:
		if len(msg.String()) == 1 {
			m.composeText += msg.String()
		}
		return m, nil
	}
}

// View renders the UI
func (m Model) View() string {
	if m.width == 0 {
		return "Loading..."
	}

	// Calculate pane widths
	topicsWidth := m.width / 4
	threadsWidth := m.width / 4
	messagesWidth := m.width - topicsWidth - threadsWidth

	// Styles
	activeStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("86"))

	inactiveStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("240"))

	// Render panes
	topicsStyle := inactiveStyle
	threadsStyle := inactiveStyle
	messagesStyle := inactiveStyle

	switch m.activePane {
	case TopicsPane:
		topicsStyle = activeStyle
	case ThreadsPane:
		threadsStyle = activeStyle
	case MessagesPane:
		messagesStyle = activeStyle
	}

	topicsView := topicsStyle.Width(topicsWidth - 2).Height(m.height - 4).Render(m.topics.View())
	threadsView := threadsStyle.Width(threadsWidth - 2).Height(m.height - 4).Render(m.threads.View())
	messagesView := messagesStyle.Width(messagesWidth - 2).Height(m.height - 4).Render(m.messages.View())

	main := lipgloss.JoinHorizontal(lipgloss.Top, topicsView, threadsView, messagesView)

	// Status bar
	status := lipgloss.NewStyle().
		Foreground(lipgloss.Color("241")).
		Render("[tab] switch pane  [j/k] navigate  [enter] select  [n] new  [r] refresh  [q] quit")

	if m.composing {
		status = lipgloss.NewStyle().
			Foreground(lipgloss.Color("86")).
			Render("Composing: " + m.composeText + "█  [enter] submit  [esc] cancel")
	}

	return lipgloss.JoinVertical(lipgloss.Left, main, status)
}

// Run starts the TUI
func Run(db *sql.DB, identity string) error {
	p := tea.NewProgram(NewModel(db, identity), tea.WithAltScreen())
	_, err := p.Run()
	return err
}
