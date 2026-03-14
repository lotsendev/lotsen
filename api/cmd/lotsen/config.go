package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	internalapi "github.com/lotsendev/lotsen/internal/api"
	"github.com/lotsendev/lotsen/internal/configplan"
	"github.com/lotsendev/lotsen/internal/configv1"
	"github.com/lotsendev/lotsen/store"
)

var (
	placeholderCleanupPattern = regexp.MustCompile(`[^A-Z0-9]+`)
	placeholderPattern        = regexp.MustCompile(`^\$\{LOTSEN_SECRET_[A-Z0-9_]+\}$`)
	managedVolumeNamePattern  = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9_.-]{0,63}$`)
)

func runConfig(args []string, stdout io.Writer) error {
	if len(args) == 0 {
		return errors.New("usage: lotsen config <validate|fmt|export|plan> [flags]")
	}

	switch args[0] {
	case "validate":
		return runConfigValidate(args[1:], stdout)
	case "fmt":
		return runConfigFmt(args[1:], stdout)
	case "export":
		return runConfigExport(args[1:])
	case "plan":
		return runConfigPlan(args[1:])
	default:
		return fmt.Errorf("unknown config command %q", args[0])
	}
}

func runConfigPlan(args []string) error {
	fs := flag.NewFlagSet("config plan", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	configPath := fs.String("f", "", "Path to config file")
	outPath := fs.String("out", "", "Path to plan file")
	if err := fs.Parse(args); err != nil {
		return fmt.Errorf("%w\n\nUsage: lotsen config plan -f <file> --out <plan-file>", err)
	}

	if strings.TrimSpace(*configPath) == "" || strings.TrimSpace(*outPath) == "" {
		return errors.New("Usage: lotsen config plan -f <file> --out <plan-file>")
	}

	desired, err := readConfigFile(*configPath)
	if err != nil {
		return err
	}

	if err := configv1.Validate(desired); err != nil {
		return fmt.Errorf("config validation failed: %w", err)
	}

	live, err := exportConfigDocument()
	if err != nil {
		return fmt.Errorf("export live state: %w", err)
	}

	planDoc, err := configplan.Build(desired, live)
	if err != nil {
		return fmt.Errorf("build plan: %w", err)
	}

	formatted, err := configplan.MarshalCanonical(planDoc)
	if err != nil {
		return fmt.Errorf("marshal plan: %w", err)
	}

	if err := os.WriteFile(*outPath, formatted, 0o644); err != nil {
		return fmt.Errorf("write plan file: %w", err)
	}

	return nil
}

func runConfigValidate(args []string, stdout io.Writer) error {
	fs := flag.NewFlagSet("config validate", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	path := fs.String("f", "", "Path to config file")
	if err := fs.Parse(args); err != nil {
		return fmt.Errorf("%w\n\nUsage: lotsen config validate -f <file>", err)
	}

	if strings.TrimSpace(*path) == "" {
		return errors.New("Usage: lotsen config validate -f <file>")
	}

	doc, err := readConfigFile(*path)
	if err != nil {
		return err
	}

	if err := configv1.Validate(doc); err != nil {
		return fmt.Errorf("config validation failed: %w", err)
	}

	if _, err := fmt.Fprintln(stdout, "config is valid"); err != nil {
		return err
	}

	return nil
}

func runConfigFmt(args []string, stdout io.Writer) error {
	fs := flag.NewFlagSet("config fmt", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	path := fs.String("f", "", "Path to config file")
	if err := fs.Parse(args); err != nil {
		return fmt.Errorf("%w\n\nUsage: lotsen config fmt -f <file>", err)
	}

	if strings.TrimSpace(*path) == "" {
		return errors.New("Usage: lotsen config fmt -f <file>")
	}

	doc, err := readConfigFile(*path)
	if err != nil {
		return err
	}

	if err := configv1.Validate(doc); err != nil {
		return fmt.Errorf("config validation failed: %w", err)
	}

	formatted, err := configv1.MarshalCanonical(doc)
	if err != nil {
		return fmt.Errorf("format config: %w", err)
	}

	if _, err := stdout.Write(formatted); err != nil {
		return err
	}

	return nil
}

func runConfigExport(args []string) error {
	fs := flag.NewFlagSet("config export", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	path := fs.String("o", "", "Path to output file")
	if err := fs.Parse(args); err != nil {
		return fmt.Errorf("%w\n\nUsage: lotsen config export -o <file>", err)
	}

	if strings.TrimSpace(*path) == "" {
		return errors.New("Usage: lotsen config export -o <file>")
	}

	doc, err := exportConfigDocument()
	if err != nil {
		return err
	}

	formatted, err := configv1.MarshalCanonical(doc)
	if err != nil {
		return fmt.Errorf("marshal exported config: %w", err)
	}

	if err := os.WriteFile(*path, formatted, 0o644); err != nil {
		return fmt.Errorf("write export file: %w", err)
	}

	return nil
}

func readConfigFile(path string) (configv1.Document, error) {
	f, err := os.Open(path)
	if err != nil {
		return configv1.Document{}, fmt.Errorf("open config file: %w", err)
	}
	defer f.Close()

	doc, err := configv1.DecodeStrict(f)
	if err != nil {
		return configv1.Document{}, fmt.Errorf("decode config file: %w", err)
	}

	return doc, nil
}

func exportConfigDocument() (configv1.Document, error) {
	storePath := dataPath()
	jsonStore, err := store.NewJSONStore(storePath)
	if err != nil {
		return configv1.Document{}, fmt.Errorf("open store: %w", err)
	}

	deployments, err := jsonStore.List()
	if err != nil {
		return configv1.Document{}, fmt.Errorf("list deployments: %w", err)
	}

	registryExports, err := jsonStore.ListRegistriesForExport()
	if err != nil {
		return configv1.Document{}, fmt.Errorf("list registries: %w", err)
	}

	hostProfileStore, err := internalapi.NewFileHostProfileStore(hostProfilePath(storePath))
	if err != nil {
		return configv1.Document{}, fmt.Errorf("open host profile store: %w", err)
	}

	hostProfile, err := hostProfileStore.Get()
	if err != nil {
		return configv1.Document{}, fmt.Errorf("get host profile: %w", err)
	}

	doc := configv1.Document{
		APIVersion: configv1.APIVersion,
		Kind:       configv1.Kind,
		Spec: &configv1.Spec{
			Deployments: make([]configv1.Deployment, 0, len(deployments)),
			Registries:  make([]configv1.Registry, 0, len(registryExports)),
		},
	}

	for _, deployment := range deployments {
		public := deployment.Public
		entry := configv1.Deployment{
			Name:      deployment.Name,
			Image:     deployment.Image,
			Domain:    deployment.Domain,
			Public:    &public,
			Env:       exportEnvMap(deployment.Name, deployment.Envs),
			Ports:     append([]string{}, deployment.Ports...),
			Security:  exportSecurity(deployment.Security),
			BasicAuth: exportBasicAuth(deployment.Name, deployment.BasicAuth),
		}

		if deployment.ProxyPort > 0 {
			proxyPort := deployment.ProxyPort
			entry.ProxyPort = &proxyPort
		}

		entry.VolumeMounts = exportVolumeMounts(deployment.ID, deployment.Volumes)
		doc.Spec.Deployments = append(doc.Spec.Deployments, entry)
	}

	for _, registry := range registryExports {
		password := registry.Password
		if !isPlaceholder(password) {
			password = placeholderFor("REGISTRY_" + registry.Prefix)
		}
		doc.Spec.Registries = append(doc.Spec.Registries, configv1.Registry{
			Prefix:   registry.Prefix,
			Username: registry.Username,
			Password: password,
		})
	}

	host := configv1.Host{
		DisplayName:         strings.TrimSpace(hostProfile.DisplayName),
		DashboardAccessMode: strings.TrimSpace(string(hostProfile.DashboardAccessMode)),
	}

	if waf := hostProfile.DashboardWAF; waf.Mode != "" || len(waf.IPAllowlist) > 0 || len(waf.CustomRules) > 0 {
		host.DashboardWAF = &configv1.DashboardWAF{
			Mode:        strings.TrimSpace(waf.Mode),
			IPAllowlist: append([]string{}, waf.IPAllowlist...),
			CustomRules: append([]string{}, waf.CustomRules...),
		}
	}

	doc.Spec.Host = &host

	if err := configv1.Validate(doc); err != nil {
		return configv1.Document{}, fmt.Errorf("validate exported config: %w", err)
	}

	return doc, nil
}

func exportSecurity(security *store.SecurityConfig) *configv1.Security {
	if security == nil {
		return nil
	}

	return &configv1.Security{
		WAFEnabled:  security.WAFEnabled,
		WAFMode:     security.WAFMode,
		IPDenylist:  append([]string{}, security.IPDenylist...),
		IPAllowlist: append([]string{}, security.IPAllowlist...),
		CustomRules: append([]string{}, security.CustomRules...),
	}
}

func exportBasicAuth(deploymentName string, auth *store.BasicAuthConfig) *configv1.BasicAuth {
	if auth == nil {
		return nil
	}

	users := make([]configv1.BasicAuthUser, 0, len(auth.Users))
	for _, user := range auth.Users {
		password := user.Password
		if !isPlaceholder(password) {
			password = placeholderFor("BASIC_AUTH_" + deploymentName + "_" + user.Username)
		}
		users = append(users, configv1.BasicAuthUser{Username: user.Username, Password: password})
	}

	return &configv1.BasicAuth{Users: users}
}

func exportEnvMap(deploymentName string, in map[string]string) map[string]string {
	if in == nil {
		return nil
	}

	out := make(map[string]string, len(in))
	for key, value := range in {
		if looksSensitiveEnvKey(key) && !isPlaceholder(value) {
			out[key] = placeholderFor("ENV_" + deploymentName + "_" + key)
			continue
		}
		out[key] = value
	}

	return out
}

func exportVolumeMounts(deploymentID string, bindings []string) []configv1.VolumeMount {
	if len(bindings) == 0 {
		return nil
	}

	mounts := make([]configv1.VolumeMount, 0, len(bindings))
	for _, binding := range bindings {
		sep := strings.IndexByte(binding, ':')
		if sep <= 0 {
			continue
		}

		source := binding[:sep]
		target := binding[sep+1:]

		if managedName, ok := managedVolumeNameForDeployment(deploymentID, source); ok {
			mounts = append(mounts, configv1.VolumeMount{Mode: "managed", Source: managedName, Target: target})
			continue
		}

		mounts = append(mounts, configv1.VolumeMount{Mode: "bind", Source: source, Target: target})
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

func managedVolumesBaseDirFromEnv() string {
	if dir := strings.TrimSpace(os.Getenv("LOTSEN_MANAGED_VOLUMES_DIR")); dir != "" {
		return filepath.Clean(dir)
	}
	return "/var/lib/lotsen/volumes"
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

func isPlaceholder(value string) bool {
	return placeholderPattern.MatchString(strings.TrimSpace(value))
}

func placeholderFor(raw string) string {
	normalized := strings.ToUpper(strings.TrimSpace(raw))
	normalized = placeholderCleanupPattern.ReplaceAllString(normalized, "_")
	normalized = strings.Trim(normalized, "_")
	if normalized == "" {
		normalized = "VALUE"
	}
	return "${LOTSEN_SECRET_" + normalized + "}"
}
