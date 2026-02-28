package middleware

import "strings"

type UAFilter struct {
	strict         bool
	builtInBlocked []string
	strictBlocked  []string
	customBlocked  []string
}

func NewUAFilter(strict bool, customBlocked []string) *UAFilter {
	custom := make([]string, 0, len(customBlocked))
	for _, token := range customBlocked {
		token = strings.ToLower(strings.TrimSpace(token))
		if token == "" {
			continue
		}
		custom = append(custom, token)
	}

	return &UAFilter{
		strict: strict,
		builtInBlocked: []string{
			"nikto", "sqlmap", "nuclei", "masscan", "zgrab", "wfuzz", "dirb", "gobuster",
			"headlesschrome", "phantomjs", "selenium", "webdriver", "puppeteer", "playwright",
		},
		strictBlocked: []string{
			"curl/", "wget/", "python-requests", "scrapy", "httpclient", "go-http-client",
		},
		customBlocked: custom,
	}
}

func (f *UAFilter) Blocked(userAgent string) bool {
	ua := strings.ToLower(strings.TrimSpace(userAgent))
	if ua == "" {
		return false
	}
	for _, token := range f.builtInBlocked {
		if strings.Contains(ua, token) {
			return true
		}
	}
	if f.strict {
		for _, token := range f.strictBlocked {
			if strings.Contains(ua, token) {
				return true
			}
		}
	}
	for _, token := range f.customBlocked {
		if strings.Contains(ua, token) {
			return true
		}
	}
	return false
}

func (f *UAFilter) CustomConfig() []string {
	out := make([]string, len(f.customBlocked))
	copy(out, f.customBlocked)
	return out
}
