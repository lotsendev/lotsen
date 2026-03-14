package configplan

import (
	"testing"

	"github.com/lotsendev/lotsen/internal/configv1"
)

func TestBuild_ActionGroupingAndSections(t *testing.T) {
	public := true
	desiredProxy := 8443

	desired := configv1.Document{
		APIVersion: configv1.APIVersion,
		Kind:       configv1.Kind,
		Spec: &configv1.Spec{
			Deployments: []configv1.Deployment{
				{Name: "app-create", Image: "ghcr.io/acme/new:1", Domain: "shared.example.com", Public: &public},
				{Name: "app-noop", Image: "ghcr.io/acme/noop:1", Domain: "noop.example.com", Public: &public},
				{Name: "app-update", Image: "ghcr.io/acme/app:2", Domain: "app.example.com", Public: &public, ProxyPort: &desiredProxy, Env: map[string]string{"DATABASE_URL": "${LOTSEN_SECRET_DESIRED_DB}"}},
				{Name: "keeper", Image: "ghcr.io/acme/keeper:1", Domain: "shared.example.com", Public: &public},
			},
			Registries: []configv1.Registry{
				{Prefix: "a.example", Username: "user-a", Password: "${LOTSEN_SECRET_A}"},
				{Prefix: "b.example", Username: "user-b", Password: "${LOTSEN_SECRET_B}"},
				{Prefix: "d.example", Username: "user-d-updated", Password: "${LOTSEN_SECRET_D}"},
			},
			Host: &configv1.Host{
				DisplayName:         "prod-vps-1",
				DashboardAccessMode: "waf_and_login",
				DashboardWAF: &configv1.DashboardWAF{
					Mode:        "enforcement",
					IPAllowlist: []string{"203.0.113.0/24"},
				},
			},
		},
	}

	liveProxy := 8080
	live := configv1.Document{
		APIVersion: configv1.APIVersion,
		Kind:       configv1.Kind,
		Spec: &configv1.Spec{
			Deployments: []configv1.Deployment{
				{Name: "app-noop", Image: "ghcr.io/acme/noop:1", Domain: "noop.example.com", Public: &public},
				{Name: "app-update", Image: "ghcr.io/acme/app:1", Domain: "old.example.com", Public: &public, ProxyPort: &liveProxy, Env: map[string]string{"DATABASE_URL": "${LOTSEN_SECRET_LIVE_DB}"}},
				{Name: "keeper", Image: "ghcr.io/acme/keeper:1", Domain: "shared.example.com", Public: &public},
				{Name: "legacy-delete", Image: "ghcr.io/acme/legacy:1", Domain: "legacy.example.com", Public: &public},
			},
			Registries: []configv1.Registry{
				{Prefix: "a.example", Username: "user-a", Password: "${LOTSEN_SECRET_LIVE_A}"},
				{Prefix: "c.example", Username: "user-c", Password: "${LOTSEN_SECRET_C}"},
				{Prefix: "d.example", Username: "user-d-old", Password: "${LOTSEN_SECRET_D_OLD}"},
			},
			Host: &configv1.Host{DisplayName: "old-host", DashboardAccessMode: "login_only"},
		},
	}

	plan, err := Build(desired, live)
	if err != nil {
		t.Fatalf("build plan: %v", err)
	}

	if got, want := len(plan.Actions.Deployments.Create), 1; got != want {
		t.Fatalf("want %d deployment creates, got %d", want, got)
	}
	if got := plan.Actions.Deployments.Create[0].Resource; got != "app-create" {
		t.Fatalf("want app-create create action, got %q", got)
	}

	if got, want := len(plan.Actions.Deployments.Update), 1; got != want {
		t.Fatalf("want %d deployment updates, got %d", want, got)
	}
	if got := plan.Actions.Deployments.Update[0].Resource; got != "app-update" {
		t.Fatalf("want app-update update action, got %q", got)
	}

	if got, want := len(plan.Actions.Deployments.Delete), 1; got != want {
		t.Fatalf("want %d deployment deletes, got %d", want, got)
	}
	if got := plan.Actions.Deployments.Delete[0].Resource; got != "legacy-delete" {
		t.Fatalf("want legacy-delete delete action, got %q", got)
	}

	if got, want := len(plan.Actions.Deployments.Noop), 2; got != want {
		t.Fatalf("want %d deployment noops, got %d", want, got)
	}

	if got, want := len(plan.Conflicts.Deployments), 1; got != want {
		t.Fatalf("want %d deployment conflicts, got %d", want, got)
	}
	if got := plan.Conflicts.Deployments[0].Resource; got != "app-create" {
		t.Fatalf("want app-create conflict, got %q", got)
	}

	if got, want := len(plan.Actions.Registries.Create), 1; got != want {
		t.Fatalf("want %d registry creates, got %d", want, got)
	}
	if got := plan.Actions.Registries.Create[0].Resource; got != "b.example" {
		t.Fatalf("want b.example create action, got %q", got)
	}

	if got, want := len(plan.Actions.Registries.Update), 1; got != want {
		t.Fatalf("want %d registry updates, got %d", want, got)
	}
	if got := plan.Actions.Registries.Update[0].Resource; got != "d.example" {
		t.Fatalf("want d.example update action, got %q", got)
	}

	if got, want := len(plan.Actions.Registries.Delete), 1; got != want {
		t.Fatalf("want %d registry deletes, got %d", want, got)
	}
	if got := plan.Actions.Registries.Delete[0].Resource; got != "c.example" {
		t.Fatalf("want c.example delete action, got %q", got)
	}

	if got, want := len(plan.Actions.Registries.Noop), 1; got != want {
		t.Fatalf("want %d registry noops, got %d", want, got)
	}
	if got := plan.Actions.Registries.Noop[0].Resource; got != "a.example" {
		t.Fatalf("want a.example noop action, got %q", got)
	}

	if got, want := len(plan.Actions.Host.Update), 1; got != want {
		t.Fatalf("want %d host updates, got %d", want, got)
	}

	if got, want := len(plan.Drift.Deployments), 2; got != want {
		t.Fatalf("want %d deployment drift entries, got %d", want, got)
	}
	if got, want := len(plan.Drift.Registries), 2; got != want {
		t.Fatalf("want %d registry drift entries, got %d", want, got)
	}
	if got, want := len(plan.Drift.Host), 1; got != want {
		t.Fatalf("want %d host drift entries, got %d", want, got)
	}

	if got, want := len(plan.Destructive.Deployments), 1; got != want {
		t.Fatalf("want %d destructive deployment entries, got %d", want, got)
	}
	if got, want := len(plan.Destructive.Registries), 1; got != want {
		t.Fatalf("want %d destructive registry entries, got %d", want, got)
	}

	if plan.Fingerprint == "" {
		t.Fatal("want non-empty plan fingerprint")
	}
}

