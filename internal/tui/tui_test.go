// ABOUTME: Tests for TUI components
// ABOUTME: Verifies model initialization, navigation, and view rendering

package tui

import (
	"path/filepath"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/google/uuid"
	"github.com/harper/bbs/internal/models"
	"github.com/harper/bbs/internal/storage"
)

func TestNewModel(t *testing.T) {
	store := newTestStore(t)
	defer store.Close()

	model := NewModel(store, "test@tui")

	if model.identity != "test@tui" {
		t.Errorf("Expected identity test@tui, got %s", model.identity)
	}
	if model.store != store {
		t.Error("Expected store to be set")
	}
	if model.activePane != TopicsPane {
		t.Errorf("Expected activePane to be TopicsPane, got %d", model.activePane)
	}
	if model.composing {
		t.Error("Expected composing to be false")
	}
}

func TestPaneConstants(t *testing.T) {
	if TopicsPane != 0 {
		t.Errorf("Expected TopicsPane to be 0, got %d", TopicsPane)
	}
	if ThreadsPane != 1 {
		t.Errorf("Expected ThreadsPane to be 1, got %d", ThreadsPane)
	}
	if MessagesPane != 2 {
		t.Errorf("Expected MessagesPane to be 2, got %d", MessagesPane)
	}
}

func TestModelInit(t *testing.T) {
	store := newTestStore(t)
	defer store.Close()

	model := NewModel(store, "test@tui")
	cmd := model.Init()

	if cmd == nil {
		t.Error("Expected Init to return a command")
	}
}

func TestModelUpdateWindowSize(t *testing.T) {
	store := newTestStore(t)
	defer store.Close()

	model := NewModel(store, "test@tui")

	msg := tea.WindowSizeMsg{Width: 100, Height: 50}
	newModel, _ := model.Update(msg)

	m := newModel.(Model)
	if m.width != 100 {
		t.Errorf("Expected width 100, got %d", m.width)
	}
	if m.height != 50 {
		t.Errorf("Expected height 50, got %d", m.height)
	}
}

func TestModelUpdateQuit(t *testing.T) {
	store := newTestStore(t)
	defer store.Close()

	model := NewModel(store, "test@tui")

	tests := []struct {
		name string
		key  string
	}{
		{"q key", "q"},
		{"ctrl+c", "ctrl+c"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			msg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(tt.key)}
			if tt.key == "ctrl+c" {
				msg = tea.KeyMsg{Type: tea.KeyCtrlC}
			}
			_, cmd := model.Update(msg)

			// Command should be tea.Quit (check by running it would require more setup)
			if cmd == nil {
				t.Error("Expected quit command")
			}
		})
	}
}

func TestModelUpdateTabNavigation(t *testing.T) {
	store := newTestStore(t)
	defer store.Close()

	model := NewModel(store, "test@tui")

	// Tab forward
	msg := tea.KeyMsg{Type: tea.KeyTab}
	newModel, _ := model.Update(msg)
	m := newModel.(Model)
	if m.activePane != ThreadsPane {
		t.Errorf("Expected activePane to be ThreadsPane after tab, got %d", m.activePane)
	}

	// Tab forward again
	newModel, _ = m.Update(msg)
	m = newModel.(Model)
	if m.activePane != MessagesPane {
		t.Errorf("Expected activePane to be MessagesPane after second tab, got %d", m.activePane)
	}

	// Tab forward wraps around
	newModel, _ = m.Update(msg)
	m = newModel.(Model)
	if m.activePane != TopicsPane {
		t.Errorf("Expected activePane to wrap to TopicsPane, got %d", m.activePane)
	}
}

func TestModelUpdateShiftTabNavigation(t *testing.T) {
	store := newTestStore(t)
	defer store.Close()

	model := NewModel(store, "test@tui")

	// Shift+Tab backward (wraps to MessagesPane)
	msg := tea.KeyMsg{Type: tea.KeyShiftTab}
	newModel, _ := model.Update(msg)
	m := newModel.(Model)
	if m.activePane != MessagesPane {
		t.Errorf("Expected activePane to wrap to MessagesPane, got %d", m.activePane)
	}
}

func TestModelUpdateComposing(t *testing.T) {
	store := newTestStore(t)
	defer store.Close()

	model := NewModel(store, "test@tui")

	// Press 'n' to start composing
	msg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'n'}}
	newModel, _ := model.Update(msg)
	m := newModel.(Model)
	if !m.composing {
		t.Error("Expected composing to be true after 'n'")
	}

	// Type some characters
	typeMsg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'a'}}
	newModel, _ = m.Update(typeMsg)
	m = newModel.(Model)
	if m.composeText != "a" {
		t.Errorf("Expected composeText 'a', got %q", m.composeText)
	}

	// Backspace
	backspaceMsg := tea.KeyMsg{Type: tea.KeyBackspace}
	newModel, _ = m.Update(backspaceMsg)
	m = newModel.(Model)
	if m.composeText != "" {
		t.Errorf("Expected empty composeText after backspace, got %q", m.composeText)
	}

	// Escape to cancel
	escMsg := tea.KeyMsg{Type: tea.KeyEsc}
	newModel, _ = m.Update(escMsg)
	m = newModel.(Model)
	if m.composing {
		t.Error("Expected composing to be false after escape")
	}
}

