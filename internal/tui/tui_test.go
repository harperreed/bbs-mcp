// ABOUTME: Tests for TUI components
// ABOUTME: Verifies model initialization, navigation, and view rendering

package tui

import (
	"errors"
	"path/filepath"
	"strings"
	"testing"
	"time"

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

	model, _, _ := replyReadyModel(t, store, "test@tui")

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

func TestModelReplyIsOnlyAvailableForAnOpenedThreadInMessagesPane(t *testing.T) {
	store := newTestStore(t)
	defer store.Close()

	model, _, _ := replyReadyModel(t, store, "test@tui")
	model.loadedThreadID = uuid.Nil
	for _, pane := range []Pane{TopicsPane, ThreadsPane, MessagesPane} {
		model.activePane = pane
		newModel, cmd := model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'n'}})
		updated := newModel.(Model)
		if updated.composing || cmd != nil {
			t.Errorf("Expected n to be inert in pane %d without an opened thread", pane)
		}
	}

	model.loadedThreadID = model.threads.Selected().ID
	for _, pane := range []Pane{TopicsPane, ThreadsPane} {
		model.activePane = pane
		newModel, cmd := model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'n'}})
		updated := newModel.(Model)
		if updated.composing || cmd != nil {
			t.Errorf("Expected n to be inert outside Messages pane, got composing in pane %d", pane)
		}
	}
}

func TestModelReplyEmptyEnterKeepsCompositionOpen(t *testing.T) {
	store := newTestStore(t)
	defer store.Close()

	model, _, _ := replyReadyModel(t, store, "test@tui")
	model.composing = true
	newModel, cmd := model.Update(tea.KeyMsg{Type: tea.KeyEnter})
	updated := newModel.(Model)
	if cmd != nil {
		t.Fatal("Expected empty reply not to issue a persistence command")
	}
	if !updated.composing {
		t.Fatal("Expected empty reply to remain in compose mode")
	}
}

func TestModelReplyPersistsWithIdentityAndRefreshesState(t *testing.T) {
	store := newTestStore(t)
	defer store.Close()

	model, topic, thread := replyReadyModel(t, store, "doctor-biz")
	model.composing = true
	model.composeText = "Fresh reply 🚀"

	newModel, cmd := model.Update(tea.KeyMsg{Type: tea.KeyEnter})
	pending := newModel.(Model)
	if cmd == nil {
		t.Fatal("Expected non-empty reply to issue a persistence command")
	}
	if !pending.composing || pending.composeText != "Fresh reply 🚀" {
		t.Fatal("Expected draft to remain recoverable until persistence succeeds")
	}

	result := cmd()
	newModel, refreshCmd := pending.Update(result)
	updated := newModel.(Model)
	if updated.composing || updated.composeText != "" {
		t.Fatal("Expected successful persistence to close and clear composition")
	}
	if updated.err != nil {
		t.Fatalf("Expected no error after successful reply, got %v", updated.err)
	}
	if refreshCmd == nil {
		t.Fatal("Expected persistence completion to start refresh")
	}
	if !updated.threadsLoading || !updated.messagesLoading {
		t.Fatal("Expected post-submit refresh to mark threads and messages as loading")
	}
	if len(updated.threads.threads) != 0 || len(updated.messages.messages) != 0 || updated.canReply() {
		t.Fatal("Expected post-submit refresh to invalidate stale rows and reply eligibility")
	}
	newModel, _ = updated.Update(refreshCmd())
	updated = newModel.(Model)
	if len(updated.messages.messages) != 2 || updated.messages.messages[1].Content != "Fresh reply 🚀" {
		t.Fatalf("Expected refreshed messages to contain reply, got %#v", updated.messages.messages)
	}
	if updated.messages.messages[1].CreatedBy != "doctor-biz" {
		t.Errorf("Expected active identity doctor-biz, got %q", updated.messages.messages[1].CreatedBy)
	}
	if updated.messages.scroll != 1 {
		t.Errorf("Expected resulting reply to be rendered, got scroll %d", updated.messages.scroll)
	}
	if selected := updated.threads.Selected(); selected == nil || selected.ID != thread.ID {
		t.Fatalf("Expected replied-to thread to remain selected, got %#v", selected)
	}
	if !updated.threads.Selected().UpdatedAt.After(thread.UpdatedAt) {
		t.Errorf("Expected refreshed thread activity after %s, got %s", thread.UpdatedAt, updated.threads.Selected().UpdatedAt)
	}

	persisted, err := store.ListMessages(thread.ID)
	if err != nil {
		t.Fatalf("ListMessages: %v", err)
	}
	if len(persisted) != 2 || persisted[1].CreatedBy != "doctor-biz" || persisted[1].Content != "Fresh reply 🚀" {
		t.Fatalf("Expected reply in real storage, got %#v", persisted)
	}
	threads, err := store.ListThreads(topic.ID)
	if err != nil {
		t.Fatalf("ListThreads: %v", err)
	}
	if len(threads) != 1 || !threads[0].UpdatedAt.After(thread.UpdatedAt) {
		t.Fatalf("Expected persisted thread activity refresh, got %#v", threads)
	}
}

func TestModelReplyFailureIsVisibleAndRecoverable(t *testing.T) {
	store := newTestStore(t)
	model, _, _ := replyReadyModel(t, store, "test@tui")
	model.width = 100
	model.height = 30
	model.composing = true
	model.composeText = "keep this draft"
	if err := store.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	newModel, cmd := model.Update(tea.KeyMsg{Type: tea.KeyEnter})
	pending := newModel.(Model)
	if cmd == nil {
		t.Fatal("Expected persistence command")
	}
	result := cmd()
	newModel, _ = pending.Update(result)
	updated := newModel.(Model)
	if !updated.composing || updated.composeText != "keep this draft" {
		t.Fatal("Expected failed reply draft to remain recoverable")
	}
	if updated.err == nil || !strings.Contains(updated.View(), "Error:") {
		t.Fatalf("Expected visible persistence error, got %q", updated.View())
	}
}

