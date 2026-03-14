package configv1

import (
	"strings"
	"testing"
)

func TestDecodeStrict_RejectsUnknownFields(t *testing.T) {
	_, err := DecodeStrict(strings.NewReader(`{"apiVersion":"lotsen/v1","kind":"LotsenConfig","spec":{},"extra":true}`))
	if err == nil {
		t.Fatal("want strict decoding error for unknown top-level field")
	}
}

func TestValidate_ReportsSemanticErrors(t *testing.T) {
	public := true
	proxyPort := 8443

	doc := Document{
		APIVersion: APIVersion,
		Kind:       Kind,
		Spec: &Spec{Deployments: []Deployment{{
			Name:      "web",
			Image:     "ghcr.io/acme/web:1.2.3",
			Domain:    "app.example.com",
			Public:    &public,
			Ports:     []string{"8080:8080"},
			ProxyPort: &proxyPort,
		}}},
	}

	err := Validate(doc)
	if err == nil {
		t.Fatal("want validation error")
	}
	if !strings.Contains(err.Error(), "proxyPort") {
		t.Fatalf("want proxyPort validation error, got %v", err)
	}
}

func TestMarshalCanonical_SortsListsDeterministically(t *testing.T) {
	public := true
	doc := Document{
		APIVersion: APIVersion,
		Kind:       Kind,
		Spec: &Spec{
			Deployments: []Deployment{
				{
					Name:   "zeta",
					Image:  "ghcr.io/acme/zeta:1.0",
					Domain: "zeta.example.com",
					Public: &public,
					BasicAuth: &BasicAuth{Users: []BasicAuthUser{
						{Username: "zz", Password: "${LOTSEN_SECRET_ZZ}"},
						{Username: "aa", Password: "${LOTSEN_SECRET_AA}"},
					}},
				},
				{
					Name:   "alpha",
					Image:  "ghcr.io/acme/alpha:1.0",
					Domain: "alpha.example.com",
					Public: &public,
				},
			},
			Registries: []Registry{
				{Prefix: "z.example", Username: "z", Password: "${LOTSEN_SECRET_Z}"},
				{Prefix: "a.example", Username: "a", Password: "${LOTSEN_SECRET_A}"},
			},
		},
	}

	first, err := MarshalCanonical(doc)
	if err != nil {
		t.Fatalf("marshal canonical: %v", err)
	}
	second, err := MarshalCanonical(doc)
	if err != nil {
		t.Fatalf("marshal canonical second run: %v", err)
	}

	if string(first) != string(second) {
		t.Fatal("want deterministic canonical output across runs")
	}

	out := string(first)
	if strings.Index(out, `"name": "alpha"`) > strings.Index(out, `"name": "zeta"`) {
		t.Fatalf("want deployments sorted by name, got %s", out)
	}
	if strings.Index(out, `"prefix": "a.example"`) > strings.Index(out, `"prefix": "z.example"`) {
		t.Fatalf("want registries sorted by prefix, got %s", out)
	}
	if strings.Index(out, `"username": "aa"`) > strings.Index(out, `"username": "zz"`) {
		t.Fatalf("want nested users sorted by username, got %s", out)
	}
}
