// ABOUTME: Attachment CRUD operations for MarkdownStore
// ABOUTME: Stores attachment files and metadata in contained per-message directories

package storage

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/google/uuid"
	"github.com/harperreed/mdstore"

	"github.com/harper/bbs/internal/models"
)

var errLegacyAttachmentCollision = errors.New("legacy attachment prefix collision")

// validateFilesystemName rejects path syntax while preserving ordinary filename characters.
func validateFilesystemName(name string, kind string) error {
	if name == "" || name == "." || name == ".." || filepath.IsAbs(name) || strings.ContainsAny(name, `/\`) {
		return fmt.Errorf("invalid %s %q", kind, name)
	}
	return nil
}

// containedPath resolves symlinks in the existing path prefix and requires board-root containment.
func containedPath(boardRoot string, candidate string) (string, error) {
	lexicalRoot, err := filepath.Abs(boardRoot)
	if err != nil {
		return "", fmt.Errorf("make board root absolute: %w", err)
	}
	resolvedRoot, err := filepath.EvalSymlinks(lexicalRoot)
	if err != nil {
		return "", fmt.Errorf("resolve board root: %w", err)
	}
	resolvedRoot, err = filepath.Abs(resolvedRoot)
	if err != nil {
		return "", fmt.Errorf("make resolved board root absolute: %w", err)
	}
	candidate, err = filepath.Abs(candidate)
	if err != nil {
		return "", fmt.Errorf("make candidate path absolute: %w", err)
	}
	if !pathWithinRoot(lexicalRoot, candidate) {
		return "", fmt.Errorf("path %q is outside board root", candidate)
	}
	resolved, err := resolveExistingPathPrefix(candidate)
	if err != nil {
		return "", err
	}
	if !pathWithinRoot(resolvedRoot, resolved) {
		return "", fmt.Errorf("resolved path %q is outside board root", resolved)
	}
	return candidate, nil
}

func pathWithinRoot(root string, candidate string) bool {
	relative, err := filepath.Rel(root, candidate)
	return err == nil && relative != ".." && !strings.HasPrefix(relative, ".."+string(filepath.Separator))
}

func resolveExistingPathPrefix(path string) (string, error) {
	current := path
	var missing []string
	for {
		resolved, err := filepath.EvalSymlinks(current)
		if err == nil {
			for i := len(missing) - 1; i >= 0; i-- {
				resolved = filepath.Join(resolved, missing[i])
			}
			return filepath.Abs(resolved)
		}
		if !os.IsNotExist(err) {
			return "", fmt.Errorf("resolve path %q: %w", path, err)
		}
		parent := filepath.Dir(current)
		if parent == current {
			return "", fmt.Errorf("resolve existing path prefix for %q", path)
		}
		missing = append(missing, filepath.Base(current))
		current = parent
	}
}

func safeNamedPath(boardRoot string, parent string, name string, kind string) (string, error) {
	if err := validateFilesystemName(name, kind); err != nil {
		return "", err
	}
	return containedPath(boardRoot, filepath.Join(parent, name))
}

func (s *MarkdownStore) safeTopicDirPath(topicName string) (string, error) {
	return safeNamedPath(s.dataDir, s.dataDir, topicName, "topic name")
}

func (s *MarkdownStore) safeAttachmentDirPath(topicName string, messageDirName string) (string, error) {
	if _, err := s.safeTopicDirPath(topicName); err != nil {
		return "", err
	}
	if err := validateFilesystemName(messageDirName, "message attachment directory"); err != nil {
		return "", err
	}
	return containedPath(s.dataDir, s.attachmentDirPath(topicName, messageDirName))
}

// CreateAttachment stores a new attachment on disk.
func (s *MarkdownStore) CreateAttachment(a *models.Attachment) error {
	return mdstore.WithLock(s.dataDir, func() error {
		if err := s.requireAvailableAttachmentID(a.ID); err != nil {
			return err
		}

		// Find the thread that contains this message to determine topic name
		topicName, err := s.topicNameForMessage(a.MessageID)
		if err != nil {
			return fmt.Errorf("find topic for message: %w", err)
		}

		attDir, err := s.safeAttachmentDirPath(topicName, a.MessageID.String())
		if err != nil {
			return fmt.Errorf("resolve attachment directory: %w", err)
		}
		dataPath, err := safeNamedPath(s.dataDir, attDir, a.Filename, "attachment filename")
		if err != nil {
			return err
		}
		_, dirErr := os.Stat(attDir)
		dirCreated := os.IsNotExist(dirErr)
		if err := mdstore.EnsureDir(attDir); err != nil {
			return fmt.Errorf("create attachment directory: %w", err)
		}

		// Resolve filename, handling collisions by prefixing with attachment ID
		storedFilename := a.Filename
		originalFilename := ""
		if _, err := os.Stat(dataPath); err == nil {
			// File already exists, make unique by prefixing with attachment ID
			originalFilename = a.Filename
			storedFilename = a.ID.String() + "-" + a.Filename
			dataPath, err = safeNamedPath(s.dataDir, attDir, storedFilename, "stored attachment filename")
			if err != nil {
				return err
			}
		}
		metaPath, err := safeNamedPath(s.dataDir, attDir, storedFilename+".meta.yaml", "attachment metadata filename")
		if err != nil {
			return err
		}
		dataBefore, dataExisted, err := readFileSnapshot(dataPath)
		if err != nil {
			return err
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
		if err := mdstore.WriteYAML(metaPath, &meta); err != nil {
			rollbackErr := restoreFileSnapshot(dataPath, dataBefore, dataExisted)
			if dirCreated {
				_ = os.Remove(attDir)
			}
			if rollbackErr != nil {
				return fmt.Errorf("write attachment metadata: %w; rollback data: %w", err, rollbackErr)
			}
			return fmt.Errorf("write attachment metadata: %w", err)
		}

		return nil
	})
}

func (s *MarkdownStore) requireAvailableAttachmentID(id uuid.UUID) error {
	locations, err := s.attachmentLocations(id)
	if err != nil {
		return err
	}
	if len(locations) > 0 {
		return fmt.Errorf("insert attachment: duplicate attachment ID %s", id)
	}
	return nil
}

// GetAttachment retrieves an attachment by ID.
func (s *MarkdownStore) GetAttachment(id uuid.UUID) (*models.Attachment, error) {
	location, err := s.uniqueAttachmentLocation(id)
	if err != nil {
		return nil, err
	}
	if err := s.validateAttachmentLocationOwner(location.topicName, location.meta); err != nil {
		return nil, err
	}
	return s.attachmentFromMeta(location.meta.metadata, location.meta.directoryPath)
}

// ListAttachments returns all attachments for a message.
func (s *MarkdownStore) ListAttachments(messageID uuid.UUID) ([]*models.Attachment, error) {
	topicName, err := s.topicNameForMessage(messageID)
	if err != nil {
		return nil, fmt.Errorf("find topic for message: %w", err)
	}

	attachmentDirs, err := s.attachmentDirsForMessage(topicName, messageID)
	if err != nil {
		return nil, err
	}
	var attachments []*models.Attachment
	for _, attDir := range attachmentDirs {
		dirEntries, err := os.ReadDir(attDir)
		if err != nil {
			return nil, fmt.Errorf("read attachment directory: %w", err)
		}
		for _, de := range dirEntries {
			if de.IsDir() || !strings.HasSuffix(de.Name(), ".meta.yaml") {
				continue
			}
			metaPath, err := safeNamedPath(s.dataDir, attDir, de.Name(), "attachment metadata filename")
			if err != nil {
				return nil, err
			}
			att, err := s.readAttachmentFromMeta(metaPath, attDir)
			if err != nil {
				return nil, err
			}
			if att.MessageID != messageID {
				return nil, fmt.Errorf("attachment metadata message %s does not match directory owner %s", att.MessageID, messageID)
			}
			attachments = append(attachments, att)
		}
	}
	sort.Slice(attachments, func(i, j int) bool {
		if attachments[i].CreatedAt.Equal(attachments[j].CreatedAt) {
			return attachments[i].ID.String() < attachments[j].ID.String()
		}
		return attachments[i].CreatedAt.Before(attachments[j].CreatedAt)
	})

	return attachments, nil
}

// DeleteAttachment deletes an attachment from disk.
func (s *MarkdownStore) DeleteAttachment(id uuid.UUID) error {
	return mdstore.WithLock(s.dataDir, func() error {
		location, err := s.uniqueAttachmentLocation(id)
		if err != nil {
			return err
		}
		return s.deleteAttachmentLocation(*location)
	})
}

// topicNameForMessage finds the topic name that contains a given message.
func (s *MarkdownStore) topicNameForMessage(messageID uuid.UUID) (string, error) {
	location, err := s.uniqueMessageLocation(messageID)
	if err != nil {
		return "", err
	}
	return location.topicName, nil
}

func (s *MarkdownStore) attachmentDirsForMessage(topicName string, messageID uuid.UUID) ([]string, error) {
	fullDir, err := s.safeAttachmentDirPath(topicName, messageID.String())
	if err != nil {
		return nil, err
	}
	var dirs []string
	if _, err := os.Stat(fullDir); err == nil {
		dirs = append(dirs, fullDir)
	} else if !os.IsNotExist(err) {
		return nil, fmt.Errorf("inspect attachment directory: %w", err)
	}

	legacyPrefix := messageID.String()[:8]
	legacyDir, err := s.safeAttachmentDirPath(topicName, legacyPrefix)
	if err != nil {
		return nil, err
	}
	if _, err := os.Stat(legacyDir); err == nil {
		if err := s.requireUnambiguousLegacyPrefix(topicName, legacyPrefix); err != nil {
			return nil, err
		}
		dirs = append(dirs, legacyDir)
	} else if !os.IsNotExist(err) {
		return nil, fmt.Errorf("inspect legacy attachment directory: %w", err)
	}
	return dirs, nil
}

func (s *MarkdownStore) requireUnambiguousLegacyPrefix(topicName string, prefix string) error {
	_, err := s.legacyPrefixOwner(topicName, prefix)
	return err
}

func (s *MarkdownStore) legacyPrefixOwner(topicName string, prefix string) (uuid.UUID, error) {
	messageIDs, err := s.messageIDsInTopic(topicName)
	if err != nil {
		return uuid.Nil, err
	}
	var owners []uuid.UUID
	for _, messageID := range messageIDs {
		if strings.HasPrefix(messageID.String(), prefix) {
			owners = append(owners, messageID)
		}
	}
	if len(owners) > 1 {
		return uuid.Nil, fmt.Errorf("%w %q belongs to %d messages", errLegacyAttachmentCollision, prefix, len(owners))
	}
	if len(owners) == 0 {
		return uuid.Nil, fmt.Errorf("legacy attachment prefix %q has no message owner", prefix)
	}
	return owners[0], nil
}

func (s *MarkdownStore) messageIDsInTopic(topicName string) ([]uuid.UUID, error) {
	topicDir, err := s.safeTopicDirPath(topicName)
	if err != nil {
		return nil, err
	}
	dirEntries, err := os.ReadDir(topicDir)
	if err != nil {
		return nil, fmt.Errorf("read topic directory: %w", err)
	}
	var messageIDs []uuid.UUID
	for _, de := range dirEntries {
		if de.IsDir() || !strings.HasSuffix(de.Name(), ".md") {
			continue
		}
		threadPath, err := containedPath(s.dataDir, filepath.Join(topicDir, de.Name()))
		if err != nil {
			return nil, err
		}
		data, err := os.ReadFile(threadPath)
		if err != nil {
			return nil, fmt.Errorf("read thread file: %w", err)
		}
		messages, err := parseThreadMessages(string(data))
		if err != nil {
			return nil, fmt.Errorf("parse thread messages for attachment ownership: %w", err)
		}
		for _, message := range messages {
			messageIDs = append(messageIDs, message.ID)
		}
	}
	return messageIDs, nil
}

type attachmentMetaLocation struct {
	directoryName string
	directoryPath string
	metadataPath  string
	metadata      *attachmentMeta
}

type attachmentLocation struct {
	topicName string
	meta      attachmentMetaLocation
}

func (s *MarkdownStore) attachmentLocations(id uuid.UUID) ([]attachmentLocation, error) {
	entries, err := s.readTopics()
	if err != nil {
		return nil, err
	}
	var matches []attachmentLocation
	for _, topic := range entries {
		locations, err := s.attachmentMetaLocations(topic.Name)
		if err != nil {
			return nil, err
		}
		for _, location := range locations {
			storedID, err := uuid.Parse(location.metadata.ID)
			if err != nil {
				return nil, fmt.Errorf("parse attachment ID in %s: %w", location.metadataPath, err)
			}
			if _, err := uuid.Parse(location.metadata.MessageID); err != nil {
				return nil, fmt.Errorf("parse attachment message ID in %s: %w", location.metadataPath, err)
			}
			if storedID == id {
				matches = append(matches, attachmentLocation{topicName: topic.Name, meta: location})
			}
		}
	}
	return matches, nil
}

func (s *MarkdownStore) uniqueAttachmentLocation(id uuid.UUID) (*attachmentLocation, error) {
	locations, err := s.attachmentLocations(id)
	if err != nil {
		return nil, err
	}
	if len(locations) > 1 {
		return nil, fmt.Errorf("ambiguous attachment ID %s: UUID collision across %d metadata files", id, len(locations))
	}
	if len(locations) == 0 {
		return nil, fmt.Errorf("attachment not found: %s", id)
	}
	return &locations[0], nil
}

func (s *MarkdownStore) attachmentMetaLocations(topicName string) ([]attachmentMetaLocation, error) {
	topicDir, err := s.safeTopicDirPath(topicName)
	if err != nil {
		return nil, err
	}
	attBase, err := containedPath(s.dataDir, filepath.Join(topicDir, "_attachments"))
	if err != nil {
		return nil, err
	}
	msgDirs, err := os.ReadDir(attBase)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	var locations []attachmentMetaLocation
	for _, msgDir := range msgDirs {
		if !msgDir.IsDir() {
			continue
		}
		dirPath, err := containedPath(s.dataDir, filepath.Join(attBase, msgDir.Name()))
		if err != nil {
			return nil, err
		}
		metaFiles, err := os.ReadDir(dirPath)
		if err != nil {
			return nil, fmt.Errorf("read attachment metadata directory %s: %w", dirPath, err)
		}
		for _, mf := range metaFiles {
			if !strings.HasSuffix(mf.Name(), ".meta.yaml") {
				continue
			}
			metaPath, err := safeNamedPath(s.dataDir, dirPath, mf.Name(), "attachment metadata filename")
			if err != nil {
				return nil, err
			}
			meta, err := readAttachmentMeta(metaPath)
			if err != nil {
				return nil, fmt.Errorf("read attachment metadata %s: %w", metaPath, err)
			}
			locations = append(locations, attachmentMetaLocation{
				directoryName: msgDir.Name(),
				directoryPath: dirPath,
				metadataPath:  metaPath,
				metadata:      meta,
			})
		}
	}
	return locations, nil
}

func (s *MarkdownStore) deleteAttachmentLocation(location attachmentLocation) error {
	if err := s.validateAttachmentLocationOwner(location.topicName, location.meta); err != nil {
		return err
	}
	dataPath, err := safeNamedPath(s.dataDir, location.meta.directoryPath, location.meta.metadata.Filename, "attachment filename")
	if err != nil {
		return err
	}
	metadata, err := os.ReadFile(location.meta.metadataPath)
	if err != nil {
		return fmt.Errorf("snapshot attachment metadata: %w", err)
	}
	if err := os.Remove(location.meta.metadataPath); err != nil {
		return fmt.Errorf("remove attachment metadata: %w", err)
	}
	if err := os.Remove(dataPath); err != nil {
		removeErr := fmt.Errorf("remove attachment data: %w", err)
		// Metadata is always written through mdstore.AtomicWrite, so rewriting it restores its mode too.
		if restoreErr := mdstore.AtomicWrite(location.meta.metadataPath, metadata); restoreErr != nil {
			return errors.Join(removeErr, fmt.Errorf("restore attachment metadata: %w", restoreErr))
		}
		return removeErr
	}
	return nil
}

func (s *MarkdownStore) validateAttachmentLocationOwner(topicName string, location attachmentMetaLocation) error {
	metadataOwner, err := uuid.Parse(location.metadata.MessageID)
	if err != nil {
		return fmt.Errorf("parse attachment message ID: %w", err)
	}
	var directoryOwner uuid.UUID
	if len(location.directoryName) == 8 {
		directoryOwner, err = s.legacyPrefixOwner(topicName, location.directoryName)
	} else {
		directoryOwner, err = uuid.Parse(location.directoryName)
	}
	if err != nil {
		return err
	}
	if metadataOwner != directoryOwner {
		return fmt.Errorf("attachment metadata message %s does not match directory owner %s", metadataOwner, directoryOwner)
	}
	return nil
}

// readAttachmentFromMeta reads an attachment from its metadata file.
func (s *MarkdownStore) readAttachmentFromMeta(metaPath, dir string) (*models.Attachment, error) {
	meta, err := readAttachmentMeta(metaPath)
	if err != nil {
		return nil, err
	}
	return s.attachmentFromMeta(meta, dir)
}

// attachmentFromMeta builds an attachment from parsed metadata and the data file it names in dir.
func (s *MarkdownStore) attachmentFromMeta(meta *attachmentMeta, dir string) (*models.Attachment, error) {
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
	dataPath, err := safeNamedPath(s.dataDir, dir, meta.Filename, "attachment filename")
	if err != nil {
		return nil, err
	}
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

func readFileSnapshot(path string) ([]byte, bool, error) {
	data, err := os.ReadFile(path)
	if err == nil {
		return data, true, nil
	}
	if os.IsNotExist(err) {
		return nil, false, nil
	}
	return nil, false, fmt.Errorf("snapshot file %s: %w", path, err)
}

func restoreFileSnapshot(path string, data []byte, existed bool) error {
	if existed {
		return mdstore.AtomicWrite(path, data)
	}
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

func preflightAttachmentRemoval(dirs []string) error {
	for _, dir := range dirs {
		parentInfo, err := os.Stat(filepath.Dir(dir))
		if err != nil {
			return fmt.Errorf("inspect attachment parent before removal: %w", err)
		}
		if parentInfo.Mode().Perm()&0300 != 0300 {
			return fmt.Errorf("attachment parent %q is not writable", filepath.Dir(dir))
		}
		err = filepath.WalkDir(dir, func(path string, entry os.DirEntry, walkErr error) error {
			if walkErr != nil {
				return walkErr
			}
			if !entry.IsDir() {
				return nil
			}
			info, err := entry.Info()
			if err != nil {
				return err
			}
			if info.Mode().Perm()&0300 != 0300 {
				return fmt.Errorf("attachment directory %q is not writable", path)
			}
			return nil
		})
		if err != nil {
			return fmt.Errorf("preflight attachment removal: %w", err)
		}
	}
	return nil
}