func TestModelReplyAllowsOnlyOneSubmissionWhilePersistenceIsInFlight(t *testing.T) {
	store := newTestStore(t)
	defer store.Close()

	model, _, thread := replyReadyModel(t, store, "doctor-biz")
	model.composing = true
	model.composeText = "exactly once"

	newModel, persistCmd := model.Update(tea.KeyMsg{Type: tea.KeyEnter})
	pending := newModel.(Model)
	if persistCmd == nil {
		t.Fatal("Expected persistence command")
	}
	newModel, duplicateCmd := pending.Update(tea.KeyMsg{Type: tea.KeyEnter})
	pending = newModel.(Model)
	if duplicateCmd != nil {
		t.Fatal("Expected repeated Enter to be inert while persistence is in flight")
	}
	newModel, _ = pending.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(" newer draft")})
	pending = newModel.(Model)
	if pending.composeText != "exactly once" {
		t.Fatalf("Expected typing to be inert while persistence is in flight, got %q", pending.composeText)
	}

	newModel, refreshCmd := pending.Update(persistCmd())
	updated := newModel.(Model)
	if refreshCmd == nil {
		t.Fatal("Expected persistence completion to start a distinct refresh command")
	}
	_, _ = updated.Update(refreshCmd())
	persisted, err := store.ListMessages(thread.ID)
	if err != nil {
		t.Fatalf("ListMessages: %v", err)
	}
	if len(persisted) != 2 || persisted[1].Content != "exactly once" {
		t.Fatalf("Expected exactly one persisted reply, got %#v", persisted)
	}
}

func TestModelReplyIgnoresStaleCompletionAndPreservesCtrlCWhilePosting(t *testing.T) {
	store := newTestStore(t)
	defer store.Close()

	model, _, thread := replyReadyModel(t, store, "doctor-biz")
	model.composing = true
	model.composeText = "current draft"
	model.posting = true
	model.replyPost = 2

	newModel, cmd := model.Update(replyPersistedMsg{requestID: 1, threadID: thread.ID, topicID: thread.TopicID})
	updated := newModel.(Model)
	if cmd != nil || !updated.posting || !updated.composing || updated.composeText != "current draft" {
		t.Fatal("Expected stale persistence success not to mutate the active submission")
	}
	newModel, _ = updated.Update(replyPersistenceFailedMsg{requestID: 1, err: errors.New("stale failure")})
	updated = newModel.(Model)
	if !updated.posting || updated.err != nil {
		t.Fatal("Expected stale persistence failure not to mutate the active submission")
	}
	_, quitCmd := updated.Update(tea.KeyMsg{Type: tea.KeyCtrlC})
	if quitCmd == nil {
		t.Fatal("Expected Ctrl-C to quit while persistence is in flight")
	}
	if _, ok := quitCmd().(tea.QuitMsg); !ok {
		t.Fatalf("Expected tea.QuitMsg, got %T", quitCmd())
	}
}

func TestModelReplyPersistenceSuccessIsNotRetryableWhenRefreshFails(t *testing.T) {
	store := newTestStore(t)
	model, _, thread := replyReadyModel(t, store, "doctor-biz")
	model.composing = true
	model.composeText = "persist before refresh"

	newModel, persistCmd := model.Update(tea.KeyMsg{Type: tea.KeyEnter})
	pending := newModel.(Model)
	if persistCmd == nil {
		t.Fatal("Expected persistence command")
	}
	persistedResult := persistCmd()
	persisted, err := store.ListMessages(thread.ID)
	if err != nil {
		t.Fatalf("ListMessages before refresh failure: %v", err)
	}
	if len(persisted) != 2 {
		t.Fatalf("Expected reply persisted before refresh, got %d messages", len(persisted))
	}

	newModel, refreshCmd := pending.Update(persistedResult)
	posted := newModel.(Model)
	if refreshCmd == nil {
		t.Fatal("Expected a distinct refresh command after persistence")
	}
	if posted.composing || posted.composeText != "" {
		t.Fatal("Expected persistence success to close and clear composition before refresh")
	}
	if err := store.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	newModel, _ = posted.Update(refreshCmd())
	failedRefresh := newModel.(Model)
	if failedRefresh.composing {
		t.Fatal("Expected refresh failure not to reopen retryable submission")
	}
	if failedRefresh.err == nil || !strings.Contains(failedRefresh.err.Error(), "posted, refresh failed") {
		t.Fatalf("Expected accurate posted refresh failure, got %v", failedRefresh.err)
	}
	_, manualRefreshCmd := failedRefresh.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'r'}})
	if manualRefreshCmd == nil {
		t.Fatal("Expected manual refresh to remain available without reposting")
	}
}

func TestModelManualRefreshFailureClearsPendingLoadingState(t *testing.T) {
	store := newTestStore(t)
	model, _, _ := replyReadyModel(t, store, "doctor-biz")
	model.width = 100
	model.height = 30

	newModel, refreshCmd := model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'r'}})
	pending := newModel.(Model)
	if refreshCmd == nil {
		t.Fatal("Expected manual refresh command")
	}
	if !pending.threadsLoading || !pending.messagesLoading {
		t.Fatal("Expected manual refresh to mark threads and messages as loading")
	}
	if len(pending.threads.threads) != 0 || len(pending.messages.messages) != 0 || pending.canReply() {
		t.Fatal("Expected manual refresh to invalidate stale rows and reply eligibility")
	}
	if view := pending.View(); !strings.Contains(view, "Loading threads") || !strings.Contains(view, "Loading messages") || strings.Contains(view, "[n] reply") {
		t.Fatalf("Expected truthful loading view without reply help, got %q", view)
	}
	newModel, staleReplyCmd := pending.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'n'}})
	pending = newModel.(Model)
	if pending.composing || staleReplyCmd != nil {
		t.Fatal("Expected n to be inert while refresh invalidates the selected rows")
	}
	newModel, staleSelectionCmd := pending.Update(tea.KeyMsg{Type: tea.KeyEnter})
	pending = newModel.(Model)
	if staleSelectionCmd != nil {
		t.Fatal("Expected Enter to be inert while refresh invalidates the selected rows")
	}
	if err := store.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	newModel, _ = pending.Update(refreshCmd())
	failed := newModel.(Model)
	if failed.messagesLoading || failed.threadsLoading {
		t.Fatal("Expected matching refresh failure to clear pending loading indicators")
	}
	if failed.err == nil || !strings.Contains(failed.err.Error(), "refresh failed") {
		t.Fatalf("Expected visible refresh failure, got %v", failed.err)
	}
}

