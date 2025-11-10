package oidc_test

import (
	"context"
	"errors"
	"fmt"
	"os"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	slogctx "github.com/veqryn/slog-context"

	"github.com/openkcm/session-manager/internal/oidc"
	oidcmock "github.com/openkcm/session-manager/internal/oidc/mock"
	"github.com/openkcm/session-manager/internal/serviceerr"
)

var (
	oidcProvider oidc.Provider
	newOIDCRepo  func(getErr, getForTenantErr, createErr, deleteErr, updateErr error) *oidcmock.Repository
	repo         oidc.ProviderRepository
)

const (
	requestURI = "http://cmk.example.com/ui"
	issuerURL  = "http://oidc.example.com"
	tenantID   = "tenant-id"
)

func TestMain(m *testing.M) {
	oidcProvider = oidc.Provider{
		IssuerURL: issuerURL,
		Blocked:   false,
		JWKSURIs:  []string{"http://jwks.example.com"},
		Audiences: []string{requestURI},
	}
	newOIDCRepo = func(getErr, getForTenantErr, createErr, deleteErr, updateErr error) *oidcmock.Repository {
		oidcRepo := oidcmock.NewInMemRepository(getErr, getForTenantErr, createErr, deleteErr, updateErr)
		oidcRepo.Add(tenantID, oidcProvider)

		return oidcRepo
	}

	ctx := context.Background()
	r, err := createRepo(ctx)
	if err != nil {
		slogctx.Error(ctx, "error while creating repo", "error", err)
	}

	repo = r

	code := m.Run()
	os.Exit(code)
}

func TestService_Get(t *testing.T) {
	tests := []struct {
		name         string
		issuerURL    string
		oidcRepo     *oidcmock.Repository
		wantProvider oidc.Provider
		assertErr    assert.ErrorAssertionFunc
	}{
		{
			name:         "Success",
			issuerURL:    issuerURL,
			oidcRepo:     newOIDCRepo(nil, nil, nil, nil, nil),
			wantProvider: oidcProvider,
			assertErr:    assert.NoError,
		},
		{
			name:      "Get OIDC error",
			oidcRepo:  newOIDCRepo(errors.New("Repository.Get() error"), nil, nil, nil, nil),
			issuerURL: "does-not-exist",
			assertErr: assert.Error,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := oidc.NewService(tt.oidcRepo)

			gotProvider, err := s.GetProvider(t.Context(), tt.issuerURL)
			if !tt.assertErr(t, err, fmt.Sprintf("Service.GetProvider() error %v", err)) || err != nil {
				assert.Zerof(t, gotProvider, "Service.GetProvider() extected zero value if an error is returned, got %v", gotProvider)
				return
			}

			assert.Equal(t, tt.wantProvider, gotProvider, "Service.GetProvider()")
		})
	}
}

func TestService_ApplyMapping(t *testing.T) {
	tests := []struct {
		name     string
		tenant   string
		oidcRepo *oidcmock.Repository
		wantErr  assert.ErrorAssertionFunc
	}{
		{
			name:     "Success",
			tenant:   tenantID,
			oidcRepo: newOIDCRepo(nil, nil, nil, nil, nil),
			wantErr:  assert.NoError,
		},
		{
			name:     "Create error",
			tenant:   tenantID,
			oidcRepo: newOIDCRepo(nil, errors.New("getForTenant failed"), errors.New("create failed"), nil, nil),
			wantErr:  assert.Error,
		},
		{
			name:     "Update error",
			tenant:   tenantID,
			oidcRepo: newOIDCRepo(nil, nil, nil, nil, errors.New("update failed")),
			wantErr:  assert.Error,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := oidc.NewService(tt.oidcRepo)
			err := s.ApplyMapping(t.Context(), tt.tenant, oidcProvider)
			tt.wantErr(t, err)
		})
	}
}

