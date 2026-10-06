// ABOUTME: Main Bubble Tea application model
// ABOUTME: Coordinates three-pane layout and navigation

package tui

import (
	"fmt"
	"slices"
	"unicode"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/harper/bbs/internal/models"
	"github.com/harper/bbs/internal/storage"
)

// Pane represents which pane is focused
type Pane int

const (
	TopicsPane Pane = iota
	ThreadsPane
	MessagesPane
)

type threadsLoadedForSelectionMsg struct {
	requestID uint64
	topicID   models.UUID
	threads   []*models.Thread
}

type threadsLoadFailedMsg struct {
	requestID uint64
	topicID   models.UUID
	err       error
}

type messagesLoadedForSelectionMsg struct {
	requestID uint64
	threadID  models.UUID
	messages  []*models.Message
}

type messagesLoadFailedMsg struct {
	requestID uint64
	threadID  models.UUID
	err       error
}

type replyPersistedMsg struct {
	requestID uint64
	threadID  models.UUID
	topicID   models.UUID
}

type replyPersistenceFailedMsg struct {
	requestID uint64
	err       error
}

type stateRefreshLoadedMsg struct {
	requestID uint64
	threadID  models.UUID
	threads   []*models.Thread
	messages  []*models.Message
}

type stateRefreshFailedMsg struct {
	requestID uint64
	threadID  models.UUID
	posted    bool
	err       error
}

// Model is the main application state
type Model struct {
	store            storage.Storage
	identity         string
	activePane       Pane
	width            int
	height           int
	topics           TopicsModel
	threads          ThreadsModel
	messages         MessagesModel
	composing        bool
	composeText      string
	posting          bool
	nextRequest      uint64
	threadLoad       uint64
	messageLoad      uint64
	replyPost        uint64
	selectedThreadID models.UUID
	loadedThreadID   models.UUID
	threadsLoading   bool
	messagesLoading  bool
	err              error
}

// NewModel creates a new TUI model
func NewModel(store storage.Storage, identity string) Model {
	return Model{
		store:      store,
		identity:   identity,
		activePane: TopicsPane,
		topics:     NewTopicsModel(store),
		threads:    NewThreadsModel(),
		messages:   NewMessagesModel(),
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
		if msg.Type == tea.KeyCtrlC {
			return m, tea.Quit
		}
		if m.posting {
			return m, nil
		}
		if m.composing {
			return m.updateCompose(msg)
		}
		return m.updateNavigation(msg)

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil

	case TopicsLoadedMsg:
		return m.applyTopics(msg), nil

	case threadsLoadedForSelectionMsg:
		return m.applySelectedThreads(msg), nil

	case threadsLoadFailedMsg:
		return m.applyThreadsLoadFailure(msg), nil

	case messagesLoadedForSelectionMsg:
		return m.applySelectedMessages(msg), nil

	case messagesLoadFailedMsg:
		return m.applyMessagesLoadFailure(msg), nil

	case replyPersistedMsg:
		return m.applyReplyPersisted(msg)

	case replyPersistenceFailedMsg:
		return m.applyReplyPersistenceFailure(msg), nil

	case stateRefreshLoadedMsg:
		return m.applyStateRefresh(msg), nil

	case stateRefreshFailedMsg:
		return m.applyStateRefreshFailure(msg), nil

	case error:
		m.err = msg
		return m, nil
	}

	return m, nil
}

func (m Model) applyTopics(msg TopicsLoadedMsg) Model {
	m.topics.SetTopics(msg.Topics)
	selectedTopicStillListed := slices.ContainsFunc(msg.Topics, func(topic *models.Topic) bool {
		return topic.ID == m.threads.topicID
	})
	if m.threads.topicID == (models.UUID{}) || selectedTopicStillListed {
		return m
	}

	requestID := m.newRequestID()
	m.threadLoad = requestID
	m.messageLoad = requestID
	m.threads.topicID = models.UUID{}
	m.threads.SetThreads(nil)
	m.messages.threadID = models.UUID{}
	m.messages.SetMessages(nil)
	m.selectedThreadID = models.UUID{}
	m.loadedThreadID = models.UUID{}
	m.threadsLoading = false
	m.messagesLoading = false
	m.composing = false
	m.composeText = ""
	m.posting = false
	m.replyPost = 0
	m.err = nil
	return m
}

func (m Model) applySelectedThreads(msg threadsLoadedForSelectionMsg) Model {
	if m.isCurrentThreadLoad(msg.requestID, msg.topicID) {
		m.threads.SetThreads(msg.threads)
		m.threadsLoading = false
		m.err = nil
	}
	return m
}

func (m Model) applyThreadsLoadFailure(msg threadsLoadFailedMsg) Model {
	if m.isCurrentThreadLoad(msg.requestID, msg.topicID) {
		m.threadsLoading = false
		m.err = msg.err
	}
	return m
}

func (m Model) applySelectedMessages(msg messagesLoadedForSelectionMsg) Model {
	if m.isCurrentMessageLoad(msg.requestID, msg.threadID) {
		m.messages.SetMessages(msg.messages)
		m.messages.threadID = msg.threadID
		m.loadedThreadID = msg.threadID
		m.messagesLoading = false
		m.err = nil
	}
	return m
}