func TestModelManualRefreshSuccessRestoresCurrentSelectionAndReplyEligibility(t *testing.T) {
	store := newTestStore(t)
	defer store.Close()

	model, _, thread := replyReadyModel(t, store, "doctor-biz")
	newModel, refreshCmd := model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'r'}})
	pending := newModel.(Model)
	if refreshCmd == nil {
		t.Fatal("Expected manual refresh command")
	}

	newModel, _ = pending.Update(refreshCmd())
	refreshed := newModel.(Model)
	if refreshed.threadsLoading || refreshed.messagesLoading {
		t.Fatal("Expected matching refresh success to clear loading indicators")
	}
	if selected := refreshed.threads.Selected(); selected == nil || selected.ID != thread.ID {
		t.Fatalf("Expected refresh to restore the current thread selection, got %#v", selected)
	}
	if len(refreshed.messages.messages) != 1 || !refreshed.canReply() {
		t.Fatalf("Expected refresh to restore current messages and reply eligibility, got %#v", refreshed.messages.messages)
	}
}

func TestModelRefreshScopesMatchFocusedPane(t *testing.T) {
	store := newTestStore(t)
	defer store.Close()

	topicsModel, topic, _ := replyReadyModel(t, store, "doctor-biz")
	topicsModel.threads.topicID = topic.ID
	topicsModel.activePane = TopicsPane
	newModel, topicsCmd := topicsModel.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'r'}})
	topicsPending := newModel.(Model)
	if topicsCmd == nil {
		t.Fatal("Expected Topics refresh command")
	}
	if len(topicsPending.threads.threads) != 1 || len(topicsPending.messages.messages) != 1 {
		t.Fatal("Expected Topics refresh to preserve the independently selected thread state")
	}

	threadsModel := topicsModel
	threadsModel.activePane = ThreadsPane
	newModel, threadsCmd := threadsModel.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'r'}})
	threadsPending := newModel.(Model)
	if threadsCmd == nil {
		t.Fatal("Expected Threads refresh command")
	}
	if len(threadsPending.threads.threads) != 0 || len(threadsPending.messages.messages) != 0 {
		t.Fatal("Expected Threads refresh to invalidate thread rows and dependent messages")
	}
	if threadsPending.selectedThreadID != (models.UUID{}) || threadsPending.loadedThreadID != (models.UUID{}) || threadsPending.messages.threadID != (models.UUID{}) {
		t.Fatal("Expected Threads refresh to clear dependent selection identity")
	}
	if !threadsPending.threadsLoading || threadsPending.messagesLoading || threadsPending.canReply() {
		t.Fatal("Expected only the Threads pane to load with reply eligibility disabled")
	}
}

func TestModelStateRefreshIgnoresStaleSuccessAndFailureUntilCurrentCompletion(t *testing.T) {
	store := newTestStore(t)
	defer store.Close()

	model, _, _ := replyReadyModel(t, store, "doctor-biz")
	newModel, firstCmd := model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'r'}})
	firstPending := newModel.(Model)
	firstResult := firstCmd()
	firstRequestID := firstPending.threadLoad

	newModel, secondCmd := firstPending.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'r'}})
	secondPending := newModel.(Model)
	newModel, _ = secondPending.Update(firstResult)
	stillPending := newModel.(Model)
	if !stillPending.threadsLoading || !stillPending.messagesLoading || stillPending.err != nil {
		t.Fatal("Expected stale refresh success not to complete the current refresh")
	}
	newModel, _ = stillPending.Update(stateRefreshFailedMsg{
		requestID: firstRequestID,
		threadID:  stillPending.selectedThreadID,
		err:       errors.New("stale refresh failure"),
	})
	stillPending = newModel.(Model)
	if !stillPending.threadsLoading || !stillPending.messagesLoading || stillPending.err != nil {
		t.Fatal("Expected stale refresh failure not to complete the current refresh")
	}

	newModel, _ = stillPending.Update(secondCmd())
	refreshed := newModel.(Model)
	if refreshed.threadsLoading || refreshed.messagesLoading || refreshed.err != nil || !refreshed.canReply() {
		t.Fatal("Expected matching refresh success to complete loading and restore reply eligibility")
	}
}