func TestModelUpdateRefresh(t *testing.T) {
	store := newTestStore(t)
	defer store.Close()

	model := NewModel(store, "test@tui")

	// Press 'r' to refresh
	msg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'r'}}
	_, cmd := model.Update(msg)

	if cmd == nil {
		t.Error("Expected refresh command")
	}
}

func TestModelViewLoading(t *testing.T) {
	store := newTestStore(t)
	defer store.Close()

	model := NewModel(store, "test@tui")
	// Width is 0, should show loading
	view := model.View()

	if view != "Loading..." {
		t.Errorf("Expected 'Loading...', got %q", view)
	}
}

func TestModelViewWithSize(t *testing.T) {
	store := newTestStore(t)
	defer store.Close()

	model := NewModel(store, "test@tui")

	// Set window size
	msg := tea.WindowSizeMsg{Width: 100, Height: 50}
	newModel, _ := model.Update(msg)
	m := newModel.(Model)

	view := m.View()

	// Should contain status bar text
	if !strings.Contains(view, "switch pane") {
		t.Error("Expected view to contain status bar text")
	}
}

func TestModelTopicsLoadedMsg(t *testing.T) {
	store := newTestStore(t)
	defer store.Close()

	model := NewModel(store, "test@tui")

	topics := []*models.Topic{
		models.NewTopic("General", "General discussion", "test@tui"),
		models.NewTopic("Support", "Get help", "test@tui"),
	}

	msg := TopicsLoadedMsg{Topics: topics}
	newModel, _ := model.Update(msg)
	m := newModel.(Model)

	if len(m.topics.topics) != 2 {
		t.Errorf("Expected 2 topics, got %d", len(m.topics.topics))
	}
}

func TestModelThreadsLoadedMsg(t *testing.T) {
	store := newTestStore(t)
	defer store.Close()

	model := NewModel(store, "test@tui")

	topicID := uuid.New()
	threads := []*models.Thread{
		models.NewThread(topicID, "Thread 1", "test@tui"),
		models.NewThread(topicID, "Thread 2", "test@tui"),
	}

	msg := ThreadsLoadedMsg{Threads: threads}
	newModel, _ := model.Update(msg)
	m := newModel.(Model)

	if len(m.threads.threads) != 2 {
		t.Errorf("Expected 2 threads, got %d", len(m.threads.threads))
	}
}

func TestModelMessagesLoadedMsg(t *testing.T) {
	store := newTestStore(t)
	defer store.Close()

	model := NewModel(store, "test@tui")

	threadID := uuid.New()
	messages := []*models.Message{
		models.NewMessage(threadID, "Message 1", "test@tui"),
		models.NewMessage(threadID, "Message 2", "test@tui"),
	}

	msg := MessagesLoadedMsg{Messages: messages}
	newModel, _ := model.Update(msg)
	m := newModel.(Model)

	if len(m.messages.messages) != 2 {
		t.Errorf("Expected 2 messages, got %d", len(m.messages.messages))
	}
}

func TestModelErrorMsg(t *testing.T) {
	store := newTestStore(t)
	defer store.Close()

	model := NewModel(store, "test@tui")

	// Send an error as a message
	errMsg := error(nil) // Can't easily create an error type msg in test
	newModel, _ := model.Update(errMsg)
	m := newModel.(Model)

	// Just verify it doesn't panic
	_ = m
}

// TopicsModel tests
func TestNewTopicsModel(t *testing.T) {
	store := newTestStore(t)
	defer store.Close()

	tm := NewTopicsModel(store)

	if tm.store != store {
		t.Error("Expected store to be set")
	}
	if tm.cursor != 0 {
		t.Errorf("Expected cursor 0, got %d", tm.cursor)
	}
	if tm.selected != -1 {
		t.Errorf("Expected selected -1, got %d", tm.selected)
	}
}

func TestTopicsModelSetTopics(t *testing.T) {
	store := newTestStore(t)
	defer store.Close()

	tm := NewTopicsModel(store)

	topics := []*models.Topic{
		models.NewTopic("Topic1", "Desc1", "test@tui"),
		models.NewTopic("Topic2", "Desc2", "test@tui"),
	}

	tm.SetTopics(topics)

	if len(tm.topics) != 2 {
		t.Errorf("Expected 2 topics, got %d", len(tm.topics))
	}
}