func TestService_BlockMapping(t *testing.T) {
	ctx := t.Context()

	t.Run("success if ", func(t *testing.T) {
		t.Run("the provider is unblocked", func(t *testing.T) {
			// given
			expTenantID := uuid.NewString()
			expUnblockedProvider := oidc.Provider{
				IssuerURL: uuid.NewString(),
				Blocked:   false,
				JWKSURIs:  []string{"http://jwks.example.com"},
				Audiences: []string{requestURI},
			}

			wrapper := &RepoWrapper{Repo: repo}
			err := wrapper.Repo.Create(ctx, expTenantID, expUnblockedProvider)
			require.NoError(t, err)
			subj := oidc.NewService(wrapper)

			// when
			err = subj.BlockMapping(ctx, expTenantID)

			// then
			assert.NoError(t, err)

			actProvider, err := wrapper.Repo.GetForTenant(ctx, expTenantID)
			assert.NoError(t, err)
			assert.True(t, actProvider.Blocked)
			assert.Equal(t, expUnblockedProvider.IssuerURL, actProvider.IssuerURL)
			assert.Equal(t, expUnblockedProvider.Audiences, actProvider.Audiences)
			assert.Equal(t, expUnblockedProvider.JWKSURIs, actProvider.JWKSURIs)
		})

		t.Run("the provider is blocked then it should not call Update", func(t *testing.T) {
			// given
			expTenantID := uuid.NewString()
			expBlockedProvider := oidc.Provider{
				IssuerURL: uuid.NewString(),
				Blocked:   true,
				JWKSURIs:  []string{"http://jwks.example.com"},
				Audiences: []string{requestURI},
			}
			repoWrapper := &RepoWrapper{Repo: repo}
			err := repoWrapper.Repo.Create(ctx, expTenantID, expBlockedProvider)
			require.NoError(t, err)

			noOfUpdateCalls := 0
			repoWrapper.MockUpdate = func(ctx context.Context, tenantID string, provider oidc.Provider) error {
				noOfUpdateCalls++
				return assert.AnError
			}
			subj := oidc.NewService(repoWrapper)

			// when
			err = subj.BlockMapping(t.Context(), expTenantID)

			// then
			assert.NoError(t, err)
			assert.Equal(t, 0, noOfUpdateCalls)

			actProvider, err := repoWrapper.Repo.GetForTenant(ctx, expTenantID)
			assert.NoError(t, err)
			assert.Equal(t, expBlockedProvider, actProvider)
		})
		t.Run("the provider is not found during the Update", func(t *testing.T) {
			// given
			expTenantID := uuid.NewString()
			expBlockedProvider := oidc.Provider{
				IssuerURL: uuid.NewString(),
				Blocked:   false,
				JWKSURIs:  []string{"http://jwks.example.com"},
				Audiences: []string{requestURI},
			}
			repoWrapper := &RepoWrapper{Repo: repo}
			err := repoWrapper.Repo.Create(ctx, expTenantID, expBlockedProvider)
			require.NoError(t, err)

			noOfUpdateCalls := 0
			repoWrapper.MockUpdate = func(ctx context.Context, tenantID string, provider oidc.Provider) error {
				noOfUpdateCalls++
				// delete the provider before updating to return an error
				err := repoWrapper.Repo.Delete(ctx, expTenantID, expBlockedProvider)
				assert.NoError(t, err)
				return nil
			}
			subj := oidc.NewService(repoWrapper)

			// when
			err = subj.BlockMapping(t.Context(), expTenantID)

			// then
			assert.NoError(t, err)
			assert.Equal(t, 1, noOfUpdateCalls)
		})
		t.Run("the provider is not found", func(t *testing.T) {
			// given
			expTenantID := uuid.NewString()
			repoWrapper := &RepoWrapper{Repo: repo}

			subj := oidc.NewService(repoWrapper)

			// when
			err := subj.BlockMapping(t.Context(), expTenantID)

			// then
			assert.NoError(t, err)
		})
	})

	t.Run("should return error", func(t *testing.T) {
		t.Run("if GetForTenant returns an error", func(t *testing.T) {
			// given
			expTenantID := uuid.NewString()
			repoWrapper := &RepoWrapper{Repo: repo}

			noOfGetForTenantCalls := 0
			repoWrapper.MockGetForTenant = func(ctx context.Context, tenantID string) (oidc.Provider, error) {
				assert.Equal(t, expTenantID, tenantID)
				noOfGetForTenantCalls++
				return oidc.Provider{}, assert.AnError
			}
			subj := oidc.NewService(repoWrapper)

			// when
			err := subj.BlockMapping(t.Context(), expTenantID)

			// then
			assert.ErrorIs(t, err, assert.AnError)
			assert.Equal(t, 1, noOfGetForTenantCalls)
		})

		t.Run("if Update returns an error", func(t *testing.T) {
			// given
			expTenantID := uuid.NewString()
			expProvider := oidc.Provider{
				IssuerURL: uuid.NewString(),
				Blocked:   false,
				JWKSURIs:  []string{"http://jwks.example.com"},
				Audiences: []string{requestURI},
			}
			repoWrapper := &RepoWrapper{Repo: repo}
			err := repoWrapper.Repo.Create(ctx, expTenantID, expProvider)
			require.NoError(t, err)

			noOfUpdateCalls := 0
			repoWrapper.MockUpdate = func(ctx context.Context, tenantID string, provider oidc.Provider) error {
				assert.Equal(t, expTenantID, tenantID)
				noOfUpdateCalls++
				return assert.AnError
			}
			subj := oidc.NewService(repoWrapper)

			// when
			err = subj.BlockMapping(t.Context(), expTenantID)

			// then
			assert.ErrorIs(t, err, assert.AnError)
			assert.Equal(t, 1, noOfUpdateCalls)

			actProvider, err := repoWrapper.Repo.GetForTenant(ctx, expTenantID)
			assert.NoError(t, err)
			assert.Equal(t, expProvider, actProvider)
		})
	})
}

