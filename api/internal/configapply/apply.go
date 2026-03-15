package configapply

import (
	"crypto/rand"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"golang.org/x/crypto/bcrypt"

	internalapi "github.com/lotsendev/lotsen/internal/api"
	"github.com/lotsendev/lotsen/internal/configplan"
	"github.com/lotsendev/lotsen/internal/configv1"
	"github.com/lotsendev/lotsen/store"
)

type StateStore interface {
	List() ([]store.Deployment, error)
	Create(d store.Deployment) (store.Deployment, error)
	Update(d store.Deployment) (store.Deployment, error)
	Delete(id string) error

	ListRegistries() ([]store.RegistryEntry, error)
	CreateRegistry(id, prefix, username, password string) (store.RegistryEntry, error)
	UpdateRegistry(id, prefix, username, password string) (store.RegistryEntry, error)
	DeleteRegistry(id string) error
}

type HostProfileStore interface {
	Update(profile internalapi.HostProfile) (internalapi.HostProfile, error)
}

type Status string

const (
	StatusApplied Status = "applied"
	StatusNoop    Status = "noop"
	StatusFailed  Status = "failed"
)

type Outcome struct {
	ResourceType string `json:"resourceType"`
	Action       string `json:"action"`
	Resource     string `json:"resource"`
	Status       Status `json:"status"`
	Message      string `json:"message,omitempty"`
}

type Summary struct {
	Applied int `json:"applied"`
	Noop    int `json:"noop"`
	Failed  int `json:"failed"`
}

type Result struct {
	Fingerprint   string    `json:"fingerprint"`
	Outcomes      []Outcome `json:"outcomes"`
	Summary       Summary   `json:"summary"`
	PartialFailed bool      `json:"partialFailed"`
}

