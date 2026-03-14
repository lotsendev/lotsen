package configplan

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"regexp"
	"slices"
	"strings"

	"github.com/lotsendev/lotsen/internal/configv1"
)

const Kind = "LotsenPlan"

var placeholderPattern = regexp.MustCompile(`^\$\{LOTSEN_SECRET_[A-Z0-9_]+\}$`)

type Document struct {
	APIVersion  string             `json:"apiVersion"`
	Kind        string             `json:"kind"`
	Fingerprint string             `json:"fingerprint"`
	Summary     Summary            `json:"summary"`
	Actions     Actions            `json:"actions"`
	Drift       DriftSection       `json:"drift"`
	Conflicts   ConflictSection    `json:"conflicts"`
	Destructive DestructiveSection `json:"destructive"`
}

type Summary struct {
	CreateCount      int `json:"createCount"`
	UpdateCount      int `json:"updateCount"`
	DeleteCount      int `json:"deleteCount"`
	NoopCount        int `json:"noopCount"`
	DriftCount       int `json:"driftCount"`
	ConflictCount    int `json:"conflictCount"`
	DestructiveCount int `json:"destructiveCount"`
}

type Actions struct {
	Deployments ResourceActions `json:"deployments"`
	Registries  ResourceActions `json:"registries"`
	Host        HostActions     `json:"host"`
}

type ResourceActions struct {
	Create []ResourceChange `json:"create"`
	Update []ResourceChange `json:"update"`
	Delete []ResourceChange `json:"delete"`
	Noop   []ResourceChange `json:"noop"`
}

type HostActions struct {
	Update []ResourceChange `json:"update"`
	Noop   []ResourceChange `json:"noop"`
}

type ResourceChange struct {
	Resource string   `json:"resource"`
	Changes  []string `json:"changes,omitempty"`
}

type DriftSection struct {
	Deployments []ResourceChange `json:"deployments"`
	Registries  []ResourceChange `json:"registries"`
	Host        []ResourceChange `json:"host"`
}

type ConflictSection struct {
	Deployments []Conflict `json:"deployments"`
	Registries  []Conflict `json:"registries"`
	Host        []Conflict `json:"host"`
}

type Conflict struct {
	Resource string `json:"resource"`
	Reason   string `json:"reason"`
}

type DestructiveSection struct {
	Deployments []ResourceChange `json:"deployments"`
	Registries  []ResourceChange `json:"registries"`
	Host        []ResourceChange `json:"host"`
}

func Build(desired, live configv1.Document) (Document, error) {
	if desired.Spec == nil {
		return Document{}, fmt.Errorf("desired config spec is required")
	}
	if live.Spec == nil {
		return Document{}, fmt.Errorf("live config spec is required")
	}

	canonicalDesired := configv1.Canonicalize(desired)
	canonicalLive := configv1.Canonicalize(live)

	plan := Document{
		APIVersion: configv1.APIVersion,
		Kind:       Kind,
		Actions: Actions{
			Deployments: ResourceActions{Create: []ResourceChange{}, Update: []ResourceChange{}, Delete: []ResourceChange{}, Noop: []ResourceChange{}},
			Registries:  ResourceActions{Create: []ResourceChange{}, Update: []ResourceChange{}, Delete: []ResourceChange{}, Noop: []ResourceChange{}},
			Host:        HostActions{Update: []ResourceChange{}, Noop: []ResourceChange{}},
		},
		Drift:     DriftSection{Deployments: []ResourceChange{}, Registries: []ResourceChange{}, Host: []ResourceChange{}},
		Conflicts: ConflictSection{Deployments: []Conflict{}, Registries: []Conflict{}, Host: []Conflict{}},
		Destructive: DestructiveSection{
			Deployments: []ResourceChange{},
			Registries:  []ResourceChange{},
			Host:        []ResourceChange{},
		},
	}

	planningDeployments(&plan, canonicalDesired.Spec.Deployments, canonicalLive.Spec.Deployments)
	planningRegistries(&plan, canonicalDesired.Spec.Registries, canonicalLive.Spec.Registries)
	planningHost(&plan, canonicalDesired.Spec.Host, canonicalLive.Spec.Host)

	plan.Summary = Summary{
		CreateCount:      len(plan.Actions.Deployments.Create) + len(plan.Actions.Registries.Create),
		UpdateCount:      len(plan.Actions.Deployments.Update) + len(plan.Actions.Registries.Update) + len(plan.Actions.Host.Update),
		DeleteCount:      len(plan.Actions.Deployments.Delete) + len(plan.Actions.Registries.Delete),
		NoopCount:        len(plan.Actions.Deployments.Noop) + len(plan.Actions.Registries.Noop) + len(plan.Actions.Host.Noop),
		DriftCount:       len(plan.Drift.Deployments) + len(plan.Drift.Registries) + len(plan.Drift.Host),
		ConflictCount:    len(plan.Conflicts.Deployments) + len(plan.Conflicts.Registries) + len(plan.Conflicts.Host),
		DestructiveCount: len(plan.Destructive.Deployments) + len(plan.Destructive.Registries) + len(plan.Destructive.Host),
	}

	plan.Fingerprint = fingerprint(plan)

	return plan, nil
}

