package middleware

import (
	"fmt"
	"net"
	"strings"

	"github.com/ercadev/lotsen/store"
)

const (
	IPFilterAllowed    = ""
	IPFilterDenied     = "ip_denied"
	IPFilterNotAllowed = "ip_not_allowed"
)

type IPFilter struct {
	globalDenylist  []*net.IPNet
	globalAllowlist []*net.IPNet
}

func NewIPFilter(globalDenylist []string, globalAllowlist []string) (*IPFilter, error) {
	denylist, err := parseCIDRs(globalDenylist)
	if err != nil {
		return nil, fmt.Errorf("parse global denylist: %w", err)
	}
	allowlist, err := parseCIDRs(globalAllowlist)
	if err != nil {
		return nil, fmt.Errorf("parse global allowlist: %w", err)
	}
	return &IPFilter{globalDenylist: denylist, globalAllowlist: allowlist}, nil
}

func (f *IPFilter) EvaluateGlobal(clientIP string) string {
	return evaluateIPNetworks(clientIP, f.globalDenylist, f.globalAllowlist)
}

func (f *IPFilter) EvaluateDeployment(clientIP string, security *store.SecurityConfig) string {
	if security == nil {
		return IPFilterAllowed
	}
	denylist, err := parseCIDRs(security.IPDenylist)
	if err != nil {
		return IPFilterDenied
	}
	allowlist, err := parseCIDRs(security.IPAllowlist)
	if err != nil {
		return IPFilterNotAllowed
	}
	return evaluateIPNetworks(clientIP, denylist, allowlist)
}

func (f *IPFilter) GlobalConfig() (denylist []string, allowlist []string) {
	denylist = stringifyCIDRs(f.globalDenylist)
	allowlist = stringifyCIDRs(f.globalAllowlist)
	return denylist, allowlist
}

func evaluateIPNetworks(clientIP string, denylist []*net.IPNet, allowlist []*net.IPNet) string {
	ip := net.ParseIP(strings.TrimSpace(clientIP))
	if ip == nil {
		return IPFilterAllowed
	}
	if ipInNetworks(ip, denylist) {
		return IPFilterDenied
	}
	if len(allowlist) > 0 && !ipInNetworks(ip, allowlist) {
		return IPFilterNotAllowed
	}
	return IPFilterAllowed
}

func parseCIDRs(raw []string) ([]*net.IPNet, error) {
	nets := make([]*net.IPNet, 0, len(raw))
	for _, entry := range raw {
		entry = strings.TrimSpace(entry)
		if entry == "" {
			continue
		}
		if _, network, err := net.ParseCIDR(entry); err == nil {
			nets = append(nets, network)
			continue
		}
		ip := net.ParseIP(entry)
		if ip == nil {
			return nil, fmt.Errorf("invalid cidr or ip %q", entry)
		}
		bits := 32
		if ip.To4() == nil {
			bits = 128
		}
		_, network, err := net.ParseCIDR(fmt.Sprintf("%s/%d", ip.String(), bits))
		if err != nil {
			return nil, fmt.Errorf("invalid ip %q: %w", entry, err)
		}
		nets = append(nets, network)
	}
	return nets, nil
}

func ipInNetworks(ip net.IP, nets []*net.IPNet) bool {
	for _, network := range nets {
		if network.Contains(ip) {
			return true
		}
	}
	return false
}

func stringifyCIDRs(nets []*net.IPNet) []string {
	out := make([]string, 0, len(nets))
	for _, network := range nets {
		if network == nil {
			continue
		}
		out = append(out, network.String())
	}
	return out
}