func Execute(desired configv1.Document, plan configplan.Document, stateStore StateStore, hostProfiles HostProfileStore) (Result, error) {
	result := Result{Fingerprint: plan.Fingerprint, Outcomes: make([]Outcome, 0)}

	desiredDeployments := make(map[string]configv1.Deployment, len(desired.Spec.Deployments))
	for _, deployment := range desired.Spec.Deployments {
		desiredDeployments[deployment.Name] = deployment
	}
	desiredRegistries := make(map[string]configv1.Registry, len(desired.Spec.Registries))
	for _, registry := range desired.Spec.Registries {
		desiredRegistries[registry.Prefix] = registry
	}

	liveDeployments, err := stateStore.List()
	if err != nil {
		return result, fmt.Errorf("list deployments: %w", err)
	}
	liveByName := make(map[string]store.Deployment, len(liveDeployments))
	for _, deployment := range liveDeployments {
		liveByName[deployment.Name] = deployment
	}

	registryEntries, err := stateStore.ListRegistries()
	if err != nil {
		return result, fmt.Errorf("list registries: %w", err)
	}
	registryIDByPrefix := make(map[string]string, len(registryEntries))
	for _, entry := range registryEntries {
		registryIDByPrefix[entry.Prefix] = entry.ID
	}

	for _, change := range plan.Actions.Registries.Noop {
		appendOutcome(&result, Outcome{ResourceType: "registry", Action: "noop", Resource: change.Resource, Status: StatusNoop})
	}
	for _, change := range plan.Actions.Registries.Create {
		registry, ok := desiredRegistries[change.Resource]
		if !ok {
			appendOutcome(&result, Outcome{ResourceType: "registry", Action: "create", Resource: change.Resource, Status: StatusFailed, Message: "missing in desired config"})
			continue
		}
		id, newIDErr := newID()
		if newIDErr != nil {
			appendOutcome(&result, Outcome{ResourceType: "registry", Action: "create", Resource: change.Resource, Status: StatusFailed, Message: newIDErr.Error()})
			continue
		}
		if _, createErr := stateStore.CreateRegistry(id, registry.Prefix, registry.Username, registry.Password); createErr != nil {
			appendOutcome(&result, Outcome{ResourceType: "registry", Action: "create", Resource: change.Resource, Status: StatusFailed, Message: createErr.Error()})
			continue
		}
		appendOutcome(&result, Outcome{ResourceType: "registry", Action: "create", Resource: change.Resource, Status: StatusApplied})
	}
	for _, change := range plan.Actions.Registries.Update {
		registry, ok := desiredRegistries[change.Resource]
		if !ok {
			appendOutcome(&result, Outcome{ResourceType: "registry", Action: "update", Resource: change.Resource, Status: StatusFailed, Message: "missing in desired config"})
			continue
		}
		id, ok := registryIDByPrefix[change.Resource]
		if !ok {
			appendOutcome(&result, Outcome{ResourceType: "registry", Action: "update", Resource: change.Resource, Status: StatusFailed, Message: "missing in live state"})
			continue
		}
		if _, updateErr := stateStore.UpdateRegistry(id, registry.Prefix, registry.Username, registry.Password); updateErr != nil {
			appendOutcome(&result, Outcome{ResourceType: "registry", Action: "update", Resource: change.Resource, Status: StatusFailed, Message: updateErr.Error()})
			continue
		}
		appendOutcome(&result, Outcome{ResourceType: "registry", Action: "update", Resource: change.Resource, Status: StatusApplied})
	}

	for _, change := range plan.Actions.Deployments.Noop {
		appendOutcome(&result, Outcome{ResourceType: "deployment", Action: "noop", Resource: change.Resource, Status: StatusNoop})
	}
	for _, change := range plan.Actions.Deployments.Create {
		deployment, ok := desiredDeployments[change.Resource]
		if !ok {
			appendOutcome(&result, Outcome{ResourceType: "deployment", Action: "create", Resource: change.Resource, Status: StatusFailed, Message: "missing in desired config"})
			continue
		}
		id, newIDErr := newID()
		if newIDErr != nil {
			appendOutcome(&result, Outcome{ResourceType: "deployment", Action: "create", Resource: change.Resource, Status: StatusFailed, Message: newIDErr.Error()})
			continue
		}

		created, convertErr := deploymentFromDesired(deployment, id)
		if convertErr != nil {
			appendOutcome(&result, Outcome{ResourceType: "deployment", Action: "create", Resource: change.Resource, Status: StatusFailed, Message: convertErr.Error()})
			continue
		}
		created.Status = store.StatusDeploying
		if _, createErr := stateStore.Create(created); createErr != nil {
			appendOutcome(&result, Outcome{ResourceType: "deployment", Action: "create", Resource: change.Resource, Status: StatusFailed, Message: createErr.Error()})
			continue
		}
		appendOutcome(&result, Outcome{ResourceType: "deployment", Action: "create", Resource: change.Resource, Status: StatusApplied})
	}
	for _, change := range plan.Actions.Deployments.Update {
		desiredDeployment, ok := desiredDeployments[change.Resource]
		if !ok {
			appendOutcome(&result, Outcome{ResourceType: "deployment", Action: "update", Resource: change.Resource, Status: StatusFailed, Message: "missing in desired config"})
			continue
		}
		existing, ok := liveByName[change.Resource]
		if !ok {
			appendOutcome(&result, Outcome{ResourceType: "deployment", Action: "update", Resource: change.Resource, Status: StatusFailed, Message: "missing in live state"})
			continue
		}

		updated, convertErr := deploymentFromDesired(desiredDeployment, existing.ID)
		if convertErr != nil {
			appendOutcome(&result, Outcome{ResourceType: "deployment", Action: "update", Resource: change.Resource, Status: StatusFailed, Message: convertErr.Error()})
			continue
		}
		updated.Status = existing.Status
		updated.Reason = existing.Reason
		updated.Error = existing.Error
		if redeployRequired(change.Changes) {
			updated.Status = store.StatusDeploying
			updated.Reason = ""
			updated.Error = ""
		}

		if _, updateErr := stateStore.Update(updated); updateErr != nil {
			appendOutcome(&result, Outcome{ResourceType: "deployment", Action: "update", Resource: change.Resource, Status: StatusFailed, Message: updateErr.Error()})
			continue
		}
		appendOutcome(&result, Outcome{ResourceType: "deployment", Action: "update", Resource: change.Resource, Status: StatusApplied})
	}

	for _, change := range plan.Actions.Host.Noop {
		appendOutcome(&result, Outcome{ResourceType: "host", Action: "noop", Resource: change.Resource, Status: StatusNoop})
	}
	for _, change := range plan.Actions.Host.Update {
		if desired.Spec.Host == nil {
			appendOutcome(&result, Outcome{ResourceType: "host", Action: "update", Resource: change.Resource, Status: StatusFailed, Message: "missing in desired config"})
			continue
		}
		profile := hostProfileFromDesired(*desired.Spec.Host)
		if _, updateErr := hostProfiles.Update(profile); updateErr != nil {
			appendOutcome(&result, Outcome{ResourceType: "host", Action: "update", Resource: change.Resource, Status: StatusFailed, Message: updateErr.Error()})
			continue
		}
		appendOutcome(&result, Outcome{ResourceType: "host", Action: "update", Resource: change.Resource, Status: StatusApplied})
	}

	for _, change := range plan.Actions.Deployments.Delete {
		existing, ok := liveByName[change.Resource]
		if !ok {
			appendOutcome(&result, Outcome{ResourceType: "deployment", Action: "delete", Resource: change.Resource, Status: StatusFailed, Message: "missing in live state"})
			continue
		}
		if deleteErr := stateStore.Delete(existing.ID); deleteErr != nil {
			appendOutcome(&result, Outcome{ResourceType: "deployment", Action: "delete", Resource: change.Resource, Status: StatusFailed, Message: deleteErr.Error()})
			continue
		}
		appendOutcome(&result, Outcome{ResourceType: "deployment", Action: "delete", Resource: change.Resource, Status: StatusApplied})
	}
	for _, change := range plan.Actions.Registries.Delete {
		id, ok := registryIDByPrefix[change.Resource]
		if !ok {
			appendOutcome(&result, Outcome{ResourceType: "registry", Action: "delete", Resource: change.Resource, Status: StatusFailed, Message: "missing in live state"})
			continue
		}
		if deleteErr := stateStore.DeleteRegistry(id); deleteErr != nil {
			appendOutcome(&result, Outcome{ResourceType: "registry", Action: "delete", Resource: change.Resource, Status: StatusFailed, Message: deleteErr.Error()})
			continue
		}
		appendOutcome(&result, Outcome{ResourceType: "registry", Action: "delete", Resource: change.Resource, Status: StatusApplied})
	}

	if result.Summary.Failed > 0 {
		result.PartialFailed = true
		return result, fmt.Errorf("apply finished with %d failed actions", result.Summary.Failed)
	}

	return result, nil
}