func MarshalCanonical(plan Document) ([]byte, error) {
	canonical := canonicalizePlan(plan)
	out, err := json.MarshalIndent(canonical, "", "  ")
	if err != nil {
		return nil, err
	}
	return append(out, '\n'), nil
}

func fingerprint(plan Document) string {
	copy := canonicalizePlan(plan)
	copy.Fingerprint = ""
	payload, _ := json.Marshal(copy)
	sum := sha256.Sum256(payload)
	return "sha256:" + hex.EncodeToString(sum[:])
}

func planningDeployments(plan *Document, desiredDeployments, liveDeployments []configv1.Deployment) {
	desiredByName := make(map[string]configv1.Deployment, len(desiredDeployments))
	for _, deployment := range desiredDeployments {
		desiredByName[deployment.Name] = deployment
	}

	liveByName := make(map[string]configv1.Deployment, len(liveDeployments))
	for _, deployment := range liveDeployments {
		liveByName[deployment.Name] = deployment
	}

	desiredNames := sortedKeys(desiredByName)
	for _, name := range desiredNames {
		desired := desiredByName[name]
		live, exists := liveByName[name]
		if !exists {
			plan.Actions.Deployments.Create = append(plan.Actions.Deployments.Create, ResourceChange{Resource: name})
			continue
		}

		changes := diffDeployment(live, desired)
		if len(changes) == 0 {
			plan.Actions.Deployments.Noop = append(plan.Actions.Deployments.Noop, ResourceChange{Resource: name})
			continue
		}

		change := ResourceChange{Resource: name, Changes: changes}
		plan.Actions.Deployments.Update = append(plan.Actions.Deployments.Update, change)
		plan.Drift.Deployments = append(plan.Drift.Deployments, change)
	}

	liveNames := sortedKeys(liveByName)
	for _, name := range liveNames {
		if _, exists := desiredByName[name]; exists {
			continue
		}
		change := ResourceChange{Resource: name}
		plan.Actions.Deployments.Delete = append(plan.Actions.Deployments.Delete, change)
		plan.Drift.Deployments = append(plan.Drift.Deployments, change)
		plan.Destructive.Deployments = append(plan.Destructive.Deployments, change)
	}

	plan.Conflicts.Deployments = deploymentConflicts(desiredByName, liveByName)
}

func planningRegistries(plan *Document, desiredRegistries, liveRegistries []configv1.Registry) {
	desiredByPrefix := make(map[string]configv1.Registry, len(desiredRegistries))
	for _, registry := range desiredRegistries {
		desiredByPrefix[registry.Prefix] = registry
	}

	liveByPrefix := make(map[string]configv1.Registry, len(liveRegistries))
	for _, registry := range liveRegistries {
		liveByPrefix[registry.Prefix] = registry
	}

	desiredPrefixes := sortedKeys(desiredByPrefix)
	for _, prefix := range desiredPrefixes {
		desired := desiredByPrefix[prefix]
		live, exists := liveByPrefix[prefix]
		if !exists {
			plan.Actions.Registries.Create = append(plan.Actions.Registries.Create, ResourceChange{Resource: prefix})
			continue
		}

		changes := diffRegistry(live, desired)
		if len(changes) == 0 {
			plan.Actions.Registries.Noop = append(plan.Actions.Registries.Noop, ResourceChange{Resource: prefix})
			continue
		}

		change := ResourceChange{Resource: prefix, Changes: changes}
		plan.Actions.Registries.Update = append(plan.Actions.Registries.Update, change)
		plan.Drift.Registries = append(plan.Drift.Registries, change)
	}

	livePrefixes := sortedKeys(liveByPrefix)
	for _, prefix := range livePrefixes {
		if _, exists := desiredByPrefix[prefix]; exists {
			continue
		}
		change := ResourceChange{Resource: prefix}
		plan.Actions.Registries.Delete = append(plan.Actions.Registries.Delete, change)
		plan.Drift.Registries = append(plan.Drift.Registries, change)
		plan.Destructive.Registries = append(plan.Destructive.Registries, change)
	}
}

