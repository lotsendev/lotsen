package api

import (
	"encoding/json"
	"errors"
	"io"
	"net"
	"net/http"
	"strings"
)

const maxHostDisplayNameLength = 64

type HostProfile struct {
	DisplayName         string              `json:"displayName"`
	DashboardAccessMode DashboardAccessMode `json:"dashboardAccessMode,omitempty"`
	DashboardWAF        DashboardWAFConfig  `json:"dashboardWaf,omitempty"`
}

type DashboardWAFConfig struct {
	Mode        string   `json:"mode"`
	IPAllowlist []string `json:"ipAllowlist,omitempty"`
	CustomRules []string `json:"customRules,omitempty"`
}

type hostResponse struct {
	DisplayName         string                    `json:"displayName"`
	DashboardAccessMode DashboardAccessMode       `json:"dashboardAccessMode"`
	DashboardWAF        DashboardWAFConfig        `json:"dashboardWaf"`
	Metadata            *HostMetadataSystemStatus `json:"metadata,omitempty"`
}

func (h *Handler) getHost(w http.ResponseWriter, r *http.Request) {
	profile := HostProfile{}
	if h.hostProfiles != nil {
		stored, err := h.hostProfiles.Get()
		if err != nil {
			http.Error(w, "failed to load host profile", http.StatusInternalServerError)
			return
		}
		profile = stored
	}

	snapshot := h.currentSystemStatusSnapshot(r)
	mode := normalizeDashboardAccessMode(string(profile.DashboardAccessMode))
	if profile.DashboardAccessMode == "" {
		mode = h.dashboardAccessMode()
	}
	wafConfig := normalizeDashboardWAFConfig(profile.DashboardWAF)
	writeJSON(w, http.StatusOK, hostResponse{DisplayName: profile.DisplayName, DashboardAccessMode: mode, DashboardWAF: wafConfig, Metadata: snapshot.Host.Metadata})
}

func (h *Handler) updateHost(w http.ResponseWriter, r *http.Request) {
	if h.hostProfiles == nil {
		http.Error(w, "host profile unavailable", http.StatusServiceUnavailable)
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)

	var body struct {
		DisplayName         *string `json:"displayName"`
		DashboardAccessMode *string `json:"dashboardAccessMode"`
		DashboardWAF        *struct {
			Mode        *string   `json:"mode"`
			IPAllowlist *[]string `json:"ipAllowlist"`
			CustomRules *[]string `json:"customRules"`
		} `json:"dashboardWaf"`
	}

	if err := json.NewDecoder(r.Body).Decode(&body); err != nil && !errors.Is(err, io.EOF) {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	if body.DisplayName == nil && body.DashboardAccessMode == nil && body.DashboardWAF == nil {
		http.Error(w, "at least one field is required", http.StatusBadRequest)
		return
	}

	profile, err := h.hostProfiles.Get()
	if err != nil {
		http.Error(w, "failed to load host profile", http.StatusInternalServerError)
		return
	}

	if body.DisplayName != nil {
		displayName := strings.TrimSpace(*body.DisplayName)
		if len(displayName) > maxHostDisplayNameLength {
			http.Error(w, "displayName too long", http.StatusBadRequest)
			return
		}
		profile.DisplayName = displayName
	}

	if body.DashboardAccessMode != nil {
		raw := strings.ToLower(strings.TrimSpace(*body.DashboardAccessMode))
		switch DashboardAccessMode(raw) {
		case DashboardAccessModeWAFOnly, DashboardAccessModeLoginOnly, DashboardAccessModeWAFAndLogin:
			profile.DashboardAccessMode = DashboardAccessMode(raw)
		default:
			http.Error(w, "invalid dashboardAccessMode", http.StatusBadRequest)
			return
		}
	}

	if body.DashboardWAF != nil {
		next := profile.DashboardWAF

		if body.DashboardWAF.Mode != nil {
			rawMode := strings.ToLower(strings.TrimSpace(*body.DashboardWAF.Mode))
			switch rawMode {
			case "detection", "enforcement":
				next.Mode = rawMode
			default:
				http.Error(w, "invalid dashboardWaf.mode", http.StatusBadRequest)
				return
			}
		}

		if body.DashboardWAF.IPAllowlist != nil {
			next.IPAllowlist = normalizeCustomRules(*body.DashboardWAF.IPAllowlist)
			if err := validateCIDRorIPList(next.IPAllowlist); err != nil {
				http.Error(w, "invalid dashboardWaf.ipAllowlist", http.StatusBadRequest)
				return
			}
		}

		if body.DashboardWAF.CustomRules != nil {
			next.CustomRules = normalizeCustomRules(*body.DashboardWAF.CustomRules)
		}
		profile.DashboardWAF = normalizeDashboardWAFConfig(next)
	}

	updated, err := h.hostProfiles.Update(profile)
	if err != nil {
		http.Error(w, "failed to update host profile", http.StatusInternalServerError)
		return
	}

	snapshot := h.currentSystemStatusSnapshot(r)
	mode := normalizeDashboardAccessMode(string(updated.DashboardAccessMode))
	wafConfig := normalizeDashboardWAFConfig(updated.DashboardWAF)
	writeJSON(w, http.StatusOK, hostResponse{DisplayName: updated.DisplayName, DashboardAccessMode: mode, DashboardWAF: wafConfig, Metadata: snapshot.Host.Metadata})
}

func normalizeDashboardWAFConfig(cfg DashboardWAFConfig) DashboardWAFConfig {
	mode := strings.ToLower(strings.TrimSpace(cfg.Mode))
	if mode != "enforcement" {
		mode = "detection"
	}
	return DashboardWAFConfig{
		Mode:        mode,
		IPAllowlist: normalizeCustomRules(cfg.IPAllowlist),
		CustomRules: normalizeCustomRules(cfg.CustomRules),
	}
}

func normalizeCustomRules(raw []string) []string {
	out := make([]string, 0, len(raw))
	for _, rule := range raw {
		rule = strings.TrimSpace(rule)
		if rule == "" {
			continue
		}
		out = append(out, rule)
	}
	return out
}

func validateCIDRorIPList(entries []string) error {
	for _, entry := range entries {
		if _, _, err := net.ParseCIDR(entry); err == nil {
			continue
		}
		if ip := net.ParseIP(entry); ip != nil {
			continue
		}
		return errors.New("invalid cidr or ip")
	}
	return nil
}