func appendOutcome(result *Result, outcome Outcome) {
	result.Outcomes = append(result.Outcomes, outcome)
	switch outcome.Status {
	case StatusApplied:
		result.Summary.Applied++
	case StatusNoop:
		result.Summary.Noop++
	case StatusFailed:
		result.Summary.Failed++
	}
}

func deploymentFromDesired(in configv1.Deployment, id string) (store.Deployment, error) {
	basicAuth, err := hashBasicAuth(in.BasicAuth)
	if err != nil {
		return store.Deployment{}, err
	}

	volumes, err := volumeBindingsFromMounts(id, in.VolumeMounts)
	if err != nil {
		return store.Deployment{}, err
	}

	out := store.Deployment{
		ID:         id,
		Name:       strings.TrimSpace(in.Name),
		Image:      strings.TrimSpace(in.Image),
		Envs:       cloneMap(in.Env),
		Ports:      slices.Clone(in.Ports),
		Volumes:    volumes,
		FileMounts: fileMountsFromDesired(in.FileMounts),
		Domain:     strings.TrimSpace(in.Domain),
		BasicAuth:  basicAuth,
		Security:   securityFromDesired(in.Security),
	}
	if in.Public != nil {
		out.Public = *in.Public
	}
	if in.ProxyPort != nil {
		out.ProxyPort = *in.ProxyPort
	}

	return out, nil
}

func hashBasicAuth(in *configv1.BasicAuth) (*store.BasicAuthConfig, error) {
	if in == nil {
		return nil, nil
	}
	if len(in.Users) == 0 {
		return nil, nil
	}

	users := make([]store.BasicAuthUser, 0, len(in.Users))
	for _, user := range in.Users {
		password := strings.TrimSpace(user.Password)
		if _, err := bcrypt.Cost([]byte(password)); err != nil {
			hashed, hashErr := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
			if hashErr != nil {
				return nil, fmt.Errorf("hash basic auth password for %q: %w", user.Username, hashErr)
			}
			password = string(hashed)
		}
		users = append(users, store.BasicAuthUser{Username: strings.TrimSpace(user.Username), Password: password})
	}

	return &store.BasicAuthConfig{Users: users}, nil
}

