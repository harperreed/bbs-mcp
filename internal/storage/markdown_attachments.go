// ABOUTME: Attachment CRUD operations for MarkdownStore
// ABOUTME: Stores attachment files and metadata in _attachments/<msg-prefix>/ directories

package storage

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/google/uuid"
	"github.com/harper/suite/mdstore"

	"github.com/harper/bbs/internal/models"
)

// CreateAttachment stores a new attachment on disk.
func (s *MarkdownStore) CreateAttachment(a *models.Attachment) error {
	return mdstore.WithLock(s.dataDir, func() error {
		// Find the thread that contains this message to determine topic name
		topicName, err := s.topicNameForMessage(a.MessageID)
		if err != nil {
			return fmt.Errorf("find topic for message: %w", err)
		}

		msgPrefix := a.MessageID.String()[:8]
		attDir := s.attachmentDirPath(topicName, msgPrefix)
		if err := mdstore.EnsureDir(attDir); err != nil {
			return fmt.Errorf("create attachment directory: %w", err)
		}

		// Resolve filename, handling collisions by prefixing with attachment ID
		storedFilename := a.Filename
		originalFilename := ""
		dataPath := filepath.Join(attDir, storedFilename)
		if _, err := os.Stat(dataPath); err == nil {
			// File already exists, make unique by prefixing with attachment ID
			originalFilename = a.Filename
			storedFilename = a.ID.String()[:8] + "-" + a.Filename
			dataPath = filepath.Join(attDir, storedFilename)
		}

		// Write attachment data file
		if err := mdstore.AtomicWrite(dataPath, a.Data); err != nil {
			return fmt.Errorf("write attachment data: %w", err)
		}

		// Write metadata file (store the resolved filename and original if munged)
		meta := attachmentMeta{
			ID:               a.ID.String(),
			MessageID:        a.MessageID.String(),
			Filename:         storedFilename,
			OriginalFilename: originalFilename,
			MimeType:         a.MimeType,
			CreatedAt:        mdstore.FormatTime(a.CreatedAt.UTC()),
		}
		metaPath := filepath.Join(attDir, storedFilename+".meta.yaml")
		if err := mdstore.WriteYAML(metaPath, &meta); err != nil {
			return fmt.Errorf("write attachment metadata: %w", err)
		}

		return nil
	})
}

// GetAttachment retrieves an attachment by ID.
func (s *MarkdownStore) GetAttachment(id uuid.UUID) (*models.Attachment, error) {
	entries, err := s.readTopics()
	if err != nil {
		return nil, err
	}

	for _, e := range entries {
		att, err := s.findAttachmentInTopic(e.Name, id)
		if err == nil {
			return att, nil
		}
	}

	return nil, fmt.Errorf("attachment not found: %s", id)
}

// ListAttachments returns all attachments for a message.
func (s *MarkdownStore) ListAttachments(messageID uuid.UUID) ([]*models.Attachment, error) {
	topicName, err := s.topicNameForMessage(messageID)
	if err != nil {
		return nil, fmt.Errorf("find topic for message: %w", err)
	}

	msgPrefix := messageID.String()[:8]
	attDir := s.attachmentDirPath(topicName, msgPrefix)

	dirEntries, err := os.ReadDir(attDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("read attachment directory: %w", err)
	}

	var attachments []*models.Attachment
	for _, de := range dirEntries {
		if de.IsDir() || !strings.HasSuffix(de.Name(), ".meta.yaml") {
			continue
		}

		metaPath := filepath.Join(attDir, de.Name())
		att, err := s.readAttachmentFromMeta(metaPath, attDir)
		if err != nil {
			continue
		}
		attachments = append(attachments, att)
	}

	return attachments, nil
}

// DeleteAttachment deletes an attachment from disk.
func (s *MarkdownStore) DeleteAttachment(id uuid.UUID) error {
	return mdstore.WithLock(s.dataDir, func() error {
		entries, err := s.readTopics()
		if err != nil {
			return err
		}

		for _, e := range entries {
			if err := s.deleteAttachmentInTopic(e.Name, id); err == nil {
				return nil
			}
		}

		return fmt.Errorf("attachment not found: %s", id)
	})
}

// topicNameForMessage finds the topic name that contains a given message.
func (s *MarkdownStore) topicNameForMessage(messageID uuid.UUID) (string, error) {
	entries, err := s.readTopics()
	if err != nil {
		return "", err
	}

	for _, e := range entries {
		topicDir := s.topicDirPath(e.Name)
		dirEntries, err := os.ReadDir(topicDir)
		if err != nil {
			continue
		}

		for _, de := range dirEntries {
			if de.IsDir() || !strings.HasSuffix(de.Name(), ".md") {
				continue
			}
			fp := filepath.Join(topicDir, de.Name())
			data, err := os.ReadFile(fp)
			if err != nil {
				continue
			}
			messages := parseThreadMessages(string(data))
			for _, msg := range messages {
				if msg.ID == messageID {
					return e.Name, nil
				}
			}
		}
	}

	return "", fmt.Errorf("message not found: %s", messageID)
}

