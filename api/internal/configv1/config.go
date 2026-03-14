package configv1

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
)

const (
	APIVersion = "lotsen/v1"
	Kind       = "LotsenConfig"
)

var (
	placeholderPattern       = regexp.MustCompile(`^\$\{LOTSEN_SECRET_[A-Z0-9_]+\}$`)
	managedVolumeNamePattern = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9_.-]{0,63}$`)
	dirModePattern           = regexp.MustCompile(`^[0-7]{4}$`)
)

type Document struct {
	APIVersion string    `json:"apiVersion"`
	Kind       string    `json:"kind"`
	Metadata   *Metadata `json:"metadata,omitempty"`
	Spec       *Spec     `json:"spec"`
}

type Metadata struct {
	Name        string            `json:"name,omitempty"`
	Environment string            `json:"environment,omitempty"`
	Labels      map[string]string `json:"labels,omitempty"`
}

type Spec struct {
	Deployments []Deployment `json:"deployments,omitempty"`
	Registries  []Registry   `json:"registries,omitempty"`
	Host        *Host        `json:"host,omitempty"`
}

type Deployment struct {
	Name         string            `json:"name"`
	Image        string            `json:"image"`
	Domain       string            `json:"domain"`
	Public       *bool             `json:"public"`
	Env          map[string]string `json:"env,omitempty"`
	Ports        []string          `json:"ports,omitempty"`
	ProxyPort    *int              `json:"proxyPort,omitempty"`
	VolumeMounts []VolumeMount     `json:"volumeMounts,omitempty"`
	BasicAuth    *BasicAuth        `json:"basicAuth,omitempty"`
	Security     *Security         `json:"security,omitempty"`
}

type VolumeMount struct {
	Mode    string `json:"mode"`
	Source  string `json:"source"`
	Target  string `json:"target"`
	UID     *int   `json:"uid,omitempty"`
	GID     *int   `json:"gid,omitempty"`
	DirMode string `json:"dirMode,omitempty"`
}

type BasicAuth struct {
	Users []BasicAuthUser `json:"users"`
}

type BasicAuthUser struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

type Security struct {
	WAFEnabled  bool     `json:"wafEnabled"`
	WAFMode     string   `json:"wafMode,omitempty"`
	IPDenylist  []string `json:"ipDenylist,omitempty"`
	IPAllowlist []string `json:"ipAllowlist,omitempty"`
	CustomRules []string `json:"customRules,omitempty"`
}

type Registry struct {
	Prefix   string `json:"prefix"`
	Username string `json:"username"`
	Password string `json:"password"`
}

type Host struct {
	DisplayName         string        `json:"displayName,omitempty"`
	DashboardAccessMode string        `json:"dashboardAccessMode,omitempty"`
	DashboardWAF        *DashboardWAF `json:"dashboardWaf,omitempty"`
}

type DashboardWAF struct {
	Mode        string   `json:"mode,omitempty"`
	IPAllowlist []string `json:"ipAllowlist,omitempty"`
	CustomRules []string `json:"customRules,omitempty"`
}

func DecodeStrict(r io.Reader) (Document, error) {
	dec := json.NewDecoder(r)
	dec.DisallowUnknownFields()

	var doc Document
	if err := dec.Decode(&doc); err != nil {
		return Document{}, err
	}

	var trailing any
	if err := dec.Decode(&trailing); err != io.EOF {
		if err == nil {
			return Document{}, fmt.Errorf("expected a single JSON object")
		}
		return Document{}, err
	}

	return doc, nil
}

func MarshalCanonical(doc Document) ([]byte, error) {
	canonical := Canonicalize(doc)
	out, err := json.MarshalIndent(canonical, "", "  ")
	if err != nil {
		return nil, err
	}
	return append(out, '\n'), nil
}

func Canonicalize(doc Document) Document {
	canonical := doc

	if canonical.Spec == nil {
		return canonical
	}

	deployments := make([]Deployment, 0, len(canonical.Spec.Deployments))
	for _, deployment := range canonical.Spec.Deployments {
		copied := deployment
		if deployment.Env != nil {
			copied.Env = cloneMap(deployment.Env)
		}
		if deployment.Ports != nil {
			copied.Ports = slices.Clone(deployment.Ports)
		}
		if deployment.VolumeMounts != nil {
			copied.VolumeMounts = slices.Clone(deployment.VolumeMounts)
		}
		if deployment.BasicAuth != nil {
			users := slices.Clone(deployment.BasicAuth.Users)
			slices.SortFunc(users, func(a, b BasicAuthUser) int {
				return strings.Compare(a.Username, b.Username)
			})
			copied.BasicAuth = &BasicAuth{Users: users}
		}
		if deployment.Security != nil {
			sec := *deployment.Security
			sec.IPDenylist = slices.Clone(sec.IPDenylist)
			sec.IPAllowlist = slices.Clone(sec.IPAllowlist)
			sec.CustomRules = slices.Clone(sec.CustomRules)
			copied.Security = &sec
		}
		deployments = append(deployments, copied)
	}
	slices.SortFunc(deployments, func(a, b Deployment) int {
		return strings.Compare(a.Name, b.Name)
	})

	registries := slices.Clone(canonical.Spec.Registries)
	slices.SortFunc(registries, func(a, b Registry) int {
		return strings.Compare(a.Prefix, b.Prefix)
	})

	var host *Host
	if canonical.Spec.Host != nil {
		copied := *canonical.Spec.Host
		if canonical.Spec.Host.DashboardWAF != nil {
			waf := *canonical.Spec.Host.DashboardWAF
			waf.IPAllowlist = slices.Clone(waf.IPAllowlist)
			waf.CustomRules = slices.Clone(waf.CustomRules)
			copied.DashboardWAF = &waf
		}
		host = &copied
	}

	canonical.Spec = &Spec{Deployments: deployments, Registries: registries, Host: host}

	if canonical.Metadata != nil {
		metadata := *canonical.Metadata
		metadata.Labels = cloneMap(metadata.Labels)
		canonical.Metadata = &metadata
	}

	return canonical
}

func Validate(doc Document) error {
	errList := make([]string, 0)
	addErr := func(path string, message string) {
		errList = append(errList, fmt.Sprintf("%s: %s", path, message))
	}

	if doc.APIVersion != APIVersion {
		addErr("apiVersion", fmt.Sprintf("must equal %q", APIVersion))
	}
	if doc.Kind != Kind {
		addErr("kind", fmt.Sprintf("must equal %q", Kind))
	}
	if doc.Spec == nil {
		addErr("spec", "is required")
	}

	if doc.Metadata != nil {
		if doc.Metadata.Name != "" && len(doc.Metadata.Name) > 63 {
			addErr("metadata.name", "must be 1..63 characters")
		}
		for key, value := range doc.Metadata.Labels {
			if strings.TrimSpace(key) == "" {
				addErr("metadata.labels", "keys must be non-empty")
			}
			if strings.TrimSpace(value) == "" {
				addErr("metadata.labels."+key, "value must be non-empty")
			}
		}
	}

	if doc.Spec != nil {
		deploymentNames := make(map[string]struct{}, len(doc.Spec.Deployments))
		for i, deployment := range doc.Spec.Deployments {
			path := fmt.Sprintf("spec.deployments[%d]", i)
			if strings.TrimSpace(deployment.Name) == "" {
				addErr(path+".name", "is required")
			} else {
				if _, exists := deploymentNames[deployment.Name]; exists {
					addErr(path+".name", "must be unique")
				}
				deploymentNames[deployment.Name] = struct{}{}
			}
			if strings.TrimSpace(deployment.Image) == "" {
				addErr(path+".image", "is required")
			}
			if strings.TrimSpace(deployment.Domain) == "" {
				addErr(path+".domain", "is required")
			}
			if deployment.Public == nil {
				addErr(path+".public", "is required")
			}

			containerTCPPorts := make(map[int]struct{})
			for portIndex, rawPort := range deployment.Ports {
				spec, err := parsePortSpec(rawPort)
				if err != nil {
					addErr(fmt.Sprintf("%s.ports[%d]", path, portIndex), err.Error())
					continue
				}
				if spec.Protocol == "tcp" {
					containerTCPPorts[spec.ContainerPort] = struct{}{}
				}
			}

			if deployment.ProxyPort != nil {
				if *deployment.ProxyPort < 1 || *deployment.ProxyPort > 65535 {
					addErr(path+".proxyPort", "must be between 1 and 65535")
				} else if _, ok := containerTCPPorts[*deployment.ProxyPort]; !ok {
					addErr(path+".proxyPort", "must match a TCP container port in ports")
				}
			}

			for key, value := range deployment.Env {
				if strings.TrimSpace(key) == "" {
					addErr(path+".env", "keys must be non-empty")
					continue
				}
				if looksSensitiveEnvKey(key) && !isPlaceholder(value) {
					addErr(path+".env."+key, "must use a ${LOTSEN_SECRET_*} placeholder")
				}
			}

			for mountIndex, mount := range deployment.VolumeMounts {
				mountPath := fmt.Sprintf("%s.volumeMounts[%d]", path, mountIndex)
				mode := strings.ToLower(strings.TrimSpace(mount.Mode))
				target := filepath.Clean(strings.TrimSpace(mount.Target))
				if mode != "managed" && mode != "bind" {
					addErr(mountPath+".mode", "must be managed or bind")
				}
				if strings.TrimSpace(mount.Source) == "" {
					addErr(mountPath+".source", "is required")
				}
				if target == "." || !filepath.IsAbs(target) {
					addErr(mountPath+".target", "must be an absolute path")
				}

				if mode == "managed" {
					if !managedVolumeNamePattern.MatchString(strings.TrimSpace(mount.Source)) {
						addErr(mountPath+".source", "must match managed volume name pattern")
					}
					if mount.UID != nil && *mount.UID < 0 {
						addErr(mountPath+".uid", "must be >= 0")
					}
					if mount.GID != nil && *mount.GID < 0 {
						addErr(mountPath+".gid", "must be >= 0")
					}
					if mount.DirMode != "" && !dirModePattern.MatchString(strings.TrimSpace(mount.DirMode)) {
						addErr(mountPath+".dirMode", "must be an octal permission between 0000 and 0777")
					}
				}

				if mode == "bind" {
					if source := filepath.Clean(strings.TrimSpace(mount.Source)); source == "." || !filepath.IsAbs(source) {
						addErr(mountPath+".source", "must be an absolute path for bind mounts")
					}
					if mount.UID != nil || mount.GID != nil || strings.TrimSpace(mount.DirMode) != "" {
						addErr(mountPath, "uid, gid, and dirMode are only supported for managed mounts")
					}
				}
			}

			if deployment.BasicAuth != nil {
				if len(deployment.BasicAuth.Users) == 0 {
					addErr(path+".basicAuth.users", "must not be empty")
				}
				usernames := make(map[string]struct{}, len(deployment.BasicAuth.Users))
				for userIndex, user := range deployment.BasicAuth.Users {
					userPath := fmt.Sprintf("%s.basicAuth.users[%d]", path, userIndex)
					if strings.TrimSpace(user.Username) == "" {
						addErr(userPath+".username", "is required")
					} else {
						if _, exists := usernames[user.Username]; exists {
							addErr(userPath+".username", "must be unique")
						}
						usernames[user.Username] = struct{}{}
					}
					if !isPlaceholder(user.Password) {
						addErr(userPath+".password", "must use a ${LOTSEN_SECRET_*} placeholder")
					}
				}
			}

			if deployment.Security != nil {
				if deployment.Security.WAFMode != "" {
					mode := strings.ToLower(strings.TrimSpace(deployment.Security.WAFMode))
					if mode != "detection" && mode != "enforcement" {
						addErr(path+".security.wafMode", "must be detection or enforcement")
					}
				}

				for idx, entry := range deployment.Security.IPDenylist {
					if !isValidCIDRorIP(entry) {
						addErr(fmt.Sprintf("%s.security.ipDenylist[%d]", path, idx), "must be a valid CIDR or IP")
					}
				}
				for idx, entry := range deployment.Security.IPAllowlist {
					if !isValidCIDRorIP(entry) {
						addErr(fmt.Sprintf("%s.security.ipAllowlist[%d]", path, idx), "must be a valid CIDR or IP")
					}
				}
			}
		}

		registryPrefixes := make(map[string]struct{}, len(doc.Spec.Registries))
		for i, registry := range doc.Spec.Registries {
			path := fmt.Sprintf("spec.registries[%d]", i)
			if strings.TrimSpace(registry.Prefix) == "" {
				addErr(path+".prefix", "is required")
			} else {
				if _, exists := registryPrefixes[registry.Prefix]; exists {
					addErr(path+".prefix", "must be unique")
				}
				registryPrefixes[registry.Prefix] = struct{}{}
			}
			if strings.TrimSpace(registry.Username) == "" {
				addErr(path+".username", "is required")
			}
			if !isPlaceholder(registry.Password) {
				addErr(path+".password", "must use a ${LOTSEN_SECRET_*} placeholder")
			}
		}

		if doc.Spec.Host != nil {
			host := doc.Spec.Host
			if host.DashboardAccessMode != "" {
				mode := strings.ToLower(strings.TrimSpace(host.DashboardAccessMode))
				if mode != "login_only" && mode != "waf_only" && mode != "waf_and_login" {
					addErr("spec.host.dashboardAccessMode", "must be login_only, waf_only, or waf_and_login")
				}
			}
			if host.DashboardWAF != nil {
				if host.DashboardWAF.Mode != "" {
					mode := strings.ToLower(strings.TrimSpace(host.DashboardWAF.Mode))
					if mode != "detection" && mode != "enforcement" {
						addErr("spec.host.dashboardWaf.mode", "must be detection or enforcement")
					}
				}
				for i, entry := range host.DashboardWAF.IPAllowlist {
					if !isValidCIDRorIP(entry) {
						addErr(fmt.Sprintf("spec.host.dashboardWaf.ipAllowlist[%d]", i), "must be a valid CIDR or IP")
					}
				}
			}
		}
	}

	if len(errList) > 0 {
		return errors.New(strings.Join(errList, "; "))
	}
	return nil
}

type portSpec struct {
	HostPort      int
	ContainerPort int
	Protocol      string
}

func parsePortSpec(raw string) (portSpec, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return portSpec{}, fmt.Errorf("port mapping must not be empty")
	}

	mainPart, protocol, hasProtocol := strings.Cut(raw, "/")
	if hasProtocol {
		protocol = strings.ToLower(strings.TrimSpace(protocol))
		if protocol != "tcp" && protocol != "udp" {
			return portSpec{}, fmt.Errorf("invalid protocol %q", protocol)
		}
	} else {
		protocol = "tcp"
	}

	parts := strings.Split(mainPart, ":")
	if len(parts) > 2 {
		return portSpec{}, fmt.Errorf("invalid port mapping %q", raw)
	}

	containerPort, err := parseValidPortNumber(parts[len(parts)-1])
	if err != nil {
		return portSpec{}, err
	}

	spec := portSpec{ContainerPort: containerPort, Protocol: protocol}
	if len(parts) == 1 {
		return spec, nil
	}

	hostPort, err := parseValidPortNumber(parts[0])
	if err != nil {
		return portSpec{}, err
	}
	spec.HostPort = hostPort

	return spec, nil
}

func parseValidPortNumber(raw string) (int, error) {
	port, err := strconv.Atoi(strings.TrimSpace(raw))
	if err != nil {
		return 0, fmt.Errorf("port must be numeric")
	}
	if port < 1 || port > 65535 {
		return 0, fmt.Errorf("port must be between 1 and 65535")
	}
	return port, nil
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

func isValidCIDRorIP(value string) bool {
	value = strings.TrimSpace(value)
	if value == "" {
		return false
	}
	if _, _, err := net.ParseCIDR(value); err == nil {
		return true
	}
	return net.ParseIP(value) != nil
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
