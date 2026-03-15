package configapply

import (
	"errors"
	"testing"

	internalapi "github.com/lotsendev/lotsen/internal/api"
	"github.com/lotsendev/lotsen/internal/configplan"
	"github.com/lotsendev/lotsen/internal/configv1"
	"github.com/lotsendev/lotsen/store"
)

func TestExecute_ContinuesAfterFailureAndReportsPartial(t *testing.T) {
	public := true
	desired := configv1.Document{
		APIVersion: configv1.APIVersion,
		Kind:       configv1.Kind,
		Spec: &configv1.Spec{
			Deployments: []configv1.Deployment{{Name: "app", Image: "ghcr.io/acme/app:2", Domain: "app.example.com", Public: &public}},
			Registries:  []configv1.Registry{{Prefix: "a.example", Username: "user", Password: "${LOTSEN_SECRET_A}"}},
			Host:        &configv1.Host{DisplayName: "host-a", DashboardAccessMode: "waf_and_login"},
		},
	}

	plan := configplan.Document{
		Fingerprint: "sha256:test",
		Actions: configplan.Actions{
			Deployments: configplan.ResourceActions{
				Update: []configplan.ResourceChange{{Resource: "app", Changes: []string{"image"}}},
			},
			Registries: configplan.ResourceActions{
				Update: []configplan.ResourceChange{{Resource: "a.example", Changes: []string{"username"}}},
			},
			Host: configplan.HostActions{
				Update: []configplan.ResourceChange{{Resource: "host"}},
			},
		},
	}

	store := &stubStore{
		deployments:        []store.Deployment{{ID: "dep-1", Name: "app", Image: "ghcr.io/acme/app:1", Domain: "app.example.com", Public: true, Status: store.StatusHealthy}},
		registries:         []store.RegistryEntry{{ID: "reg-1", Prefix: "a.example", Username: "old-user"}},
		failRegistryUpdate: true,
	}
	hostStore := &stubHostStore{}

	result, err := Execute(desired, plan, store, hostStore)
	if err == nil {
		t.Fatal("want partial failure error")
	}
	if result.Summary.Failed != 1 {
		t.Fatalf("want 1 failure, got %d", result.Summary.Failed)
	}
	if result.Summary.Applied != 2 {
		t.Fatalf("want 2 applied outcomes, got %d", result.Summary.Applied)
	}
	if !result.PartialFailed {
		t.Fatal("want partialFailed=true")
	}

	if len(store.updatedDeployments) != 1 {
		t.Fatalf("want deployment update to continue, got %d updates", len(store.updatedDeployments))
	}
	if hostStore.updated.DisplayName != "host-a" {
		t.Fatalf("want host update to continue, got %#v", hostStore.updated)
	}
}

type stubStore struct {
	deployments []store.Deployment
	registries  []store.RegistryEntry

	updatedDeployments []store.Deployment

	failRegistryUpdate bool
}

func (s *stubStore) List() ([]store.Deployment, error) { return s.deployments, nil }
func (s *stubStore) Create(d store.Deployment) (store.Deployment, error) {
	return d, nil
}
func (s *stubStore) Update(d store.Deployment) (store.Deployment, error) {
	s.updatedDeployments = append(s.updatedDeployments, d)
	return d, nil
}
func (s *stubStore) Delete(_ string) error { return nil }

func (s *stubStore) ListRegistries() ([]store.RegistryEntry, error) {
	return s.registries, nil
}
func (s *stubStore) CreateRegistry(id, prefix, username, password string) (store.RegistryEntry, error) {
	return store.RegistryEntry{ID: id, Prefix: prefix, Username: username}, nil
}
func (s *stubStore) UpdateRegistry(id, prefix, username, password string) (store.RegistryEntry, error) {
	if s.failRegistryUpdate {
		return store.RegistryEntry{}, errors.New("update failed")
	}
	return store.RegistryEntry{ID: id, Prefix: prefix, Username: username}, nil
}
func (s *stubStore) DeleteRegistry(_ string) error { return nil }

type stubHostStore struct {
	updated internalapi.HostProfile
}

func (s *stubHostStore) Update(profile internalapi.HostProfile) (internalapi.HostProfile, error) {
	s.updated = profile
	return profile, nil
}