func volumeBindingsFromMounts(deploymentID string, mounts []configv1.VolumeMount) ([]string, error) {
	if len(mounts) == 0 {
		return nil, nil
	}

	bindings := make([]string, 0, len(mounts))
	for _, mount := range mounts {
		mode := strings.ToLower(strings.TrimSpace(mount.Mode))
		target := strings.TrimSpace(mount.Target)
		source := strings.TrimSpace(mount.Source)
		switch mode {
		case "managed":
			source = filepath.Join(managedVolumesBaseDirFromEnv(), deploymentID, source)
		case "bind":
		default:
			return nil, fmt.Errorf("unsupported volume mount mode %q", mount.Mode)
		}
		bindings = append(bindings, source+":"+target)
	}

	return bindings, nil
}

func managedVolumesBaseDirFromEnv() string {
	if dir := strings.TrimSpace(os.Getenv("LOTSEN_MANAGED_VOLUMES_DIR")); dir != "" {
		return filepath.Clean(dir)
	}
	return "/var/lib/lotsen/volumes"
}

func fileMountsFromDesired(in []configv1.FileMount) []store.FileMount {
	if len(in) == 0 {
		return nil
	}

	out := make([]store.FileMount, 0, len(in))
	for _, mount := range in {
		out = append(out, store.FileMount{
			Source:   strings.TrimSpace(mount.Source),
			Target:   strings.TrimSpace(mount.Target),
			Content:  mount.Content,
			UID:      mount.UID,
			GID:      mount.GID,
			FileMode: strings.TrimSpace(mount.FileMode),
			ReadOnly: mount.ReadOnly,
		})
	}

	return out
}

func securityFromDesired(in *configv1.Security) *store.SecurityConfig {
	if in == nil {
		return nil
	}

	return &store.SecurityConfig{
		WAFEnabled:  in.WAFEnabled,
		WAFMode:     strings.TrimSpace(in.WAFMode),
		IPDenylist:  slices.Clone(in.IPDenylist),
		IPAllowlist: slices.Clone(in.IPAllowlist),
		CustomRules: slices.Clone(in.CustomRules),
	}
}

func redeployRequired(changes []string) bool {
	for _, field := range changes {
		switch strings.TrimSpace(field) {
		case "image", "env", "ports", "proxyPort", "volumeMounts", "basicAuth":
			return true
		}
	}
	return false
}

func hostProfileFromDesired(in configv1.Host) internalapi.HostProfile {
	profile := internalapi.HostProfile{
		DisplayName:         strings.TrimSpace(in.DisplayName),
		DashboardAccessMode: normalizeDashboardAccessMode(in.DashboardAccessMode),
		DashboardWAF: internalapi.DashboardWAFConfig{
			Mode: "detection",
		},
	}

	if in.DashboardWAF == nil {
		return profile
	}

	mode := strings.ToLower(strings.TrimSpace(in.DashboardWAF.Mode))
	if mode == "enforcement" {
		profile.DashboardWAF.Mode = "enforcement"
	}
	profile.DashboardWAF.IPAllowlist = slices.Clone(in.DashboardWAF.IPAllowlist)
	profile.DashboardWAF.CustomRules = slices.Clone(in.DashboardWAF.CustomRules)

	return profile
}

func normalizeDashboardAccessMode(mode string) internalapi.DashboardAccessMode {
	switch strings.ToLower(strings.TrimSpace(mode)) {
	case "waf_only":
		return internalapi.DashboardAccessModeWAFOnly
	case "waf_and_login":
		return internalapi.DashboardAccessModeWAFAndLogin
	default:
		return internalapi.DashboardAccessModeLoginOnly
	}
}

func cloneMap(in map[string]string) map[string]string {
	if in == nil {
		return nil
	}

	out := make(map[string]string, len(in))
	for key, value := range in {
		out[key] = value
	}

	return out
}

func newID() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("new id: %w", err)
	}
	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:]), nil
}
