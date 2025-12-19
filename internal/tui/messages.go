// ABOUTME: Messages pane component
// ABOUTME: Displays messages in a thread

package tui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/google/uuid"
	"github.com/harper/bbs/internal/charm"
	"github.com/harper/bbs/internal/models"
)

type MessagesLoadedMsg struct {
	Messages []*models.Message
}

type MessagesModel struct {
	client   *charm.Client
	messages []*models.Message
	cursor   int
	scroll   int
	threadID uuid.UUID
}

func NewMessagesModel(client *charm.Client) MessagesModel {
	return MessagesModel{client: client, cursor: 0, scroll: 0}
}

func (m *MessagesModel) LoadMessages(threadID uuid.UUID) tea.Cmd {
	m.threadID = threadID
	return func() tea.Msg {
		messages, err := m.client.ListMessages(threadID)
		if err != nil {
			return err
		}
		return MessagesLoadedMsg{Messages: messages}
	}
}

func (m *MessagesModel) SetMessages(messages []*models.Message) {
	m.messages = messages
	m.cursor = 0
	m.scroll = 0
}

func (m *MessagesModel) MoveUp() {
	if m.scroll > 0 {
		m.scroll--
	}
}

func (m *MessagesModel) MoveDown() {
	if m.scroll < len(m.messages)-1 {
		m.scroll++
	}
}

func (m *MessagesModel) Selected() *models.Message {
	if m.cursor >= 0 && m.cursor < len(m.messages) {
		return m.messages[m.cursor]
	}
	return nil
}

func (m MessagesModel) View() string {
	if len(m.messages) == 0 {
		return lipgloss.NewStyle().Faint(true).Render("No messages\n\nSelect a thread")
	}

	var s string
	s += lipgloss.NewStyle().Bold(true).Render("Messages") + "\n\n"

	headerStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("86"))
	faintStyle := lipgloss.NewStyle().Faint(true)

	for i, msg := range m.messages {
		if i < m.scroll {
			continue
		}

		// Header
		edited := ""
		if msg.EditedAt != nil {
			edited = " (edited)"
		}
		s += headerStyle.Render(msg.CreatedBy)
		s += faintStyle.Render(fmt.Sprintf(" · %s%s\n", msg.CreatedAt.Format("Jan 02 15:04"), edited))

		// Content (truncate long messages)
		content := msg.Content
		if len(content) > 200 {
			content = content[:200] + "..."
		}
		// Wrap content
		lines := strings.Split(content, "\n")
		for _, line := range lines {
			s += line + "\n"
		}
		s += "\n"
	}

	return s
}
