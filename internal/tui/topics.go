// ABOUTME: Topics pane component
// ABOUTME: Lists and navigates topics

package tui

import (
	"database/sql"
	"fmt"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/harper/bbs/internal/db"
	"github.com/harper/bbs/internal/models"
)

type TopicsLoadedMsg struct {
	Topics []*models.Topic
}

type TopicsModel struct {
	db       *sql.DB
	topics   []*models.Topic
	cursor   int
	selected int
}

func NewTopicsModel(database *sql.DB) TopicsModel {
	return TopicsModel{db: database, cursor: 0, selected: -1}
}

func (m *TopicsModel) LoadTopics() tea.Cmd {
	return func() tea.Msg {
		topics, err := db.ListTopics(m.db, false)
		if err != nil {
			return err
		}
		return TopicsLoadedMsg{Topics: topics}
	}
}

func (m *TopicsModel) SetTopics(topics []*models.Topic) {
	m.topics = topics
	if m.cursor >= len(topics) {
		m.cursor = len(topics) - 1
	}
	if m.cursor < 0 {
		m.cursor = 0
	}
}

func (m *TopicsModel) MoveUp() {
	if m.cursor > 0 {
		m.cursor--
	}
}

func (m *TopicsModel) MoveDown() {
	if m.cursor < len(m.topics)-1 {
		m.cursor++
	}
}

func (m *TopicsModel) Selected() *models.Topic {
	if m.cursor >= 0 && m.cursor < len(m.topics) {
		return m.topics[m.cursor]
	}
	return nil
}

func (m TopicsModel) View() string {
	if len(m.topics) == 0 {
		return lipgloss.NewStyle().Faint(true).Render("No topics")
	}

	var s string
	s += lipgloss.NewStyle().Bold(true).Render("Topics") + "\n\n"

	for i, topic := range m.topics {
		cursor := "  "
		style := lipgloss.NewStyle()

		if i == m.cursor {
			cursor = "> "
			style = style.Foreground(lipgloss.Color("86"))
		}

		archived := ""
		if topic.Archived {
			archived = " (archived)"
			style = style.Faint(true)
		}

		s += fmt.Sprintf("%s%s%s\n", cursor, style.Render(topic.Name), archived)
	}

	return s
}