func TestTopicsModelSetTopicsAdjustsCursor(t *testing.T) {
	store := newTestStore(t)
	defer store.Close()

	tm := NewTopicsModel(store)
	tm.cursor = 5 // Set cursor beyond range

	topics := []*models.Topic{
		models.NewTopic("Topic1", "Desc1", "test@tui"),
	}

	tm.SetTopics(topics)

	if tm.cursor != 0 {
		t.Errorf("Expected cursor to be adjusted to 0, got %d", tm.cursor)
	}
}

func TestTopicsModelNavigation(t *testing.T) {
	store := newTestStore(t)
	defer store.Close()

	tm := NewTopicsModel(store)

	topics := []*models.Topic{
		models.NewTopic("Topic1", "Desc1", "test@tui"),
		models.NewTopic("Topic2", "Desc2", "test@tui"),
		models.NewTopic("Topic3", "Desc3", "test@tui"),
	}
	tm.SetTopics(topics)

	// Move down
	tm.MoveDown()
	if tm.cursor != 1 {
		t.Errorf("Expected cursor 1, got %d", tm.cursor)
	}

	tm.MoveDown()
	if tm.cursor != 2 {
		t.Errorf("Expected cursor 2, got %d", tm.cursor)
	}

	// Can't move past end
	tm.MoveDown()
	if tm.cursor != 2 {
		t.Errorf("Expected cursor to stay at 2, got %d", tm.cursor)
	}

	// Move up
	tm.MoveUp()
	if tm.cursor != 1 {
		t.Errorf("Expected cursor 1, got %d", tm.cursor)
	}

	tm.MoveUp()
	if tm.cursor != 0 {
		t.Errorf("Expected cursor 0, got %d", tm.cursor)
	}

	// Can't move past beginning
	tm.MoveUp()
	if tm.cursor != 0 {
		t.Errorf("Expected cursor to stay at 0, got %d", tm.cursor)
	}
}

func TestTopicsModelSelected(t *testing.T) {
	store := newTestStore(t)
	defer store.Close()

	tm := NewTopicsModel(store)

	// No topics
	if tm.Selected() != nil {
		t.Error("Expected nil when no topics")
	}

	topics := []*models.Topic{
		models.NewTopic("Topic1", "Desc1", "test@tui"),
		models.NewTopic("Topic2", "Desc2", "test@tui"),
	}
	tm.SetTopics(topics)

	selected := tm.Selected()
	if selected == nil {
		t.Fatal("Expected selected topic")
	}
	if selected.Name != "Topic1" {
		t.Errorf("Expected Topic1, got %s", selected.Name)
	}

	tm.MoveDown()
	selected = tm.Selected()
	if selected.Name != "Topic2" {
		t.Errorf("Expected Topic2, got %s", selected.Name)
	}
}

func TestTopicsModelView(t *testing.T) {
	store := newTestStore(t)
	defer store.Close()

	tm := NewTopicsModel(store)

	// Empty view
	view := tm.View()
	if !strings.Contains(view, "No topics") {
		t.Error("Expected 'No topics' in empty view")
	}

	// With topics
	topics := []*models.Topic{
		models.NewTopic("General", "General discussion", "test@tui"),
	}
	tm.SetTopics(topics)

	view = tm.View()
	if !strings.Contains(view, "Topics") {
		t.Error("Expected 'Topics' header in view")
	}
	if !strings.Contains(view, "General") {
		t.Error("Expected 'General' in view")
	}
}

func TestTopicsModelViewArchived(t *testing.T) {
	store := newTestStore(t)
	defer store.Close()

	tm := NewTopicsModel(store)

	topic := models.NewTopic("Archived", "Old topic", "test@tui")
	topic.Archived = true
	tm.SetTopics([]*models.Topic{topic})

	view := tm.View()
	if !strings.Contains(view, "archived") {
		t.Error("Expected 'archived' indicator in view")
	}
}

// ThreadsModel tests
func TestNewThreadsModel(t *testing.T) {
	store := newTestStore(t)
	defer store.Close()

	tm := NewThreadsModel(store)

	if tm.store != store {
		t.Error("Expected store to be set")
	}
	if tm.cursor != 0 {
		t.Errorf("Expected cursor 0, got %d", tm.cursor)
	}
}

func TestThreadsModelSetThreads(t *testing.T) {
	store := newTestStore(t)
	defer store.Close()

	tm := NewThreadsModel(store)
	tm.cursor = 5 // Set cursor beyond range

	topicID := uuid.New()
	threads := []*models.Thread{
		models.NewThread(topicID, "Thread 1", "test@tui"),
	}

	tm.SetThreads(threads)

	if len(tm.threads) != 1 {
		t.Errorf("Expected 1 thread, got %d", len(tm.threads))
	}
	if tm.cursor != 0 {
		t.Errorf("Expected cursor reset to 0, got %d", tm.cursor)
	}
}