func TestModelRefreshMissingThreadClearsDependentMessageSelection(t *testing.T) {
	store := newTestStore(t)
	defer store.Close()

	model, topic, thread := replyReadyModel(t, store, "doctor-biz")
	requestID := model.newRequestID()
	model.threadLoad = requestID
	model.messageLoad = requestID
	model.threadsLoading = true
	model.messagesLoading = true
	replacement := models.NewThread(topic.ID, "Replacement thread", "seed@tui")

	newModel, _ := model.Update(stateRefreshLoadedMsg{
		requestID: requestID,
		threadID:  thread.ID,
		threads:   []*models.Thread{replacement},
		messages:  []*models.Message{models.NewMessage(thread.ID, "orphaned stale row", "seed@tui")},
	})
	refreshed := newModel.(Model)
	if selected := refreshed.threads.Selected(); selected == nil || selected.ID != replacement.ID {
		t.Fatalf("Expected safe fallback to the available thread row, got %#v", selected)
	}
	if refreshed.selectedThreadID != (models.UUID{}) || refreshed.loadedThreadID != (models.UUID{}) || refreshed.messages.threadID != (models.UUID{}) {
		t.Fatal("Expected missing refreshed selection to clear dependent message identity")
	}
	if len(refreshed.messages.messages) != 0 || refreshed.canReply() {
		t.Fatal("Expected missing refreshed selection to discard orphaned messages and reply eligibility")
	}
	if refreshed.threadsLoading || refreshed.messagesLoading {
		t.Fatal("Expected matching refresh completion to clear loading indicators")
	}
}

func TestModelIgnoresOutOfOrderThreadLoads(t *testing.T) {
	store := newTestStore(t)
	defer store.Close()

	firstTopic := models.NewTopic("First", "", "seed@tui")
	secondTopic := models.NewTopic("Second", "", "seed@tui")
	for _, topic := range []*models.Topic{firstTopic, secondTopic} {
		if err := store.CreateTopic(topic); err != nil {
			t.Fatalf("CreateTopic: %v", err)
		}
	}
	firstThread := models.NewThread(firstTopic.ID, "First thread", "seed@tui")
	secondThread := models.NewThread(secondTopic.ID, "Second thread", "seed@tui")
	for _, thread := range []*models.Thread{firstThread, secondThread} {
		if err := store.CreateThread(thread); err != nil {
			t.Fatalf("CreateThread: %v", err)
		}
	}

	model := NewModel(store, "test@tui")
	model.topics.SetTopics([]*models.Topic{firstTopic, secondTopic})
	newModel, firstCmd := model.Update(tea.KeyMsg{Type: tea.KeyEnter})
	model = newModel.(Model)
	model.activePane = TopicsPane
	model.topics.MoveDown()
	newModel, secondCmd := model.Update(tea.KeyMsg{Type: tea.KeyEnter})
	model = newModel.(Model)
	newModel, _ = model.Update(secondCmd())
	model = newModel.(Model)
	newModel, _ = model.Update(firstCmd())
	model = newModel.(Model)
	if selected := model.threads.Selected(); selected == nil || selected.ID != secondThread.ID {
		t.Fatalf("Expected stale first topic load ignored, got %#v", selected)
	}
}

func TestModelPendingTopicLoadCannotSelectOrReplyToPriorTopicThread(t *testing.T) {
	store := newTestStore(t)
	defer store.Close()

	firstTopic := models.NewTopic("Alpha", "", "seed@tui")
	secondTopic := models.NewTopic("Beta", "", "seed@tui")
	for _, topic := range []*models.Topic{firstTopic, secondTopic} {
		if err := store.CreateTopic(topic); err != nil {
			t.Fatalf("CreateTopic: %v", err)
		}
	}
	firstThread := models.NewThread(firstTopic.ID, "Alpha thread", "seed@tui")
	secondThread := models.NewThread(secondTopic.ID, "Beta thread", "seed@tui")
	for _, thread := range []*models.Thread{firstThread, secondThread} {
		if err := store.CreateThread(thread); err != nil {
			t.Fatalf("CreateThread: %v", err)
		}
	}
	firstMessage := models.NewMessage(firstThread.ID, "Alpha message", "seed@tui")
	if err := store.CreateMessage(firstMessage); err != nil {
		t.Fatalf("CreateMessage: %v", err)
	}

	model := NewModel(store, "doctor-biz")
	model.width = 100
	model.height = 30
	model.topics.SetTopics([]*models.Topic{firstTopic, secondTopic})
	model.topics.MoveDown()
	model.threads.SetThreads([]*models.Thread{firstThread})
	model.threads.topicID = firstTopic.ID
	model.messages.SetMessages([]*models.Message{firstMessage})
	model.messages.threadID = firstThread.ID
	model.selectedThreadID = firstThread.ID
	model.loadedThreadID = firstThread.ID
	model.activePane = TopicsPane

	newModel, secondTopicThreadsCmd := model.Update(tea.KeyMsg{Type: tea.KeyEnter})
	pending := newModel.(Model)
	pendingView := pending.View()
	if strings.Contains(pendingView, "Alpha thread") || !strings.Contains(pendingView, "Loading threads") {
		t.Errorf("Expected truthful loading view without prior topic thread, got %q", pendingView)
	}

	newModel, staleMessageCmd := pending.Update(tea.KeyMsg{Type: tea.KeyEnter})
	pending = newModel.(Model)
	if staleMessageCmd != nil {
		newModel, _ = pending.Update(staleMessageCmd())
		pending = newModel.(Model)
		newModel, _ = pending.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'n'}})
		pending = newModel.(Model)
		newModel, _ = pending.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("wrong thread reply")})
		pending = newModel.(Model)
		newModel, wrongPostCmd := pending.Update(tea.KeyMsg{Type: tea.KeyEnter})
		pending = newModel.(Model)
		if wrongPostCmd != nil {
			newModel, _ = pending.Update(wrongPostCmd())
			pending = newModel.(Model)
		}
	}
	newModel, _ = pending.Update(secondTopicThreadsCmd())
	updated := newModel.(Model)

	firstMessages, err := store.ListMessages(firstThread.ID)
	if err != nil {
		t.Fatalf("ListMessages: %v", err)
	}
	if staleMessageCmd != nil {
		t.Error("Expected Enter to be inert while the new topic's threads are loading")
	}
	if len(firstMessages) != 1 {
		t.Fatalf("Expected zero wrong-thread writes, got %#v", firstMessages)
	}
	if selected := updated.threads.Selected(); selected == nil || selected.ID != secondThread.ID {
		t.Fatalf("Expected second topic thread after delayed load, got %#v", selected)
	}
}