func planningHost(plan *Document, desiredHost, liveHost *configv1.Host) {
	if desiredHost == nil {
		plan.Actions.Host.Noop = append(plan.Actions.Host.Noop, ResourceChange{Resource: "host"})
		return
	}

	if liveHost == nil {
		liveHost = &configv1.Host{}
	}

	changes := diffHost(*liveHost, *desiredHost)
	if len(changes) == 0 {
		plan.Actions.Host.Noop = append(plan.Actions.Host.Noop, ResourceChange{Resource: "host"})
		return
	}

	change := ResourceChange{Resource: "host", Changes: changes}
	plan.Actions.Host.Update = append(plan.Actions.Host.Update, change)
	plan.Drift.Host = append(plan.Drift.Host, change)
}

func deploymentConflicts(desiredByName, liveByName map[string]configv1.Deployment) []Conflict {
	liveByDomain := make(map[string]string, len(liveByName))
	for name, deployment := range liveByName {
		domain := strings.TrimSpace(deployment.Domain)
		if domain == "" {
			continue
		}
		if _, exists := liveByDomain[domain]; !exists {
			liveByDomain[domain] = name
		}
	}

	conflicts := make([]Conflict, 0)
	for name, deployment := range desiredByName {
		domain := strings.TrimSpace(deployment.Domain)
		if domain == "" {
			continue
		}

		owner, exists := liveByDomain[domain]
		if !exists || owner == name {
			continue
		}

		if _, managed := desiredByName[owner]; !managed {
			continue
		}

		conflicts = append(conflicts, Conflict{
			Resource: name,
			Reason:   fmt.Sprintf("domain %q is already claimed by deployment %q", domain, owner),
		})
	}

	slices.SortFunc(conflicts, func(a, b Conflict) int {
		return strings.Compare(a.Resource, b.Resource)
	})
	return conflicts
}

func diffDeployment(live, desired configv1.Deployment) []string {
	changes := make([]string, 0)
	if strings.TrimSpace(live.Image) != strings.TrimSpace(desired.Image) {
		changes = append(changes, "image")
	}
	if strings.TrimSpace(live.Domain) != strings.TrimSpace(desired.Domain) {
		changes = append(changes, "domain")
	}
	if !equalBoolPtr(live.Public, desired.Public) {
		changes = append(changes, "public")
	}
	if !equalPorts(live.Ports, desired.Ports) {
		changes = append(changes, "ports")
	}
	if !equalIntPtr(live.ProxyPort, desired.ProxyPort) {
		changes = append(changes, "proxyPort")
	}
	if !equalEnv(live.Env, desired.Env) {
		changes = append(changes, "env")
	}
	if !equalVolumeMounts(live.VolumeMounts, desired.VolumeMounts) {
		changes = append(changes, "volumeMounts")
	}
	if !equalBasicAuth(live.BasicAuth, desired.BasicAuth) {
		changes = append(changes, "basicAuth")
	}
	if !equalSecurity(live.Security, desired.Security) {
		changes = append(changes, "security")
	}

	slices.Sort(changes)
	return changes
}

func diffRegistry(live, desired configv1.Registry) []string {
	changes := make([]string, 0)
	if strings.TrimSpace(live.Username) != strings.TrimSpace(desired.Username) {
		changes = append(changes, "username")
	}
	if !equalSecret(live.Password, desired.Password) {
		changes = append(changes, "password")
	}

	slices.Sort(changes)
	return changes
}