func TestBuild_DeterministicFingerprint(t *testing.T) {
	public := true
	doc := configv1.Document{
		APIVersion: configv1.APIVersion,
		Kind:       configv1.Kind,
		Spec: &configv1.Spec{
			Deployments: []configv1.Deployment{{Name: "app", Image: "ghcr.io/acme/app:1", Domain: "app.example.com", Public: &public}},
			Registries:  []configv1.Registry{{Prefix: "a.example", Username: "user", Password: "${LOTSEN_SECRET_A}"}},
		},
	}

	first, err := Build(doc, doc)
	if err != nil {
		t.Fatalf("build first plan: %v", err)
	}
	second, err := Build(doc, doc)
	if err != nil {
		t.Fatalf("build second plan: %v", err)
	}

	if first.Fingerprint != second.Fingerprint {
		t.Fatalf("want deterministic fingerprints, got %q and %q", first.Fingerprint, second.Fingerprint)
	}

	b1, err := MarshalCanonical(first)
	if err != nil {
		t.Fatalf("marshal first plan: %v", err)
	}
	b2, err := MarshalCanonical(second)
	if err != nil {
		t.Fatalf("marshal second plan: %v", err)
	}

	if string(b1) != string(b2) {
		t.Fatal("want deterministic marshaled output across runs")
	}
}