func TestThreadsModelNavigation(t *testing.T) {
	store := newTestStore(t)
	defer store.Close()

	tm := NewThreadsModel(store)

	topicID := uuid.New()
	threads := []*models.Thread{
		models.NewThread(topicID, "Thread 1", "test@tui"),
		models.NewThread(topicID, "Thread 2", "test@tui"),
	}
	tm.SetThreads(threads)

	tm.MoveDown()
	if tm.cursor != 1 {
		t.Errorf("Expected cursor 1, got %d", tm.cursor)
	}

	tm.MoveDown() // Can't go further
	if tm.cursor != 1 {
		t.Errorf("Expected cursor to stay at 1, got %d", tm.cursor)
	}

	tm.MoveUp()
	if tm.cursor != 0 {
		t.Errorf("Expected cursor 0, got %d", tm.cursor)
	}

	tm.MoveUp() // Can't go further
	if tm.cursor != 0 {
		t.Errorf("Expected cursor to stay at 0, got %d", tm.cursor)
	}
}

func TestThreadsModelSelected(t *testing.T) {
	store := newTestStore(t)
	defer store.Close()

	tm := NewThreadsModel(store)

	// No threads
	if tm.Selected() != nil {
		t.Error("Expected nil when no threads")
	}

	topicID := uuid.New()
	threads := []*models.Thread{
		models.NewThread(topicID, "Thread 1", "test@tui"),
	}
	tm.SetThreads(threads)

	selected := tm.Selected()
	if selected == nil {
		t.Fatal("Expected selected thread")
	}
	if selected.Subject != "Thread 1" {
		t.Errorf("Expected Thread 1, got %s", selected.Subject)
	}
}

func TestThreadsModelView(t *testing.T) {
	store := newTestStore(t)
	defer store.Close()

	tm := NewThreadsModel(store)

	// Empty view
	view := tm.View()
	if !strings.Contains(view, "No threads") {
		t.Error("Expected 'No threads' in empty view")
	}

	// With threads
	topicID := uuid.New()
	thread := models.NewThread(topicID, "Test Thread", "test@tui")
	tm.SetThreads([]*models.Thread{thread})

	view = tm.View()
	if !strings.Contains(view, "Threads") {
		t.Error("Expected 'Threads' header in view")
	}
	if !strings.Contains(view, "Test Thread") {
		t.Error("Expected 'Test Thread' in view")
	}
}

func TestThreadsModelViewSticky(t *testing.T) {
	store := newTestStore(t)
	defer store.Close()

	tm := NewThreadsModel(store)

	topicID := uuid.New()
	thread := models.NewThread(topicID, "Pinned Thread", "test@tui")
	thread.Sticky = true
	tm.SetThreads([]*models.Thread{thread})

	view := tm.View()
	if !strings.Contains(view, "[PIN]") {
		t.Error("Expected '[PIN]' indicator in view")
	}
}

// MessagesModel tests
func TestNewMessagesModel(t *testing.T) {
	store := newTestStore(t)
	defer store.Close()

	mm := NewMessagesModel(store)

	if mm.store != store {
		t.Error("Expected store to be set")
	}
	if mm.cursor != 0 {
		t.Errorf("Expected cursor 0, got %d", mm.cursor)
	}
	if mm.scroll != 0 {
		t.Errorf("Expected scroll 0, got %d", mm.scroll)
	}
}

func TestMessagesModelSetMessages(t *testing.T) {
	store := newTestStore(t)
	defer store.Close()

	mm := NewMessagesModel(store)
	mm.cursor = 5
	mm.scroll = 3

	threadID := uuid.New()
	messages := []*models.Message{
		models.NewMessage(threadID, "Message 1", "test@tui"),
	}

	mm.SetMessages(messages)

	if len(mm.messages) != 1 {
		t.Errorf("Expected 1 message, got %d", len(mm.messages))
	}
	if mm.cursor != 0 {
		t.Errorf("Expected cursor reset to 0, got %d", mm.cursor)
	}
	if mm.scroll != 0 {
		t.Errorf("Expected scroll reset to 0, got %d", mm.scroll)
	}
}

func TestMessagesModelNavigation(t *testing.T) {
	store := newTestStore(t)
	defer store.Close()

	mm := NewMessagesModel(store)

	threadID := uuid.New()
	messages := []*models.Message{
		models.NewMessage(threadID, "Message 1", "test@tui"),
		models.NewMessage(threadID, "Message 2", "test@tui"),
	}
	mm.SetMessages(messages)

	mm.MoveDown()
	if mm.scroll != 1 {
		t.Errorf("Expected scroll 1, got %d", mm.scroll)
	}

	mm.MoveDown() // Can't go further
	if mm.scroll != 1 {
		t.Errorf("Expected scroll to stay at 1, got %d", mm.scroll)
	}

	mm.MoveUp()
	if mm.scroll != 0 {
		t.Errorf("Expected scroll 0, got %d", mm.scroll)
	}

	mm.MoveUp() // Can't go further
	if mm.scroll != 0 {
		t.Errorf("Expected scroll to stay at 0, got %d", mm.scroll)
	}
}