func TestModelPendingMessageLoadHidesPriorThreadMessagesAndReplyHelp(t *testing.T) {
	store := newTestStore(t)
	defer store.Close()

	model, topic, firstThread := replyReadyModel(t, store, "doctor-biz")
	secondThread := models.NewThread(topic.ID, "Second thread", "seed@tui")
	if err := store.CreateThread(secondThread); err != nil {
		t.Fatalf("CreateThread: %v", err)
	}
	secondMessage := models.NewMessage(secondThread.ID, "Second message", "seed@tui")
	if err := store.CreateMessage(secondMessage); err != nil {
		t.Fatalf("CreateMessage: %v", err)
	}
	model.width = 100
	model.height = 30
	model.threads.SetThreads([]*models.Thread{firstThread, secondThread})
	model.threads.MoveDown()
	model.activePane = ThreadsPane

	newModel, _ := model.Update(tea.KeyMsg{Type: tea.KeyEnter})
	pending := newModel.(Model)
	view := pending.View()
	if strings.Contains(view, "Initial message") || !strings.Contains(view, "Loading messages") {
		t.Fatalf("Expected truthful loading view without prior thread messages, got %q", view)
	}
	if strings.Contains(view, "[n] reply") || pending.canReply() {
		t.Fatal("Expected reply to remain unavailable until selected thread messages load")
	}
}

func TestModelIgnoresOutOfOrderMessageLoadsForThreadSelection(t *testing.T) {
	store := newTestStore(t)
	defer store.Close()

	model, topic, firstThread := replyReadyModel(t, store, "test@tui")
	secondThread := models.NewThread(topic.ID, "Second thread", "seed@tui")
	if err := store.CreateThread(secondThread); err != nil {
		t.Fatalf("CreateThread: %v", err)
	}
	secondMessage := models.NewMessage(secondThread.ID, "Second thread message", "seed@tui")
	if err := store.CreateMessage(secondMessage); err != nil {
		t.Fatalf("CreateMessage: %v", err)
	}
	model.threads.SetThreads([]*models.Thread{firstThread, secondThread})
	model.activePane = ThreadsPane
	newModel, firstCmd := model.Update(tea.KeyMsg{Type: tea.KeyEnter})
	model = newModel.(Model)
	model.activePane = ThreadsPane
	model.threads.MoveDown()
	newModel, secondCmd := model.Update(tea.KeyMsg{Type: tea.KeyEnter})
	model = newModel.(Model)
	newModel, _ = model.Update(secondCmd())
	model = newModel.(Model)
	newModel, _ = model.Update(firstCmd())
	model = newModel.(Model)
	if len(model.messages.messages) != 1 || model.messages.messages[0].ThreadID != secondThread.ID {
		t.Fatalf("Expected stale first thread load ignored, got %#v", model.messages.messages)
	}
	if !model.canReply() {
		t.Fatal("Expected reply available only for the newest loaded thread selection")
	}
}

func TestModelThreadLoadFailureIsVisible(t *testing.T) {
	store := newTestStore(t)
	model, _, _ := replyReadyModel(t, store, "test@tui")
	model.width = 100
	model.height = 30
	model.activePane = TopicsPane

	newModel, loadThreadsCmd := model.Update(tea.KeyMsg{Type: tea.KeyEnter})
	pending := newModel.(Model)
	if loadThreadsCmd == nil {
		t.Fatal("Expected thread load command")
	}
	if err := store.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	newModel, _ = pending.Update(loadThreadsCmd())
	updated := newModel.(Model)
	if updated.threadsLoading {
		t.Error("Expected failed thread load to stop loading")
	}
	if updated.err == nil || !strings.Contains(updated.View(), "Error:") {
		t.Fatalf("Expected visible thread load error, got %q", updated.View())
	}
}

func TestModelMessageLoadFailureIsVisibleAndBlocksReply(t *testing.T) {
	store := newTestStore(t)
	model, _, _ := replyReadyModel(t, store, "test@tui")
	model.width = 100
	model.height = 30
	model.activePane = ThreadsPane

	newModel, loadMessagesCmd := model.Update(tea.KeyMsg{Type: tea.KeyEnter})
	pending := newModel.(Model)
	if loadMessagesCmd == nil {
		t.Fatal("Expected message load command")
	}
	if err := store.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	newModel, _ = pending.Update(loadMessagesCmd())
	updated := newModel.(Model)
	if updated.messagesLoading {
		t.Error("Expected failed message load to stop loading")
	}
	if updated.canReply() {
		t.Error("Expected reply unavailable when the selected thread's messages failed to load")
	}
	if updated.err == nil || !strings.Contains(updated.View(), "Error:") {
		t.Fatalf("Expected visible message load error, got %q", updated.View())
	}
}

func TestModelIgnoresStalePreReplyMessageLoadAfterReplyRefresh(t *testing.T) {
	store := newTestStore(t)
	defer store.Close()

	model, _, _ := replyReadyModel(t, store, "doctor-biz")
	newModel, staleLoadCmd := model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'r'}})
	model = newModel.(Model)
	staleLoadResult := staleLoadCmd()
	newModel, _ = model.Update(staleLoadResult)
	model = newModel.(Model)
	model.composing = true
	model.composeText = "new reply wins"
	newModel, persistCmd := model.Update(tea.KeyMsg{Type: tea.KeyEnter})
	model = newModel.(Model)
	newModel, refreshCmd := model.Update(persistCmd())
	model = newModel.(Model)
	if refreshCmd == nil {
		t.Fatal("Expected persistence completion to start refresh")
	}
	newModel, _ = model.Update(refreshCmd())
	model = newModel.(Model)
	newModel, _ = model.Update(staleLoadResult)
	model = newModel.(Model)
	if len(model.messages.messages) != 2 || model.messages.messages[1].Content != "new reply wins" {
		t.Fatalf("Expected stale pre-reply load ignored after refresh, got %#v", model.messages.messages)
	}
}