// findAttachmentInTopic searches a topic's _attachments directories for an attachment by ID.
func (s *MarkdownStore) findAttachmentInTopic(topicName string, id uuid.UUID) (*models.Attachment, error) {
	attBase := filepath.Join(s.topicDirPath(topicName), "_attachments")
	msgDirs, err := os.ReadDir(attBase)
	if err != nil {
		return nil, err
	}

	for _, msgDir := range msgDirs {
		if !msgDir.IsDir() {
			continue
		}
		dirPath := filepath.Join(attBase, msgDir.Name())
		metaFiles, err := os.ReadDir(dirPath)
		if err != nil {
			continue
		}
		for _, mf := range metaFiles {
			if !strings.HasSuffix(mf.Name(), ".meta.yaml") {
				continue
			}
			metaPath := filepath.Join(dirPath, mf.Name())
			att, err := s.readAttachmentFromMeta(metaPath, dirPath)
			if err != nil {
				continue
			}
			if att.ID == id {
				return att, nil
			}
		}
	}

	return nil, fmt.Errorf("attachment not found: %s", id)
}

// deleteAttachmentInTopic deletes a specific attachment from a topic's _attachments.
func (s *MarkdownStore) deleteAttachmentInTopic(topicName string, id uuid.UUID) error {
	attBase := filepath.Join(s.topicDirPath(topicName), "_attachments")
	msgDirs, err := os.ReadDir(attBase)
	if err != nil {
		return err
	}

	for _, msgDir := range msgDirs {
		if !msgDir.IsDir() {
			continue
		}
		dirPath := filepath.Join(attBase, msgDir.Name())
		metaFiles, err := os.ReadDir(dirPath)
		if err != nil {
			continue
		}
		for _, mf := range metaFiles {
			if !strings.HasSuffix(mf.Name(), ".meta.yaml") {
				continue
			}
			metaPath := filepath.Join(dirPath, mf.Name())
			meta, err := readAttachmentMeta(metaPath)
			if err != nil {
				continue
			}
			if meta.ID == id.String() {
				// Delete data file and meta file
				dataPath := filepath.Join(dirPath, meta.Filename)
				os.Remove(dataPath)
				os.Remove(metaPath)
				return nil
			}
		}
	}

	return fmt.Errorf("attachment not found: %s", id)
}

// readAttachmentFromMeta reads an attachment from its metadata file.
func (s *MarkdownStore) readAttachmentFromMeta(metaPath, dir string) (*models.Attachment, error) {
	meta, err := readAttachmentMeta(metaPath)
	if err != nil {
		return nil, err
	}

	id, err := uuid.Parse(meta.ID)
	if err != nil {
		return nil, fmt.Errorf("parse attachment ID %q: %w", meta.ID, err)
	}
	messageID, err := uuid.Parse(meta.MessageID)
	if err != nil {
		return nil, fmt.Errorf("parse attachment message ID %q: %w", meta.MessageID, err)
	}
	createdAt, err := mdstore.ParseTime(meta.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("parse attachment created_at %q: %w", meta.CreatedAt, err)
	}

	// Read data file
	dataPath := filepath.Join(dir, meta.Filename)
	data, err := os.ReadFile(dataPath)
	if err != nil {
		return nil, fmt.Errorf("read attachment data: %w", err)
	}

	// Use original filename if available (collision case), otherwise use stored filename
	displayFilename := meta.Filename
	if meta.OriginalFilename != "" {
		displayFilename = meta.OriginalFilename
	}

	return &models.Attachment{
		ID:        id,
		MessageID: messageID,
		Filename:  displayFilename,
		MimeType:  meta.MimeType,
		Data:      data,
		CreatedAt: createdAt,
	}, nil
}

// readAttachmentMeta reads an attachment metadata YAML file.
func readAttachmentMeta(path string) (*attachmentMeta, error) {
	var meta attachmentMeta
	if err := mdstore.ReadYAML(path, &meta); err != nil {
		return nil, err
	}
	// ReadYAML returns nil for missing files, but we need an error for missing metadata
	if meta.ID == "" {
		return nil, fmt.Errorf("attachment metadata not found or empty: %s", path)
	}
	return &meta, nil
}