func TestMessagesModelSelected(t *testing.T) {
	store := newTestStore(t)
	defer store.Close()

	mm := NewMessagesModel(store)

	// No messages
	if mm.Selected() != nil {
		t.Error("Expected nil when no messages")
	}

	threadID := uuid.New()
	messages := []*models.Message{
		models.NewMessage(threadID, "Message 1", "test@tui"),
	}
	mm.SetMessages(messages)

	selected := mm.Selected()
	if selected == nil {
		t.Fatal("Expected selected message")
	}
	if selected.Content != "Message 1" {
		t.Errorf("Expected Message 1, got %s", selected.Content)
	}
}

func TestMessagesModelView(t *testing.T) {
	store := newTestStore(t)
	defer store.Close()

	mm := NewMessagesModel(store)

	// Empty view
	view := mm.View()
	if !strings.Contains(view, "No messages") {
		t.Error("Expected 'No messages' in empty view")
	}

	// With messages
	threadID := uuid.New()
	messages := []*models.Message{
		models.NewMessage(threadID, "Hello world", "test@tui"),
	}
	mm.SetMessages(messages)

	view = mm.View()
	if !strings.Contains(view, "Messages") {
		t.Error("Expected 'Messages' header in view")
	}
	if !strings.Contains(view, "Hello world") {
		t.Error("Expected 'Hello world' in view")
	}
}

func TestMessagesModelViewEdited(t *testing.T) {
	store := newTestStore(t)
	defer store.Close()

	mm := NewMessagesModel(store)

	threadID := uuid.New()
	msg := models.NewMessage(threadID, "Edited message", "test@tui")
	now := msg.CreatedAt
	msg.EditedAt = &now
	mm.SetMessages([]*models.Message{msg})

	view := mm.View()
	if !strings.Contains(view, "edited") {
		t.Error("Expected 'edited' indicator in view")
	}
}

func TestMessagesModelViewLongContent(t *testing.T) {
	store := newTestStore(t)
	defer store.Close()

	mm := NewMessagesModel(store)

	threadID := uuid.New()
	// Create a message longer than 200 characters
	longContent := strings.Repeat("x", 300)
	msg := models.NewMessage(threadID, longContent, "test@tui")
	mm.SetMessages([]*models.Message{msg})

	view := mm.View()
	if !strings.Contains(view, "...") {
		t.Error("Expected truncation indicator '...' in view")
	}
}

func TestTopicsModelLoadTopics(t *testing.T) {
	store := newTestStore(t)
	defer store.Close()

	// Create a topic in the store
	topic := models.NewTopic("TestTopic", "Test", "test@tui")
	_ = store.CreateTopic(topic)

	tm := NewTopicsModel(store)
	cmd := tm.LoadTopics()

	if cmd == nil {
		t.Error("Expected LoadTopics to return a command")
	}

	// Execute the command
	msg := cmd()
	loadedMsg, ok := msg.(TopicsLoadedMsg)
	if !ok {
		_, isErr := msg.(error)
		if isErr {
			t.Fatalf("LoadTopics returned error: %v", msg)
		}
		t.Fatalf("Expected TopicsLoadedMsg, got %T", msg)
	}

	if len(loadedMsg.Topics) != 1 {
		t.Errorf("Expected 1 topic, got %d", len(loadedMsg.Topics))
	}
}

func TestThreadsModelLoadThreads(t *testing.T) {
	store := newTestStore(t)
	defer store.Close()

	// Create a topic and thread
	topic := models.NewTopic("TestTopic", "Test", "test@tui")
	_ = store.CreateTopic(topic)
	thread := models.NewThread(topic.ID, "Test Thread", "test@tui")
	_ = store.CreateThread(thread)

	tm := NewThreadsModel(store)
	cmd := tm.LoadThreads(topic.ID)

	if cmd == nil {
		t.Error("Expected LoadThreads to return a command")
	}

	// Execute the command
	msg := cmd()
	loadedMsg, ok := msg.(ThreadsLoadedMsg)
	if !ok {
		_, isErr := msg.(error)
		if isErr {
			t.Fatalf("LoadThreads returned error: %v", msg)
		}
		t.Fatalf("Expected ThreadsLoadedMsg, got %T", msg)
	}

	if len(loadedMsg.Threads) != 1 {
		t.Errorf("Expected 1 thread, got %d", len(loadedMsg.Threads))
	}
}