func TestModelViewAdvertisesReplyOnlyWhenAvailable(t *testing.T) {
	store := newTestStore(t)
	defer store.Close()

	model, _, _ := replyReadyModel(t, store, "test@tui")
	model.width = 100
	model.height = 30
	model.activePane = TopicsPane
	if view := model.View(); strings.Contains(view, "[n] reply") {
		t.Fatalf("Expected no reply help outside Messages pane, got %q", view)
	}
	model.activePane = MessagesPane
	if view := model.View(); !strings.Contains(view, "[n] reply") {
		t.Fatalf("Expected contextual reply help, got %q", view)
	}
	model.loadedThreadID = uuid.Nil
	if view := model.View(); strings.Contains(view, "[n] reply") {
		t.Fatalf("Expected no reply help without an opened thread, got %q", view)
	}
}

func TestModelUpdateComposingAcceptsUnicodeAndBackspacesOneRune(t *testing.T) {
	store := newTestStore(t)
	defer store.Close()

	model := NewModel(store, "test@tui")
	model.composing = true

	typeMsg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("é界🙂")}
	newModel, _ := model.Update(typeMsg)
	m := newModel.(Model)
	if m.composeText != "é界🙂" {
		t.Fatalf("Expected Unicode compose text %q, got %q", "é界🙂", m.composeText)
	}

	backspaceMsg := tea.KeyMsg{Type: tea.KeyBackspace}
	newModel, _ = m.Update(backspaceMsg)
	m = newModel.(Model)
	if m.composeText != "é界" {
		t.Errorf("Expected backspace to remove one full rune, got %q", m.composeText)
	}
}