func diffHost(live, desired configv1.Host) []string {
	changes := make([]string, 0)
	if strings.TrimSpace(live.DisplayName) != strings.TrimSpace(desired.DisplayName) {
		changes = append(changes, "displayName")
	}
	if normalizeDashboardAccessMode(live.DashboardAccessMode) != normalizeDashboardAccessMode(desired.DashboardAccessMode) {
		changes = append(changes, "dashboardAccessMode")
	}
	if !equalDashboardWAF(live.DashboardWAF, desired.DashboardWAF) {
		changes = append(changes, "dashboardWaf")
	}

	slices.Sort(changes)
	return changes
}

func canonicalizePlan(plan Document) Document {
	canonical := plan
	canonical.Actions.Deployments = canonicalizeResourceActions(canonical.Actions.Deployments)
	canonical.Actions.Registries = canonicalizeResourceActions(canonical.Actions.Registries)
	canonical.Actions.Host.Update = canonicalizeChanges(canonical.Actions.Host.Update)
	canonical.Actions.Host.Noop = canonicalizeChanges(canonical.Actions.Host.Noop)

	canonical.Drift.Deployments = canonicalizeChanges(canonical.Drift.Deployments)
	canonical.Drift.Registries = canonicalizeChanges(canonical.Drift.Registries)
	canonical.Drift.Host = canonicalizeChanges(canonical.Drift.Host)

	canonical.Conflicts.Deployments = slices.Clone(canonical.Conflicts.Deployments)
	canonical.Conflicts.Registries = slices.Clone(canonical.Conflicts.Registries)
	canonical.Conflicts.Host = slices.Clone(canonical.Conflicts.Host)
	slices.SortFunc(canonical.Conflicts.Deployments, func(a, b Conflict) int { return strings.Compare(a.Resource, b.Resource) })
	slices.SortFunc(canonical.Conflicts.Registries, func(a, b Conflict) int { return strings.Compare(a.Resource, b.Resource) })
	slices.SortFunc(canonical.Conflicts.Host, func(a, b Conflict) int { return strings.Compare(a.Resource, b.Resource) })

	canonical.Destructive.Deployments = canonicalizeChanges(canonical.Destructive.Deployments)
	canonical.Destructive.Registries = canonicalizeChanges(canonical.Destructive.Registries)
	canonical.Destructive.Host = canonicalizeChanges(canonical.Destructive.Host)

	return canonical
}

func canonicalizeResourceActions(in ResourceActions) ResourceActions {
	out := in
	out.Create = canonicalizeChanges(in.Create)
	out.Update = canonicalizeChanges(in.Update)
	out.Delete = canonicalizeChanges(in.Delete)
	out.Noop = canonicalizeChanges(in.Noop)
	return out
}

func canonicalizeChanges(in []ResourceChange) []ResourceChange {
	out := slices.Clone(in)
	for i := range out {
		if len(out[i].Changes) == 0 {
			continue
		}
		out[i].Changes = slices.Clone(out[i].Changes)
		slices.Sort(out[i].Changes)
	}
	slices.SortFunc(out, func(a, b ResourceChange) int {
		return strings.Compare(a.Resource, b.Resource)
	})
	return out
}

func sortedKeys[T any](in map[string]T) []string {
	keys := make([]string, 0, len(in))
	for key := range in {
		keys = append(keys, key)
	}
	slices.Sort(keys)
	return keys
}

func equalBoolPtr(a, b *bool) bool {
	if a == nil && b == nil {
		return true
	}
	if a == nil || b == nil {
		return false
	}
	return *a == *b
}

func equalIntPtr(a, b *int) bool {
	if a == nil && b == nil {
		return true
	}
	if a == nil || b == nil {
		return false
	}
	return *a == *b
}

func equalPorts(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if strings.TrimSpace(a[i]) != strings.TrimSpace(b[i]) {
			return false
		}
	}
	return true
}

func equalEnv(a, b map[string]string) bool {
	if len(a) != len(b) {
		return false
	}
	for key, aValue := range a {
		bValue, ok := b[key]
		if !ok {
			return false
		}
		if looksSensitiveEnvKey(key) {
			if !equalSecret(aValue, bValue) {
				return false
			}
			continue
		}
		if strings.TrimSpace(aValue) != strings.TrimSpace(bValue) {
			return false
		}
	}
	return true
}

