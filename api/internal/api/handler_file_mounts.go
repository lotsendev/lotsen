package api

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/lotsendev/lotsen/store"
)

const (
	defaultManagedFileMode = 0o644
	maxFileMountContentLen = 64 * 1024
	maxFileMountTotalLen   = 512 * 1024
)

type fileMountRequest struct {
	Source   string `json:"source"`
	Target   string `json:"target"`
	Content  string `json:"content"`
	UID      *int   `json:"uid,omitempty"`
	GID      *int   `json:"gid,omitempty"`
	FileMode string `json:"file_mode,omitempty"`
	ReadOnly bool   `json:"read_only,omitempty"`
}

type managedFileSettings struct {
	hasUID      bool
	uid         int
	hasGID      bool
	gid         int
	hasFileMode bool
	fileMode    os.FileMode
}

func managedFilesBaseDirFromEnv() string {
	if dir := strings.TrimSpace(os.Getenv("LOTSEN_MANAGED_FILES_DIR")); dir != "" {
		return filepath.Clean(dir)
	}
	return "/var/lib/lotsen/files"
}

func resolveFileMounts(deploymentID string, mounts []fileMountRequest) ([]store.FileMount, error) {
	if mounts == nil {
		return []store.FileMount{}, nil
	}

	resolved := make([]store.FileMount, 0, len(mounts))
	seenSources := make(map[string]struct{}, len(mounts))
	seenTargets := make(map[string]struct{}, len(mounts))
	totalBytes := 0

	for _, mount := range mounts {
		source := strings.TrimSpace(mount.Source)
		if !managedVolumeNamePattern.MatchString(source) {
			return nil, fmt.Errorf("file source names must match %q", managedVolumeNamePattern.String())
		}
		if _, exists := seenSources[source]; exists {
			return nil, fmt.Errorf("file source %q is already used", source)
		}
		seenSources[source] = struct{}{}

		target, err := cleanAbsolutePath(mount.Target)
		if err != nil {
			return nil, fmt.Errorf("file target must be an absolute path")
		}
		if _, exists := seenTargets[target]; exists {
			return nil, fmt.Errorf("file target %q is already used", target)
		}
		seenTargets[target] = struct{}{}

		if !utf8.ValidString(mount.Content) {
			return nil, fmt.Errorf("file content for %q must be valid UTF-8 text", source)
		}
		contentBytes := len([]byte(mount.Content))
		if contentBytes > maxFileMountContentLen {
			return nil, fmt.Errorf("file content for %q exceeds %d bytes", source, maxFileMountContentLen)
		}
		totalBytes += contentBytes
		if totalBytes > maxFileMountTotalLen {
			return nil, fmt.Errorf("total file content exceeds %d bytes", maxFileMountTotalLen)
		}

		settings, err := managedFileSettingsFromRequest(mount)
		if err != nil {
			return nil, err
		}

		if _, err := ensureManagedFileWithSettings(deploymentID, source, mount.Content, settings); err != nil {
			return nil, err
		}

		resolved = append(resolved, store.FileMount{
			Source:   source,
			Target:   target,
			Content:  mount.Content,
			UID:      mount.UID,
			GID:      mount.GID,
			FileMode: strings.TrimSpace(mount.FileMode),
			ReadOnly: mount.ReadOnly,
		})
	}

	return resolved, nil
}

func managedFileSettingsFromRequest(mount fileMountRequest) (managedFileSettings, error) {
	settings := managedFileSettings{}

	if mount.UID != nil {
		if *mount.UID < 0 {
			return managedFileSettings{}, fmt.Errorf("uid must be >= 0")
		}
		settings.hasUID = true
		settings.uid = *mount.UID
	}

	if mount.GID != nil {
		if *mount.GID < 0 {
			return managedFileSettings{}, fmt.Errorf("gid must be >= 0")
		}
		settings.hasGID = true
		settings.gid = *mount.GID
	}

	rawFileMode := strings.TrimSpace(mount.FileMode)
	if rawFileMode != "" {
		parsed, err := strconv.ParseUint(rawFileMode, 8, 32)
		if err != nil || parsed > 0o777 {
			return managedFileSettings{}, fmt.Errorf("file_mode must be an octal permission between 0000 and 0777")
		}
		settings.hasFileMode = true
		settings.fileMode = os.FileMode(parsed)
	}

	return settings, nil
}

func ensureManagedFileWithSettings(deploymentID, source, content string, settings managedFileSettings) (string, error) {
	base := managedFilesBaseDirFromEnv()
	if !filepath.IsAbs(base) {
		return "", fmt.Errorf("managed files base path must be absolute")
	}

	cleanBase := filepath.Clean(base)
	filePath := filepath.Join(cleanBase, deploymentID, source)
	if !isPathWithinBase(cleanBase, filePath) {
		return "", fmt.Errorf("managed file path escapes configured base directory")
	}

	parent := filepath.Dir(filePath)
	if err := os.MkdirAll(parent, 0o755); err != nil {
		return "", fmt.Errorf("create managed file parent directory: %w", err)
	}

	existed := true
	if info, err := os.Lstat(filePath); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			existed = false
		} else {
			return "", fmt.Errorf("stat managed file: %w", err)
		}
	} else if info.Mode()&os.ModeSymlink != 0 {
		return "", fmt.Errorf("managed file path must not be a symlink")
	}

	f, err := os.OpenFile(filePath, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o600)
	if err != nil {
		return "", fmt.Errorf("open managed file: %w", err)
	}
	if _, err := f.WriteString(content); err != nil {
		_ = f.Close()
		return "", fmt.Errorf("write managed file: %w", err)
	}
	if err := f.Close(); err != nil {
		return "", fmt.Errorf("close managed file: %w", err)
	}

	if err := applyManagedFileSettings(filePath, settings, existed); err != nil {
		return "", err
	}

	return filePath, nil
}

func applyManagedFileSettings(filePath string, settings managedFileSettings, existed bool) error {
	if !settings.hasFileMode && !existed {
		settings.hasFileMode = true
		settings.fileMode = defaultManagedFileMode
	}

	if settings.hasUID || settings.hasGID {
		uid := -1
		if settings.hasUID {
			uid = settings.uid
		}
		gid := -1
		if settings.hasGID {
			gid = settings.gid
		}
		if err := os.Chown(filePath, uid, gid); err != nil {
			return fmt.Errorf("set managed file ownership: %w", err)
		}
	}

	if settings.hasFileMode {
		if err := os.Chmod(filePath, settings.fileMode); err != nil {
			return fmt.Errorf("set managed file permissions: %w", err)
		}
	}

	return nil
}