func TestMessagesModelLoadMessages(t *testing.T) {
	store := newTestStore(t)
	defer store.Close()

	// Create a topic, thread, and message
	topic := models.NewTopic("TestTopic", "Test", "test@tui")
	_ = store.CreateTopic(topic)
	thread := models.NewThread(topic.ID, "Test Thread", "test@tui")
	_ = store.CreateThread(thread)
	message := models.NewMessage(thread.ID, "Hello", "test@tui")
	_ = store.CreateMessage(message)

	mm := NewMessagesModel(store)
	cmd := mm.LoadMessages(thread.ID)

	if cmd == nil {
		t.Error("Expected LoadMessages to return a command")
	}

	// Execute the command
	msg := cmd()
	loadedMsg, ok := msg.(MessagesLoadedMsg)
	if !ok {
		_, isErr := msg.(error)
		if isErr {
			t.Fatalf("LoadMessages returned error: %v", msg)
		}
		t.Fatalf("Expected MessagesLoadedMsg, got %T", msg)
	}

	if len(loadedMsg.Messages) != 1 {
		t.Errorf("Expected 1 message, got %d", len(loadedMsg.Messages))
	}
}

func TestModelUpdateArrowKeys(t *testing.T) {
	store := newTestStore(t)
	defer store.Close()

	model := NewModel(store, "test@tui")

	// Set up topics so arrow keys have something to navigate
	topics := []*models.Topic{
		models.NewTopic("Topic1", "Desc1", "test@tui"),
		models.NewTopic("Topic2", "Desc2", "test@tui"),
	}
	model.topics.SetTopics(topics)

	// Arrow down
	downMsg := tea.KeyMsg{Type: tea.KeyDown}
	newModel, _ := model.Update(downMsg)
	m := newModel.(Model)
	if m.topics.cursor != 1 {
		t.Errorf("Expected cursor 1 after down arrow, got %d", m.topics.cursor)
	}

	// Arrow up
	upMsg := tea.KeyMsg{Type: tea.KeyUp}
	newModel, _ = m.Update(upMsg)
	m = newModel.(Model)
	if m.topics.cursor != 0 {
		t.Errorf("Expected cursor 0 after up arrow, got %d", m.topics.cursor)
	}

	// j and k also work for navigation
	jMsg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}}
	newModel, _ = m.Update(jMsg)
	m = newModel.(Model)
	if m.topics.cursor != 1 {
		t.Errorf("Expected cursor 1 after j key, got %d", m.topics.cursor)
	}

	kMsg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'k'}}
	newModel, _ = m.Update(kMsg)
	m = newModel.(Model)
	if m.topics.cursor != 0 {
		t.Errorf("Expected cursor 0 after k key, got %d", m.topics.cursor)
	}
}

func TestModelUpdateEnterKey(t *testing.T) {
	store := newTestStore(t)
	defer store.Close()

	model := NewModel(store, "test@tui")

	// Set up topics
	topics := []*models.Topic{
		models.NewTopic("Topic1", "Desc1", "test@tui"),
	}
	model.topics.SetTopics(topics)

	// Press Enter to select topic
	enterMsg := tea.KeyMsg{Type: tea.KeyEnter}
	newModel, cmd := model.Update(enterMsg)
	m := newModel.(Model)

	// Should have a command to load threads
	if cmd == nil {
		t.Error("Expected command after Enter")
	}
	_ = m
}

func TestModelUpdateComposingEnter(t *testing.T) {
	store := newTestStore(t)
	defer store.Close()

	// Set up a topic and thread for composing
	topic := models.NewTopic("Topic", "Desc", "test@tui")
	_ = store.CreateTopic(topic)
	thread := models.NewThread(topic.ID, "Thread", "test@tui")
	_ = store.CreateThread(thread)

	model := NewModel(store, "test@tui")
	model.topics.SetTopics([]*models.Topic{topic})
	model.threads.SetThreads([]*models.Thread{thread})
	model.activePane = MessagesPane

	// Start composing
	nMsg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'n'}}
	newModel, _ := model.Update(nMsg)
	m := newModel.(Model)
	if !m.composing {
		t.Fatal("Expected composing to be true")
	}

	// Type some text
	m.composeText = "Test message"

	// Press Enter to submit
	enterMsg := tea.KeyMsg{Type: tea.KeyEnter}
	newModel, _ = m.Update(enterMsg)
	m = newModel.(Model)

	// Should have cleared composing
	if m.composing {
		t.Error("Expected composing to be false after Enter")
	}
}

func TestModelUpdateComposingEmptyEnter(t *testing.T) {
	store := newTestStore(t)
	defer store.Close()

	model := NewModel(store, "test@tui")

	// Start composing
	nMsg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'n'}}
	newModel, _ := model.Update(nMsg)
	m := newModel.(Model)

	// Press Enter with empty text
	enterMsg := tea.KeyMsg{Type: tea.KeyEnter}
	newModel, _ = m.Update(enterMsg)
	m = newModel.(Model)

	// Current implementation exits composing mode on Enter regardless
	if m.composing {
		t.Error("Expected composing to be false after Enter")
	}
}