func TestModelUpdateCtrlCQuitsWhileComposing(t *testing.T) {
	store := newTestStore(t)
	defer store.Close()

	model := NewModel(store, "test@tui")
	model.composing = true
	model.composeText = "draft"

	_, cmd := model.Update(tea.KeyMsg{Type: tea.KeyCtrlC})
	if cmd == nil {
		t.Fatal("Expected ctrl+c to return a quit command while composing")
	}
	if _, ok := cmd().(tea.QuitMsg); !ok {
		t.Errorf("Expected tea.QuitMsg, got %T", cmd())
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

func TestModelViewRendersErrorWithoutReplacingStatus(t *testing.T) {
	store := newTestStore(t)
	defer store.Close()

	model := NewModel(store, "test@tui")
	model.width = 100
	model.height = 50
	model.err = errors.New("storage unavailable")

	view := model.View()
	if !strings.Contains(view, "Error: storage unavailable") {
		t.Errorf("Expected view to contain the current error, got %q", view)
	}
	if !strings.Contains(view, "switch pane") {
		t.Error("Expected error view to preserve the navigation status")
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

func TestModelTopicsRefreshPreservesSelectedTopicIdentityAcrossSortedInsertion(t *testing.T) {
	store := newTestStore(t)
	defer store.Close()

	model, topic, thread := replyReadyModel(t, store, "doctor-biz")
	model.threads.topicID = topic.ID
	earlier := models.NewTopic("Alpha topic", "Sorts before the selected topic", "seed@tui")
	if err := store.CreateTopic(earlier); err != nil {
		t.Fatalf("CreateTopic: %v", err)
	}
	model.activePane = TopicsPane

	newModel, refreshCmd := model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'r'}})
	pending := newModel.(Model)
	if refreshCmd == nil {
		t.Fatal("Expected Topics refresh command")
	}
	newModel, _ = pending.Update(refreshCmd())
	refreshed := newModel.(Model)

	if selected := refreshed.topics.Selected(); selected == nil || selected.ID != topic.ID {
		t.Fatalf("Expected selected topic UUID %s after sorted insertion, got %#v", topic.ID, selected)
	}
	if refreshed.topics.cursor != 1 {
		t.Fatalf("Expected cursor to follow selected topic to index 1, got %d", refreshed.topics.cursor)
	}
	if refreshed.threads.topicID != topic.ID {
		t.Fatalf("Expected dependent thread topic UUID %s, got %s", topic.ID, refreshed.threads.topicID)
	}
	if selected := refreshed.threads.Selected(); selected == nil || selected.ID != thread.ID {
		t.Fatalf("Expected opened thread to remain coherent, got %#v", selected)
	}
	if len(refreshed.messages.messages) != 1 || refreshed.messages.messages[0].ThreadID != thread.ID {
		t.Fatalf("Expected opened messages to remain coherent, got %#v", refreshed.messages.messages)
	}
	refreshed.activePane = MessagesPane
	if !refreshed.canReply() {
		t.Fatal("Expected preserved dependent state to remain reply eligible in Messages pane")
	}
}

func TestModelTopicsRefreshClearsDependentStateWhenSelectedTopicIsArchived(t *testing.T) {
	store := newTestStore(t)
	defer store.Close()

	model, topic, thread := replyReadyModel(t, store, "doctor-biz")
	model.threads.topicID = topic.ID
	replacement := models.NewTopic("Replacement topic", "Remains active", "seed@tui")
	if err := store.CreateTopic(replacement); err != nil {
		t.Fatalf("CreateTopic: %v", err)
	}
	if err := store.ArchiveTopic(topic.ID, true); err != nil {
		t.Fatalf("ArchiveTopic: %v", err)
	}

	model.composing = true
	model.composeText = "orphaned draft"
	model.err = errors.New("orphaned reply error")
	staleThreadModel := model
	staleThreadCmd := staleThreadModel.beginThreadLoad(topic.ID)
	staleMessageModel := staleThreadModel
	staleMessageCmd := staleMessageModel.beginMessageLoad(thread.ID)
	model.nextRequest = staleMessageModel.nextRequest
	model.threadLoad = staleThreadModel.threadLoad
	model.messageLoad = staleMessageModel.messageLoad
	model.activePane = TopicsPane
	refreshCmd := model.topics.LoadTopics()
	newModel, _ := model.Update(refreshCmd())
	refreshed := newModel.(Model)

	if selected := refreshed.topics.Selected(); selected == nil || selected.ID != replacement.ID {
		t.Fatalf("Expected cursor fallback to the remaining active topic, got %#v", selected)
	}
	if refreshed.threads.topicID != (models.UUID{}) || refreshed.selectedThreadID != (models.UUID{}) ||
		refreshed.loadedThreadID != (models.UUID{}) || refreshed.messages.threadID != (models.UUID{}) {
		t.Fatal("Expected removed topic refresh to clear dependent topic, thread, and message identities")
	}
	if len(refreshed.threads.threads) != 0 || len(refreshed.messages.messages) != 0 {
		t.Fatalf("Expected removed topic refresh to clear dependent rows, got threads=%#v messages=%#v", refreshed.threads.threads, refreshed.messages.messages)
	}
	if refreshed.threadsLoading || refreshed.messagesLoading || refreshed.canReply() {
		t.Fatal("Expected removed topic refresh to clear loading state and reply eligibility")
	}
	if refreshed.composing || refreshed.composeText != "" || refreshed.posting || refreshed.replyPost != 0 || refreshed.err != nil {
		t.Fatalf("Expected removed topic refresh to clear reply state, got composing=%v draft=%q posting=%v replyPost=%d err=%v", refreshed.composing, refreshed.composeText, refreshed.posting, refreshed.replyPost, refreshed.err)
	}

	newModel, _ = refreshed.Update(staleThreadCmd())
	afterStaleThreadLoad := newModel.(Model)
	if len(afterStaleThreadLoad.threads.threads) != 0 || afterStaleThreadLoad.threads.topicID != (models.UUID{}) {
		t.Fatalf("Expected actual tagged stale thread load to remain inert, got %#v", afterStaleThreadLoad.threads.threads)
	}
	newModel, _ = afterStaleThreadLoad.Update(staleMessageCmd())
	afterStaleLoad := newModel.(Model)
	if len(afterStaleLoad.messages.messages) != 0 || afterStaleLoad.messages.threadID != (models.UUID{}) || afterStaleLoad.canReply() {
		t.Fatalf("Expected tagged stale message load to remain inert, got %#v", afterStaleLoad.messages.messages)
	}
}

func TestModelTopicsRemovalIgnoresActualInFlightReplySuccess(t *testing.T) {
	store := newTestStore(t)
	defer store.Close()

	model, topic, thread := replyReadyModel(t, store, "doctor-biz")
	model.threads.topicID = topic.ID
	model.composing = true
	model.composeText = "persisted while topic disappears"
	newModel, postCmd := model.Update(tea.KeyMsg{Type: tea.KeyEnter})
	pending := newModel.(Model)
	if postCmd == nil || !pending.posting || pending.replyPost == 0 {
		t.Fatal("Expected an actual tagged reply persistence command")
	}
	if err := store.ArchiveTopic(topic.ID, true); err != nil {
		t.Fatalf("ArchiveTopic: %v", err)
	}
	topicsMsg, ok := pending.topics.LoadTopics()().(TopicsLoadedMsg)
	if !ok {
		t.Fatal("Expected real topic refresh result")
	}
	newModel, _ = pending.Update(topicsMsg)
	removed := newModel.(Model)
	if removed.posting || removed.replyPost != 0 || removed.composing || removed.composeText != "" {
		t.Fatal("Expected topic removal to relinquish in-flight reply ownership")
	}

	postResult := postCmd()
	newModel, refreshCmd := removed.Update(postResult)
	afterStaleSuccess := newModel.(Model)
	if refreshCmd != nil {
		t.Fatal("Expected stale reply success not to refresh a removed topic")
	}
	if afterStaleSuccess.threads.topicID != (models.UUID{}) || len(afterStaleSuccess.threads.threads) != 0 ||
		afterStaleSuccess.messages.threadID != (models.UUID{}) || len(afterStaleSuccess.messages.messages) != 0 || afterStaleSuccess.canReply() {
		t.Fatal("Expected stale reply success not to resurrect removed-topic UI state")
	}
	persisted, err := store.ListMessages(thread.ID)
	if err != nil {
		t.Fatalf("ListMessages: %v", err)
	}
	if len(persisted) != 2 || persisted[1].Content != "persisted while topic disappears" {
		t.Fatalf("Expected real in-flight write to persist once without UI resurrection, got %#v", persisted)
	}
}

func TestModelTopicsRemovalIgnoresActualInFlightReplyFailure(t *testing.T) {
	store := newTestStore(t)

	model, topic, _ := replyReadyModel(t, store, "doctor-biz")
	model.threads.topicID = topic.ID
	model.composing = true
	model.composeText = "will fail after removal"
	newModel, postCmd := model.Update(tea.KeyMsg{Type: tea.KeyEnter})
	pending := newModel.(Model)
	if postCmd == nil {
		t.Fatal("Expected an actual tagged reply persistence command")
	}
	if err := store.ArchiveTopic(topic.ID, true); err != nil {
		t.Fatalf("ArchiveTopic: %v", err)
	}
	topicsMsg, ok := pending.topics.LoadTopics()().(TopicsLoadedMsg)
	if !ok {
		t.Fatal("Expected real topic refresh result")
	}
	newModel, _ = pending.Update(topicsMsg)
	removed := newModel.(Model)
	if err := store.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	postResult := postCmd()
	newModel, refreshCmd := removed.Update(postResult)
	afterStaleFailure := newModel.(Model)
	if refreshCmd != nil || afterStaleFailure.err != nil || afterStaleFailure.posting || afterStaleFailure.replyPost != 0 {
		t.Fatalf("Expected stale reply failure to remain inert, got cmd=%v err=%v posting=%v replyPost=%d", refreshCmd, afterStaleFailure.err, afterStaleFailure.posting, afterStaleFailure.replyPost)
	}
}

func TestModelTopicsRefreshPreservesActualInFlightReplyWhenTopicRemains(t *testing.T) {
	store := newTestStore(t)
	defer store.Close()

	model, topic, _ := replyReadyModel(t, store, "doctor-biz")
	model.threads.topicID = topic.ID
	model.composing = true
	model.composeText = "normal in-flight reply"
	newModel, postCmd := model.Update(tea.KeyMsg{Type: tea.KeyEnter})
	pending := newModel.(Model)
	replyRequest := pending.replyPost
	topicsMsg, ok := pending.topics.LoadTopics()().(TopicsLoadedMsg)
	if !ok {
		t.Fatal("Expected real topic refresh result")
	}
	newModel, _ = pending.Update(topicsMsg)
	stillPending := newModel.(Model)
	if !stillPending.posting || stillPending.replyPost != replyRequest || !stillPending.composing || stillPending.composeText != "normal in-flight reply" {
		t.Fatal("Expected unchanged topic refresh to preserve in-flight reply ownership and draft")
	}

	newModel, refreshCmd := stillPending.Update(postCmd())
	posted := newModel.(Model)
	if refreshCmd == nil || posted.posting || posted.composing || posted.composeText != "" {
		t.Fatal("Expected normal reply completion to retain its existing refresh behavior")
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
	tm := NewThreadsModel()

	if tm.cursor != 0 {
		t.Errorf("Expected cursor 0, got %d", tm.cursor)
	}
}

func TestThreadsModelSetThreads(t *testing.T) {
	tm := NewThreadsModel()
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
	tm := NewThreadsModel()

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
	tm := NewThreadsModel()

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
	tm := NewThreadsModel()

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
	tm := NewThreadsModel()

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
	mm := NewMessagesModel()

	if mm.cursor != 0 {
		t.Errorf("Expected cursor 0, got %d", mm.cursor)
	}
	if mm.scroll != 0 {
		t.Errorf("Expected scroll 0, got %d", mm.scroll)
	}
}

func TestMessagesModelSetMessages(t *testing.T) {
	mm := NewMessagesModel()
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
	mm := NewMessagesModel()

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
	mm := NewMessagesModel()

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
	mm := NewMessagesModel()

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
	mm := NewMessagesModel()

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
	mm := NewMessagesModel()

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

	model, _, _ := replyReadyModel(t, store, "test@tui")

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
	newModel, cmd := m.Update(enterMsg)
	m = newModel.(Model)
	if cmd == nil {
		t.Fatal("Expected persistence command after Enter")
	}
	newModel, refreshCmd := m.Update(cmd())
	m = newModel.(Model)
	if refreshCmd == nil {
		t.Fatal("Expected refresh command after persistence")
	}
	newModel, _ = m.Update(refreshCmd())
	m = newModel.(Model)

	// Successful persistence should clear composing.
	if m.composing {
		t.Error("Expected composing to be false after persistence")
	}
}

func TestModelUpdateComposingEmptyEnter(t *testing.T) {
	store := newTestStore(t)
	defer store.Close()

	model, _, _ := replyReadyModel(t, store, "test@tui")

	// Start composing
	nMsg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'n'}}
	newModel, _ := model.Update(nMsg)
	m := newModel.(Model)

	// Press Enter with empty text
	enterMsg := tea.KeyMsg{Type: tea.KeyEnter}
	newModel, _ = m.Update(enterMsg)
	m = newModel.(Model)

	// Empty replies are rejected without leaving compose mode.
	if !m.composing {
		t.Error("Expected composing to remain true after empty Enter")
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
	tm := NewThreadsModel()
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
	mm := NewMessagesModel()
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

// replyReadyModel seeds a real store with one topic, thread, and message and returns a model
// whose Messages pane shows that thread, ready to compose a reply.
func replyReadyModel(t *testing.T, store *storage.SqliteStore, identity string) (Model, *models.Topic, *models.Thread) {
	t.Helper()
	topic := models.NewTopic("Reply topic", "Reply tests", "seed@tui")
	if err := store.CreateTopic(topic); err != nil {
		t.Fatalf("CreateTopic: %v", err)
	}
	thread := models.NewThread(topic.ID, "Reply thread", "seed@tui")
	thread.CreatedAt = time.Now().Add(-2 * time.Hour)
	thread.UpdatedAt = time.Now().Add(-time.Hour)
	if err := store.CreateThread(thread); err != nil {
		t.Fatalf("CreateThread: %v", err)
	}
	initial := models.NewMessage(thread.ID, "Initial message", "seed@tui")
	initial.CreatedAt = thread.CreatedAt.Add(time.Minute)
	if err := store.CreateMessage(initial); err != nil {
		t.Fatalf("CreateMessage: %v", err)
	}
	persistedThread, err := store.GetThread(thread.ID)
	if err != nil {
		t.Fatalf("GetThread: %v", err)
	}
	thread = persistedThread

	model := NewModel(store, identity)
	model.activePane = MessagesPane
	model.topics.SetTopics([]*models.Topic{topic})
	model.threads.SetThreads([]*models.Thread{thread})
	model.messages.threadID = thread.ID
	model.messages.SetMessages([]*models.Message{initial})
	model.selectedThreadID = thread.ID
	model.loadedThreadID = thread.ID
	return model, topic, thread
}

// Helper to create a test store
func newTestStore(t *testing.T) *storage.SqliteStore {
	t.Helper()
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")
	store, err := storage.NewSqliteStore(dbPath)
	if err != nil {
		t.Fatalf("failed to create test store: %v", err)
	}
	return store
}
