package api

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
)

const (
	volumeMountModeManaged = "managed"
	volumeMountModeBind    = "bind"
	defaultManagedDirMode  = 0o777
)

var managedVolumeNamePattern = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9_.-]{0,63}$`)

type volumeMountRequest struct {
	Mode    string `json:"mode"`
	Source  string `json:"source"`
	Target  string `json:"target"`
	UID     *int   `json:"uid,omitempty"`
	GID     *int   `json:"gid,omitempty"`
	DirMode string `json:"dir_mode,omitempty"`
}

type volumeMountResponse struct {
	Mode   string `json:"mode"`
	Source string `json:"source"`
	Target string `json:"target"`
}

type managedVolumeSettings struct {
	hasUID     bool
	uid        int
	hasGID     bool
	gid        int
	hasDirMode bool
	dirMode    os.FileMode
}

func managedVolumesBaseDirFromEnv() string {
	if dir := strings.TrimSpace(os.Getenv("LOTSEN_MANAGED_VOLUMES_DIR")); dir != "" {
		return filepath.Clean(dir)
	}
	return "/var/lib/lotsen/volumes"
}

func resolveVolumeBindings(deploymentID string, volumes []string, mounts []volumeMountRequest) ([]string, error) {
	if mounts == nil {
		if volumes == nil {
			return []string{}, nil
		}
		return volumes, nil
	}

	bindings := make([]string, 0, len(mounts))
	seenSources := make(map[string]struct{}, len(mounts))
	seenTargets := make(map[string]struct{}, len(mounts))

	for _, mount := range mounts {
		mode := strings.ToLower(strings.TrimSpace(mount.Mode))
		source := strings.TrimSpace(mount.Source)
		target, err := cleanAbsolutePath(mount.Target)
		if err != nil {
			return nil, fmt.Errorf("volume target must be an absolute path")
		}

		if _, duplicateTarget := seenTargets[target]; duplicateTarget {
			return nil, fmt.Errorf("volume target %q is already used", target)
		}
		seenTargets[target] = struct{}{}

		switch mode {
		case volumeMountModeManaged:
			if !managedVolumeNamePattern.MatchString(source) {
				return nil, fmt.Errorf("managed volume names must match %q", managedVolumeNamePattern.String())
			}

			settings, settingsErr := managedVolumeSettingsFromRequest(mount)
			if settingsErr != nil {
				return nil, settingsErr
			}

			hostPath, pathErr := ensureManagedVolumeDirectoryWithSettings(deploymentID, source, settings)
			if pathErr != nil {
				return nil, pathErr
			}

			if _, duplicateSource := seenSources[hostPath]; duplicateSource {
				return nil, fmt.Errorf("volume source %q is already used", source)
			}
			seenSources[hostPath] = struct{}{}
			bindings = append(bindings, hostPath+":"+target)
		case volumeMountModeBind:
			if mount.UID != nil || mount.GID != nil || strings.TrimSpace(mount.DirMode) != "" {
				return nil, fmt.Errorf("uid, gid, and dir_mode are only supported for managed volume mounts")
			}

			hostPath, hostErr := cleanAbsolutePath(source)
			if hostErr != nil {
				return nil, fmt.Errorf("bind source must be an absolute path")
			}
			if _, duplicateSource := seenSources[hostPath]; duplicateSource {
				return nil, fmt.Errorf("volume source %q is already used", hostPath)
			}
			seenSources[hostPath] = struct{}{}
			bindings = append(bindings, hostPath+":"+target)
		default:
			return nil, fmt.Errorf("volume mode must be %q or %q", volumeMountModeManaged, volumeMountModeBind)
		}
	}

	return bindings, nil
}

func ensureManagedVolumeDirectoryWithSettings(deploymentID, volumeName string, settings managedVolumeSettings) (string, error) {
	base := managedVolumesBaseDirFromEnv()
	if !filepath.IsAbs(base) {
		return "", fmt.Errorf("managed volumes base path must be absolute")
	}

	cleanBase := filepath.Clean(base)
	volumeDir := filepath.Join(cleanBase, deploymentID, volumeName)
	if !isPathWithinBase(cleanBase, volumeDir) {
		return "", fmt.Errorf("managed volume path escapes configured base directory")
	}

	existed := true
	if _, err := os.Stat(volumeDir); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			existed = false
		} else {
			return "", fmt.Errorf("stat managed volume directory: %w", err)
		}
	}

	if err := os.MkdirAll(volumeDir, 0o755); err != nil {
		return "", fmt.Errorf("create managed volume directory: %w", err)
	}

	if err := applyManagedVolumeSettings(volumeDir, settings, existed); err != nil {
		return "", err
	}

	return volumeDir, nil
}

func managedVolumeSettingsFromRequest(mount volumeMountRequest) (managedVolumeSettings, error) {
	settings := managedVolumeSettings{}

	if mount.UID != nil {
		if *mount.UID < 0 {
			return managedVolumeSettings{}, fmt.Errorf("uid must be >= 0")
		}
		settings.hasUID = true
		settings.uid = *mount.UID
	}

	if mount.GID != nil {
		if *mount.GID < 0 {
			return managedVolumeSettings{}, fmt.Errorf("gid must be >= 0")
		}
		settings.hasGID = true
		settings.gid = *mount.GID
	}

	rawDirMode := strings.TrimSpace(mount.DirMode)
	if rawDirMode != "" {
		parsed, err := strconv.ParseUint(rawDirMode, 8, 32)
		if err != nil || parsed > 0o777 {
			return managedVolumeSettings{}, fmt.Errorf("dir_mode must be an octal permission between 0000 and 0777")
		}
		settings.hasDirMode = true
		settings.dirMode = os.FileMode(parsed)
	}

	return settings, nil
}

func applyManagedVolumeSettings(volumeDir string, settings managedVolumeSettings, existed bool) error {
	if !settings.hasDirMode && !existed {
		settings.hasDirMode = true
		settings.dirMode = defaultManagedDirMode
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
		if err := os.Chown(volumeDir, uid, gid); err != nil {
			return fmt.Errorf("set managed volume ownership: %w", err)
		}
	}

	if settings.hasDirMode {
		if err := os.Chmod(volumeDir, settings.dirMode); err != nil {
			return fmt.Errorf("set managed volume permissions: %w", err)
		}
	}

	return nil
}

func cleanAbsolutePath(raw string) (string, error) {
	cleaned := filepath.Clean(strings.TrimSpace(raw))
	if cleaned == "." || !filepath.IsAbs(cleaned) {
		return "", fmt.Errorf("path must be absolute")
	}
	return cleaned, nil
}

func isPathWithinBase(basePath, candidate string) bool {
	rel, err := filepath.Rel(basePath, candidate)
	if err != nil {
		return false
	}
	if rel == "." {
		return true
	}
	return rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}

func volumeMountsFromBindings(deploymentID string, bindings []string) []volumeMountResponse {
	if len(bindings) == 0 {
		return nil
	}

	mounts := make([]volumeMountResponse, 0, len(bindings))
	for _, binding := range bindings {
		sep := strings.IndexByte(binding, ':')
		if sep <= 0 {
			continue
		}

		source := binding[:sep]
		target := binding[sep+1:]

		managedName, managed := managedVolumeNameForDeployment(deploymentID, source)
		if managed {
			mounts = append(mounts, volumeMountResponse{Mode: volumeMountModeManaged, Source: managedName, Target: target})
			continue
		}

		mounts = append(mounts, volumeMountResponse{Mode: volumeMountModeBind, Source: source, Target: target})
	}

	return mounts
}

func managedVolumeNameForDeployment(deploymentID, source string) (string, bool) {
	base := managedVolumesBaseDirFromEnv()
	if !filepath.IsAbs(base) {
		return "", false
	}

	prefix := filepath.Join(filepath.Clean(base), deploymentID) + string(filepath.Separator)
	cleanSource := filepath.Clean(strings.TrimSpace(source))
	if !strings.HasPrefix(cleanSource, prefix) {
		return "", false
	}

	name := strings.TrimPrefix(cleanSource, prefix)
	if strings.Contains(name, string(filepath.Separator)) {
		return "", false
	}
	if !managedVolumeNamePattern.MatchString(name) {
		return "", false
	}

	return name, true
}