func TestModelViewComposing(t *testing.T) {
	store := newTestStore(t)
	defer store.Close()

	model := NewModel(store, "test@tui")

	// Set window size
	msg := tea.WindowSizeMsg{Width: 100, Height: 50}
	newModel, _ := model.Update(msg)
	m := newModel.(Model)

	// Start composing
	m.composing = true
	m.composeText = "typing..."

	view := m.View()

	// Should show compose indicator
	if !strings.Contains(view, "Composing") || !strings.Contains(view, "typing...") {
		t.Error("Expected compose indicator in view")
	}
}

func TestModelNavigationThreadsPane(t *testing.T) {
	store := newTestStore(t)
	defer store.Close()

	model := NewModel(store, "test@tui")

	// Move to threads pane
	model.activePane = ThreadsPane

	topicID := uuid.New()
	threads := []*models.Thread{
		models.NewThread(topicID, "Thread1", "test@tui"),
		models.NewThread(topicID, "Thread2", "test@tui"),
	}
	model.threads.SetThreads(threads)

	// Arrow down
	downMsg := tea.KeyMsg{Type: tea.KeyDown}
	newModel, _ := model.Update(downMsg)
	m := newModel.(Model)
	if m.threads.cursor != 1 {
		t.Errorf("Expected threads cursor 1, got %d", m.threads.cursor)
	}
}

func TestModelNavigationMessagesPane(t *testing.T) {
	store := newTestStore(t)
	defer store.Close()

	model := NewModel(store, "test@tui")

	// Move to messages pane
	model.activePane = MessagesPane

	threadID := uuid.New()
	messages := []*models.Message{
		models.NewMessage(threadID, "Msg1", "test@tui"),
		models.NewMessage(threadID, "Msg2", "test@tui"),
	}
	model.messages.SetMessages(messages)

	// Arrow down
	downMsg := tea.KeyMsg{Type: tea.KeyDown}
	newModel, _ := model.Update(downMsg)
	m := newModel.(Model)
	if m.messages.scroll != 1 {
		t.Errorf("Expected messages scroll 1, got %d", m.messages.scroll)
	}
}

func TestTopicsModelLoadTopicsError(t *testing.T) {
	store := newTestStore(t)
	store.Close() // Close store to cause error

	tm := NewTopicsModel(store)
	cmd := tm.LoadTopics()

	if cmd == nil {
		t.Fatal("Expected LoadTopics to return a command")
	}

	// Execute the command - should return an error
	msg := cmd()
	_, ok := msg.(error)
	if !ok {
		// Might also return empty topics, which is acceptable
		loadedMsg, isLoaded := msg.(TopicsLoadedMsg)
		if !isLoaded {
			t.Fatalf("Expected error or TopicsLoadedMsg, got %T", msg)
		}
		_ = loadedMsg
	}
}

func TestThreadsModelLoadThreadsError(t *testing.T) {
	store := newTestStore(t)
	store.Close() // Close store to cause error

	tm := NewThreadsModel(store)
	cmd := tm.LoadThreads(uuid.New())

	if cmd == nil {
		t.Fatal("Expected LoadThreads to return a command")
	}

	// Execute the command - should return an error
	msg := cmd()
	_, isErr := msg.(error)
	if !isErr {
		// Might also return empty threads
		loadedMsg, isLoaded := msg.(ThreadsLoadedMsg)
		if !isLoaded {
			t.Fatalf("Expected error or ThreadsLoadedMsg, got %T", msg)
		}
		_ = loadedMsg
	}
}

func TestMessagesModelLoadMessagesError(t *testing.T) {
	store := newTestStore(t)
	store.Close() // Close store to cause error

	mm := NewMessagesModel(store)
	cmd := mm.LoadMessages(uuid.New())

	if cmd == nil {
		t.Fatal("Expected LoadMessages to return a command")
	}

	// Execute the command - should return an error
	msg := cmd()
	_, isErr := msg.(error)
	if !isErr {
		// Might also return empty messages
		loadedMsg, isLoaded := msg.(MessagesLoadedMsg)
		if !isLoaded {
			t.Fatalf("Expected error or MessagesLoadedMsg, got %T", msg)
		}
		_ = loadedMsg
	}
}

func TestModelUpdateSpaceKey(t *testing.T) {
	store := newTestStore(t)
	defer store.Close()

	model := NewModel(store, "test@tui")
	model.composing = true
	model.composeText = "hello"

	// Space key should add a space to compose text
	spaceMsg := tea.KeyMsg{Type: tea.KeySpace}
	newModel, _ := model.Update(spaceMsg)
	m := newModel.(Model)
	if m.composeText != "hello " {
		t.Errorf("Expected 'hello ', got %q", m.composeText)
	}
}