func TestService_UnBlockMapping(t *testing.T) {
	ctx := t.Context()

	t.Run("success if ", func(t *testing.T) {
		t.Run("the provider is blocked", func(t *testing.T) {
			// given
			expTenantID := uuid.NewString()
			expBlockedProvider := oidc.Provider{
				IssuerURL: uuid.NewString(),
				Blocked:   true,
				JWKSURIs:  []string{"http://jwks.example.com"},
				Audiences: []string{requestURI},
			}

			wrapper := &RepoWrapper{Repo: repo}
			err := wrapper.Repo.Create(ctx, expTenantID, expBlockedProvider)
			require.NoError(t, err)
			subj := oidc.NewService(wrapper)

			// when
			err = subj.UnBlockMapping(t.Context(), expTenantID)

			// then
			assert.NoError(t, err)

			actProvider, err := wrapper.Repo.GetForTenant(ctx, expTenantID)
			assert.NoError(t, err)
			assert.False(t, actProvider.Blocked)
			assert.Equal(t, expBlockedProvider.IssuerURL, actProvider.IssuerURL)
			assert.Equal(t, expBlockedProvider.Audiences, actProvider.Audiences)
			assert.Equal(t, expBlockedProvider.JWKSURIs, actProvider.JWKSURIs)
		})

		t.Run("the provider is unblocked then it should not call Update", func(t *testing.T) {
			// given
			expTenantID := uuid.NewString()
			expUnblockedProvider := oidc.Provider{
				IssuerURL: uuid.NewString(),
				Blocked:   false,
				JWKSURIs:  []string{"http://jwks.example.com"},
				Audiences: []string{requestURI},
			}
			repoWrapper := &RepoWrapper{Repo: repo}
			err := repoWrapper.Repo.Create(ctx, expTenantID, expUnblockedProvider)
			require.NoError(t, err)

			noOfUpdateCalls := 0
			repoWrapper.MockUpdate = func(ctx context.Context, tenantID string, provider oidc.Provider) error {
				noOfUpdateCalls++
				return assert.AnError
			}
			subj := oidc.NewService(repoWrapper)

			// when
			err = subj.UnBlockMapping(t.Context(), expTenantID)

			// then
			assert.NoError(t, err)
			assert.Equal(t, 0, noOfUpdateCalls)

			actProvider, err := repoWrapper.Repo.GetForTenant(ctx, expTenantID)
			assert.NoError(t, err)
			assert.False(t, actProvider.Blocked)
		})
		t.Run("the provider is not found during the Update", func(t *testing.T) {
			// given
			expTenantID := uuid.NewString()
			expUnblockedProvider := oidc.Provider{
				IssuerURL: uuid.NewString(),
				Blocked:   true,
				JWKSURIs:  []string{"http://jwks.example.com"},
				Audiences: []string{requestURI},
			}
			repoWrapper := &RepoWrapper{Repo: repo}
			err := repoWrapper.Repo.Create(ctx, expTenantID, expUnblockedProvider)
			require.NoError(t, err)

			noOfUpdateCalls := 0
			repoWrapper.MockUpdate = func(ctx context.Context, tenantID string, provider oidc.Provider) error {
				noOfUpdateCalls++
				// delete the provider before updating to return an error
				err := repoWrapper.Repo.Delete(ctx, expTenantID, expUnblockedProvider)
				assert.NoError(t, err)
				return nil
			}
			subj := oidc.NewService(repoWrapper)

			// when
			err = subj.UnBlockMapping(t.Context(), expTenantID)

			// then
			assert.NoError(t, err)
			assert.Equal(t, 1, noOfUpdateCalls)
		})
		t.Run("the provider is not found", func(t *testing.T) {
			// given
			expTenantID := uuid.NewString()
			repoWrapper := &RepoWrapper{Repo: repo}

			subj := oidc.NewService(repoWrapper)

			// when
			err := subj.UnBlockMapping(t.Context(), expTenantID)

			// then
			assert.NoError(t, err)
		})
	})
	t.Run("should return error", func(t *testing.T) {
		t.Run("if GetForTenant returns an error", func(t *testing.T) {
			// given
			expTenantID := uuid.NewString()
			mockRepo := &RepoWrapper{Repo: repo}

			noOfGetTenantCalls := 0
			mockRepo.MockGetForTenant = func(ctx context.Context, tenantID string) (oidc.Provider, error) {
				assert.Equal(t, expTenantID, tenantID)
				noOfGetTenantCalls++
				return oidc.Provider{}, assert.AnError
			}
			subj := oidc.NewService(mockRepo)

			// when
			err := subj.UnBlockMapping(t.Context(), expTenantID)

			// then
			assert.ErrorIs(t, err, assert.AnError)
			assert.Equal(t, 1, noOfGetTenantCalls)
		})

		t.Run("if Update returns an error", func(t *testing.T) {
			// given
			expTenantIDtoUpdate := uuid.NewString()
			expBlockedProvider := oidc.Provider{
				IssuerURL: uuid.NewString(),
				Blocked:   true,
				JWKSURIs:  []string{"http://jwks.example.com"},
				Audiences: []string{requestURI},
			}
			repoWrapper := &RepoWrapper{Repo: repo}
			err := repoWrapper.Repo.Create(ctx, expTenantIDtoUpdate, expBlockedProvider)
			require.NoError(t, err)

			noOfUpdateCalls := 0
			repoWrapper.MockUpdate = func(ctx context.Context, tenantID string, provider oidc.Provider) error {
				assert.Equal(t, expTenantIDtoUpdate, tenantID)
				noOfUpdateCalls++
				return assert.AnError
			}
			subj := oidc.NewService(repoWrapper)

			// when
			err = subj.UnBlockMapping(t.Context(), expTenantIDtoUpdate)

			// then
			assert.ErrorIs(t, err, assert.AnError)
			assert.Equal(t, 1, noOfUpdateCalls)

			actProvider, err := repoWrapper.Repo.GetForTenant(ctx, expTenantIDtoUpdate)
			assert.NoError(t, err)
			assert.Equal(t, expBlockedProvider, actProvider)
		})
	})
}