func (m Model) applyMessagesLoadFailure(msg messagesLoadFailedMsg) Model {
	if m.isCurrentMessageLoad(msg.requestID, msg.threadID) {
		m.loadedThreadID = models.UUID{}
		m.messagesLoading = false
		m.err = msg.err
	}
	return m
}

func (m Model) applyReplyPersisted(msg replyPersistedMsg) (Model, tea.Cmd) {
	if !m.isCurrentReplyPost(msg.requestID) {
		return m, nil
	}
	m.posting = false
	m.composing = false
	m.composeText = ""
	m.err = nil
	return m, m.beginStateRefresh(msg.threadID, msg.topicID, true)
}

func (m Model) applyReplyPersistenceFailure(msg replyPersistenceFailedMsg) Model {
	if m.isCurrentReplyPost(msg.requestID) {
		m.posting = false
		m.err = msg.err
	}
	return m
}

func (m Model) applyStateRefresh(msg stateRefreshLoadedMsg) Model {
	if m.isCurrentStateRefresh(msg.requestID, msg.threadID) {
		m.setRefreshedState(msg.threadID, msg.threads, msg.messages)
		m.err = nil
	}
	return m
}

func (m Model) applyStateRefreshFailure(msg stateRefreshFailedMsg) Model {
	if !m.isCurrentStateRefresh(msg.requestID, msg.threadID) {
		return m
	}
	m.threadsLoading = false
	m.messagesLoading = false
	if msg.posted {
		m.err = fmt.Errorf("reply posted, refresh failed: %w", msg.err)
	} else {
		m.err = fmt.Errorf("refresh failed: %w", msg.err)
	}
	return m
}

func (m Model) isCurrentThreadLoad(requestID uint64, topicID models.UUID) bool {
	return requestID == m.threadLoad && topicID == m.threads.topicID
}

func (m Model) isCurrentMessageLoad(requestID uint64, threadID models.UUID) bool {
	return requestID == m.messageLoad && threadID == m.selectedThreadID
}

func (m Model) isCurrentReplyPost(requestID uint64) bool {
	return requestID == m.replyPost && m.posting
}

func (m Model) isCurrentStateRefresh(requestID uint64, threadID models.UUID) bool {
	if requestID != m.threadLoad {
		return false
	}
	if requestID != m.messageLoad {
		return false
	}
	return threadID == m.selectedThreadID
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
				return m, m.beginThreadLoad(topic.ID)
			}
		case ThreadsPane:
			if thread := m.threads.Selected(); thread != nil {
				m.activePane = MessagesPane
				return m, m.beginMessageLoad(thread.ID)
			}
		}
		return m, nil

	case "n":
		if m.canReply() {
			m.composing = true
			m.composeText = ""
			m.err = nil
		}
		return m, nil

	case "r":
		switch m.activePane {
		case ThreadsPane:
			if m.threads.topicID != (models.UUID{}) {
				return m, m.beginThreadLoad(m.threads.topicID)
			}
		case MessagesPane:
			if thread := m.threads.Selected(); thread != nil && thread.ID == m.selectedThreadID {
				return m, m.beginStateRefresh(thread.ID, thread.TopicID, false)
			}
			if m.selectedThreadID != (models.UUID{}) && m.threads.topicID != (models.UUID{}) {
				return m, m.beginStateRefresh(m.selectedThreadID, m.threads.topicID, false)
			}
		}
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
		if m.composeText == "" || !m.canReply() {
			return m, nil
		}
		return m, m.beginReplyPost()
	case "backspace":
		runes := []rune(m.composeText)
		if len(runes) > 0 {
			m.composeText = string(runes[:len(runes)-1])
		}
		return m, nil
	case " ":
		m.composeText += " "
		return m, nil
	default:
		for _, r := range msg.Runes {
			if unicode.IsPrint(r) {
				m.composeText += string(r)
			}
		}
		return m, nil
	}
}

func (m Model) canReply() bool {
	thread := m.threads.Selected()
	return m.activePane == MessagesPane && !m.posting && thread != nil && thread.ID == m.selectedThreadID && m.loadedThreadID == m.selectedThreadID
}

func (m *Model) beginReplyPost() tea.Cmd {
	thread := m.threads.Selected()
	reply := models.NewMessage(thread.ID, m.composeText, m.identity)
	requestID := m.newRequestID()
	m.replyPost = requestID
	m.posting = true
	store := m.store
	return func() tea.Msg {
		if err := store.CreateMessage(reply); err != nil {
			return replyPersistenceFailedMsg{requestID: requestID, err: err}
		}
		return replyPersistedMsg{requestID: requestID, threadID: thread.ID, topicID: thread.TopicID}
	}
}

func (m *Model) newRequestID() uint64 {
	m.nextRequest++
	return m.nextRequest
}