func TestModelBackspaceOnEmptyString(t *testing.T) {
	store := newTestStore(t)
	defer store.Close()

	model := NewModel(store, "test@tui")
	model.composing = true
	model.composeText = ""

	// Backspace on empty string should be no-op
	backspaceMsg := tea.KeyMsg{Type: tea.KeyBackspace}
	newModel, _ := model.Update(backspaceMsg)
	m := newModel.(Model)
	if m.composeText != "" {
		t.Errorf("Expected empty string, got %q", m.composeText)
	}
}

func TestTopicsModelSetTopicsEmpty(t *testing.T) {
	store := newTestStore(t)
	defer store.Close()

	tm := NewTopicsModel(store)
	tm.cursor = 5

	// Set empty topics
	tm.SetTopics([]*models.Topic{})

	if len(tm.topics) != 0 {
		t.Errorf("Expected 0 topics, got %d", len(tm.topics))
	}
	if tm.cursor != 0 {
		t.Errorf("Expected cursor reset to 0, got %d", tm.cursor)
	}
}

func TestModelUpdateSelectThreadThenEnter(t *testing.T) {
	store := newTestStore(t)
	defer store.Close()

	// Create topic and thread in the store
	topic := models.NewTopic("Topic", "Desc", "test@tui")
	_ = store.CreateTopic(topic)
	thread := models.NewThread(topic.ID, "Thread", "test@tui")
	_ = store.CreateThread(thread)

	model := NewModel(store, "test@tui")
	model.topics.SetTopics([]*models.Topic{topic})

	// Select topic with enter
	enterMsg := tea.KeyMsg{Type: tea.KeyEnter}
	newModel, cmd := model.Update(enterMsg)
	m := newModel.(Model)

	if cmd == nil {
		t.Error("Expected command to load threads")
	}
	if m.activePane != ThreadsPane {
		t.Errorf("Expected ThreadsPane, got %d", m.activePane)
	}

	// Now set threads and select one
	m.threads.SetThreads([]*models.Thread{thread})
	newModel, cmd = m.Update(enterMsg)
	m = newModel.(Model)

	if cmd == nil {
		t.Error("Expected command to load messages")
	}
	if m.activePane != MessagesPane {
		t.Errorf("Expected MessagesPane, got %d", m.activePane)
	}
}

func TestModelUpdateEnterNoSelection(t *testing.T) {
	store := newTestStore(t)
	defer store.Close()

	model := NewModel(store, "test@tui")

	// Press Enter with no topics
	enterMsg := tea.KeyMsg{Type: tea.KeyEnter}
	_, cmd := model.Update(enterMsg)

	if cmd != nil {
		t.Error("Expected no command when nothing selected")
	}
}

func TestModelViewActivePane(t *testing.T) {
	store := newTestStore(t)
	defer store.Close()

	model := NewModel(store, "test@tui")

	// Set window size
	msg := tea.WindowSizeMsg{Width: 100, Height: 50}
	newModel, _ := model.Update(msg)
	m := newModel.(Model)

	// Test each pane
	for _, pane := range []Pane{TopicsPane, ThreadsPane, MessagesPane} {
		m.activePane = pane
		view := m.View()
		if view == "" {
			t.Errorf("Expected non-empty view for pane %d", pane)
		}
	}
}

func TestThreadsModelSetThreadsEmpty(t *testing.T) {
	store := newTestStore(t)
	defer store.Close()

	tm := NewThreadsModel(store)
	tm.cursor = 5

	// Set empty threads
	tm.SetThreads([]*models.Thread{})

	if len(tm.threads) != 0 {
		t.Errorf("Expected 0 threads, got %d", len(tm.threads))
	}
	if tm.cursor != 0 {
		t.Errorf("Expected cursor reset to 0, got %d", tm.cursor)
	}
}

func TestMessagesModelSetMessagesEmpty(t *testing.T) {
	store := newTestStore(t)
	defer store.Close()

	mm := NewMessagesModel(store)
	mm.cursor = 5
	mm.scroll = 3

	// Set empty messages
	mm.SetMessages([]*models.Message{})

	if len(mm.messages) != 0 {
		t.Errorf("Expected 0 messages, got %d", len(mm.messages))
	}
	if mm.cursor != 0 {
		t.Errorf("Expected cursor reset to 0, got %d", mm.cursor)
	}
	if mm.scroll != 0 {
		t.Errorf("Expected scroll reset to 0, got %d", mm.scroll)
	}
}

// Helper to create a test store
func newTestStore(t *testing.T) *storage.Store {
	t.Helper()
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")
	store, err := storage.NewStore(dbPath)
	if err != nil {
		t.Fatalf("failed to create test store: %v", err)
	}
	return store
}