func TestService_RemoveMapping(t *testing.T) {
	ctx := t.Context()

	t.Run("success if ", func(t *testing.T) {
		t.Run("the mapping and provider exist", func(t *testing.T) {
			// given
			expTenantID := uuid.NewString()
			expProvider := oidc.Provider{
				IssuerURL: uuid.NewString(),
			}

			wrapper := &RepoWrapper{Repo: repo}
			err := wrapper.Repo.Create(ctx, expTenantID, expProvider)
			require.NoError(t, err)
			subj := oidc.NewService(wrapper)

			// when
			err = subj.RemoveMapping(ctx, expTenantID)

			// then
			assert.NoError(t, err)
			_, err = wrapper.Repo.GetForTenant(ctx, expTenantID)
			assert.ErrorIs(t, err, serviceerr.ErrNotFound)
		})

		t.Run("the provider is not found", func(t *testing.T) {
			// given
			wrapper := &RepoWrapper{Repo: repo}
			noOfCalls := 0
			wrapper.MockDelete = func(ctx context.Context, tenantID string, provider oidc.Provider) error {
				noOfCalls++
				return nil
			}
			subj := oidc.NewService(wrapper)

			// when
			err := subj.RemoveMapping(ctx, uuid.NewString())

			// then
			assert.NoError(t, err)
			assert.Equal(t, 0, noOfCalls)
		})
	})

	t.Run("should return error if", func(t *testing.T) {
		t.Run("GetForTenant returns an error", func(t *testing.T) {
			// given
			expTenantID := uuid.NewString()

			wrapper := &RepoWrapper{Repo: repo}
			noOfCalls := 0
			wrapper.MockGetForTenant = func(ctx context.Context, tenantID string) (oidc.Provider, error) {
				assert.Equal(t, expTenantID, tenantID)
				noOfCalls++
				return oidc.Provider{}, assert.AnError
			}
			subj := oidc.NewService(wrapper)

			// when
			err := subj.RemoveMapping(ctx, expTenantID)

			// then
			assert.ErrorIs(t, err, assert.AnError)
			assert.Equal(t, 1, noOfCalls)
		})

		t.Run("Delete returns an error", func(t *testing.T) {
			// given
			expTenantID := uuid.NewString()
			expProvider := oidc.Provider{
				IssuerURL: uuid.NewString(),
			}

			wrapper := &RepoWrapper{Repo: repo}
			err := wrapper.Repo.Create(ctx, expTenantID, expProvider)
			require.NoError(t, err)
			noOfCalls := 0
			wrapper.MockDelete = func(ctx context.Context, tenantID string, provider oidc.Provider) error {
				assert.Equal(t, expTenantID, tenantID)
				noOfCalls++
				return assert.AnError
			}
			subj := oidc.NewService(wrapper)

			// when
			err = subj.RemoveMapping(ctx, expTenantID)

			// then
			assert.ErrorIs(t, err, assert.AnError)
			assert.Equal(t, 1, noOfCalls)
		})
	})
}
