// ABOUTME: Threads pane component
// ABOUTME: Lists and navigates threads in a topic

package tui

import (
	"fmt"

	"github.com/charmbracelet/lipgloss"
	"github.com/google/uuid"
	"github.com/harper/bbs/internal/models"
)

type ThreadsModel struct {
	threads []*models.Thread
	cursor  int
	topicID uuid.UUID
}

func NewThreadsModel() ThreadsModel {
	return ThreadsModel{cursor: 0}
}

func (m *ThreadsModel) SetThreads(threads []*models.Thread) {
	m.threads = threads
	m.cursor = 0
}

func (m *ThreadsModel) MoveUp() {
	if m.cursor > 0 {
		m.cursor--
	}
}

func (m *ThreadsModel) MoveDown() {
	if m.cursor < len(m.threads)-1 {
		m.cursor++
	}
}

func (m *ThreadsModel) Selected() *models.Thread {
	if m.cursor >= 0 && m.cursor < len(m.threads) {
		return m.threads[m.cursor]
	}
	return nil
}

func (m ThreadsModel) View() string {
	if len(m.threads) == 0 {
		return lipgloss.NewStyle().Faint(true).Render("No threads\n\nSelect a topic")
	}

	var s string
	s += lipgloss.NewStyle().Bold(true).Render("Threads") + "\n\n"

	for i, thread := range m.threads {
		cursor := "  "
		style := lipgloss.NewStyle()

		if i == m.cursor {
			cursor = "> "
			style = style.Foreground(lipgloss.Color("86"))
		}

		prefix := ""
		if thread.Sticky {
			prefix = "[PIN] "
		}

		s += fmt.Sprintf("%s%s%s\n", cursor, prefix, style.Render(thread.Subject))
		s += lipgloss.NewStyle().Faint(true).Render(fmt.Sprintf("   %s\n", thread.CreatedBy))
	}

	return s
}