func equalVolumeMounts(a, b []configv1.VolumeMount) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if strings.TrimSpace(a[i].Mode) != strings.TrimSpace(b[i].Mode) {
			return false
		}
		if strings.TrimSpace(a[i].Source) != strings.TrimSpace(b[i].Source) {
			return false
		}
		if strings.TrimSpace(a[i].Target) != strings.TrimSpace(b[i].Target) {
			return false
		}
		if !equalIntPtr(a[i].UID, b[i].UID) || !equalIntPtr(a[i].GID, b[i].GID) {
			return false
		}
		if strings.TrimSpace(a[i].DirMode) != strings.TrimSpace(b[i].DirMode) {
			return false
		}
	}
	return true
}

func equalBasicAuth(a, b *configv1.BasicAuth) bool {
	if a == nil && b == nil {
		return true
	}
	if a == nil || b == nil {
		return false
	}
	if len(a.Users) != len(b.Users) {
		return false
	}
	for i := range a.Users {
		if strings.TrimSpace(a.Users[i].Username) != strings.TrimSpace(b.Users[i].Username) {
			return false
		}
		if !equalSecret(a.Users[i].Password, b.Users[i].Password) {
			return false
		}
	}
	return true
}

func equalSecurity(a, b *configv1.Security) bool {
	if a == nil && b == nil {
		return true
	}
	if a == nil || b == nil {
		return false
	}
	if a.WAFEnabled != b.WAFEnabled {
		return false
	}
	if strings.TrimSpace(a.WAFMode) != strings.TrimSpace(b.WAFMode) {
		return false
	}
	if !equalStringSlices(a.IPDenylist, b.IPDenylist) {
		return false
	}
	if !equalStringSlices(a.IPAllowlist, b.IPAllowlist) {
		return false
	}
	if !equalStringSlices(a.CustomRules, b.CustomRules) {
		return false
	}
	return true
}

func equalDashboardWAF(a, b *configv1.DashboardWAF) bool {
	normalizedA := normalizeDashboardWAF(a)
	normalizedB := normalizeDashboardWAF(b)
	if normalizedA.Mode != normalizedB.Mode {
		return false
	}
	if !equalStringSlices(normalizedA.IPAllowlist, normalizedB.IPAllowlist) {
		return false
	}
	if !equalStringSlices(normalizedA.CustomRules, normalizedB.CustomRules) {
		return false
	}
	return true
}

func normalizeDashboardWAF(cfg *configv1.DashboardWAF) configv1.DashboardWAF {
	if cfg == nil {
		return configv1.DashboardWAF{Mode: "detection", IPAllowlist: []string{}, CustomRules: []string{}}
	}

	mode := strings.ToLower(strings.TrimSpace(cfg.Mode))
	if mode != "enforcement" {
		mode = "detection"
	}

	return configv1.DashboardWAF{
		Mode:        mode,
		IPAllowlist: normalizeStringList(cfg.IPAllowlist),
		CustomRules: normalizeStringList(cfg.CustomRules),
	}
}

func normalizeDashboardAccessMode(mode string) string {
	normalized := strings.ToLower(strings.TrimSpace(mode))
	switch normalized {
	case "waf_only", "waf_and_login":
		return normalized
	default:
		return "login_only"
	}
}

func normalizeStringList(in []string) []string {
	out := make([]string, 0, len(in))
	for _, value := range in {
		trimmed := strings.TrimSpace(value)
		if trimmed == "" {
			continue
		}
		out = append(out, trimmed)
	}
	return out
}

func equalStringSlices(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if strings.TrimSpace(a[i]) != strings.TrimSpace(b[i]) {
			return false
		}
	}
	return true
}

func equalSecret(a, b string) bool {
	aTrimmed := strings.TrimSpace(a)
	bTrimmed := strings.TrimSpace(b)
	if placeholderPattern.MatchString(aTrimmed) && placeholderPattern.MatchString(bTrimmed) {
		return true
	}
	return aTrimmed == bTrimmed
}

func looksSensitiveEnvKey(key string) bool {
	normalized := strings.ToUpper(strings.TrimSpace(key))
	if normalized == "" {
		return false
	}

	keywords := []string{"SECRET", "TOKEN", "PASSWORD", "DATABASE_URL", "PRIVATE_KEY", "API_KEY", "ACCESS_KEY", "_KEY", "KEY_"}
	for _, keyword := range keywords {
		if strings.Contains(normalized, keyword) {
			return true
		}
	}

	return strings.HasSuffix(normalized, "KEY")
}