func (m *Model) beginThreadLoad(topicID models.UUID) tea.Cmd {
	requestID := m.newRequestID()
	m.threadLoad = requestID
	m.messageLoad = requestID
	m.threads.topicID = topicID
	m.threads.SetThreads(nil)
	m.messages.SetMessages(nil)
	m.selectedThreadID = models.UUID{}
	m.loadedThreadID = models.UUID{}
	m.messages.threadID = models.UUID{}
	m.threadsLoading = true
	m.messagesLoading = false
	m.err = nil
	store := m.store
	return func() tea.Msg {
		threads, err := store.ListThreads(topicID)
		if err != nil {
			return threadsLoadFailedMsg{requestID: requestID, topicID: topicID, err: err}
		}
		return threadsLoadedForSelectionMsg{requestID: requestID, topicID: topicID, threads: threads}
	}
}

func (m *Model) beginMessageLoad(threadID models.UUID) tea.Cmd {
	requestID := m.newRequestID()
	m.messageLoad = requestID
	m.selectedThreadID = threadID
	m.loadedThreadID = models.UUID{}
	m.messages.SetMessages(nil)
	m.messages.threadID = threadID
	m.messagesLoading = true
	m.err = nil
	store := m.store
	return func() tea.Msg {
		messages, err := store.ListMessages(threadID)
		if err != nil {
			return messagesLoadFailedMsg{requestID: requestID, threadID: threadID, err: err}
		}
		return messagesLoadedForSelectionMsg{requestID: requestID, threadID: threadID, messages: messages}
	}
}

func (m *Model) beginStateRefresh(threadID, topicID models.UUID, posted bool) tea.Cmd {
	requestID := m.newRequestID()
	m.threadLoad = requestID
	m.messageLoad = requestID
	m.threads.topicID = topicID
	m.threads.SetThreads(nil)
	m.messages.SetMessages(nil)
	m.messages.threadID = threadID
	m.loadedThreadID = models.UUID{}
	m.threadsLoading = true
	m.messagesLoading = true
	m.err = nil
	store := m.store
	return func() tea.Msg {
		messages, err := store.ListMessages(threadID)
		if err != nil {
			return stateRefreshFailedMsg{requestID: requestID, threadID: threadID, posted: posted, err: err}
		}
		threads, err := store.ListThreads(topicID)
		if err != nil {
			return stateRefreshFailedMsg{requestID: requestID, threadID: threadID, posted: posted, err: err}
		}
		return stateRefreshLoadedMsg{requestID: requestID, threadID: threadID, threads: threads, messages: messages}
	}
}

func (m *Model) setRefreshedState(threadID models.UUID, threads []*models.Thread, messages []*models.Message) {
	m.threads.SetThreads(threads)
	selectionFound := false
	for i, thread := range m.threads.threads {
		if thread.ID == threadID {
			m.threads.cursor = i
			selectionFound = true
			break
		}
	}
	m.threadsLoading = false
	m.messagesLoading = false
	if !selectionFound {
		m.messages.SetMessages(nil)
		m.selectedThreadID = models.UUID{}
		m.loadedThreadID = models.UUID{}
		m.messages.threadID = models.UUID{}
		return
	}
	m.messages.SetMessages(messages)
	m.messages.threadID = threadID
	m.loadedThreadID = threadID
	if len(messages) > 0 {
		m.messages.scroll = len(messages) - 1
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

	threadsContent := m.threads.View()
	if m.threadsLoading {
		threadsContent = "Loading threads..."
	}
	messagesContent := m.messages.View()
	if m.messagesLoading {
		messagesContent = "Loading messages..."
	}
	topicsView := topicsStyle.Width(topicsWidth - 2).Height(m.height - 4).Render(m.topics.View())
	threadsView := threadsStyle.Width(threadsWidth - 2).Height(m.height - 4).Render(threadsContent)
	messagesView := messagesStyle.Width(messagesWidth - 2).Height(m.height - 4).Render(messagesContent)

	main := lipgloss.JoinHorizontal(lipgloss.Top, topicsView, threadsView, messagesView)

	// Status bar
	help := "[tab] switch pane  [j/k] navigate  [enter] select  [r] refresh  [q] quit"
	if m.canReply() {
		help = "[tab] switch pane  [j/k] navigate  [n] reply  [r] refresh  [q] quit"
	}
	status := lipgloss.NewStyle().Foreground(lipgloss.Color("241")).Render(help)

	if m.composing {
		status = lipgloss.NewStyle().
			Foreground(lipgloss.Color("86")).
			Render("Composing: " + m.composeText + "_  [enter] submit  [esc] cancel")
	}
	if m.err != nil {
		errorStatus := lipgloss.NewStyle().
			Foreground(lipgloss.Color("196")).
			Render("Error: " + m.err.Error())
		status = lipgloss.JoinVertical(lipgloss.Left, errorStatus, status)
	}

	return lipgloss.JoinVertical(lipgloss.Left, main, status)
}

// Run starts the TUI
func Run(store storage.Storage, identity string) error {
	p := tea.NewProgram(NewModel(store, identity), tea.WithAltScreen())
	_, err := p.Run()
	return err
}
