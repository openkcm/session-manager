package oidc_test

import (
	"context"
	"os"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	slogctx "github.com/veqryn/slog-context"

	"github.com/openkcm/session-manager/internal/oidc"
	"github.com/openkcm/session-manager/internal/serviceerr"
)

var repo oidc.ProviderRepository

const (
	requestURI = "http://cmk.example.com/ui"
	jwksURI    = "http://jwks.example.com"
)

func TestMain(m *testing.M) {
	ctx := context.Background()
	r, err := createRepo(ctx)
	if err != nil {
		slogctx.Error(ctx, "error while creating repo", "error", err)
	}

	repo = r

	code := m.Run()
	os.Exit(code)
}

func TestService_GetProvider(t *testing.T) {
	ctx := t.Context()

	t.Run("success if provider exists", func(t *testing.T) {
		expTenantID := uuid.NewString()
		expProvider := oidc.Provider{
			IssuerURL: uuid.NewString(),
			JWKSURIs:  []string{jwksURI},
			Audiences: []string{requestURI},
		}

		wrapper := &RepoWrapper{Repo: repo}
		subj := oidc.NewService(wrapper)

		err := subj.ApplyMapping(ctx, expTenantID, expProvider)
		assert.NoError(t, err)

		actProvider, err := subj.GetProvider(ctx, expProvider.IssuerURL)
		assert.NoError(t, err)
		assert.Equal(t, expProvider, actProvider)
	})

	t.Run("should return error if", func(t *testing.T) {
		t.Run("Get returns and error", func(t *testing.T) {
			expIssuer := uuid.NewString()
			wrapper := &RepoWrapper{Repo: repo}
			noOfCalls := 0
			wrapper.MockGet = func(ctx context.Context, issuerURL string) (oidc.Provider, error) {
				assert.Equal(t, expIssuer, issuerURL)
				noOfCalls++
				return oidc.Provider{}, assert.AnError
			}
			subj := oidc.NewService(wrapper)

			_, err := subj.GetProvider(ctx, expIssuer)

			assert.ErrorIs(t, err, assert.AnError)
		})

		t.Run("the provider does not exist", func(t *testing.T) {
			expIssuer := uuid.NewString()
			wrapper := &RepoWrapper{Repo: repo}
			subj := oidc.NewService(wrapper)

			_, err := subj.GetProvider(ctx, expIssuer)

			assert.ErrorIs(t, err, serviceerr.ErrNotFound)
		})
	})
}

func TestService_ApplyMapping(t *testing.T) {
	ctx := t.Context()

	t.Run("success if", func(t *testing.T) {
		t.Run("the mapping does not exist", func(t *testing.T) {
			expTenantID := uuid.NewString()
			expProvider := oidc.Provider{
				IssuerURL: uuid.NewString(),
				JWKSURIs:  []string{jwksURI},
				Audiences: []string{requestURI},
			}

			wrapper := &RepoWrapper{Repo: repo}
			subj := oidc.NewService(wrapper)

			err := subj.ApplyMapping(ctx, expTenantID, expProvider)
			assert.NoError(t, err)

			actProvider, err := wrapper.Repo.GetForTenant(ctx, expTenantID)
			assert.NoError(t, err)
			assert.Equal(t, expProvider, actProvider)
		})

		t.Run("the mapping exists", func(t *testing.T) {
			expTenantID := uuid.NewString()
			expProvider := oidc.Provider{
				IssuerURL: uuid.NewString(),
				JWKSURIs:  []string{jwksURI},
				Audiences: []string{requestURI},
			}

			wrapper := &RepoWrapper{Repo: repo}
			subj := oidc.NewService(wrapper)

			err := subj.ApplyMapping(ctx, expTenantID, expProvider)
			assert.NoError(t, err)

			expUpdatedProvider := oidc.Provider{
				IssuerURL: expProvider.IssuerURL,
				JWKSURIs:  []string{"http://updated-jwks.example.com"},
				Audiences: []string{requestURI, "http://new-aud.example.com"},
			}

			err = subj.ApplyMapping(ctx, expTenantID, expUpdatedProvider)
			assert.NoError(t, err)

			actProvider, err := wrapper.Repo.GetForTenant(ctx, expTenantID)
			assert.NoError(t, err)
			assert.Equal(t, expUpdatedProvider, actProvider)
		})
	})

	t.Run("should return error if", func(t *testing.T) {
		t.Run("Create returns an error", func(t *testing.T) {
			expTenantID := uuid.NewString()
			expProvider := oidc.Provider{
				IssuerURL: uuid.NewString(),
				JWKSURIs:  []string{jwksURI},
				Audiences: []string{requestURI},
			}

			wrapper := &RepoWrapper{Repo: repo}
			noOfCalls := 0
			wrapper.MockCreate = func(ctx context.Context, tenantID string, provider oidc.Provider) error {
				assert.Equal(t, expTenantID, tenantID)
				assert.Equal(t, expProvider, provider)
				noOfCalls++
				return assert.AnError
			}

			subj := oidc.NewService(wrapper)
			err := subj.ApplyMapping(ctx, expTenantID, expProvider)

			assert.ErrorIs(t, err, assert.AnError)
			assert.Equal(t, 1, noOfCalls)
		})

		t.Run("Update returns an error", func(t *testing.T) {
			expTenantID := uuid.NewString()
			expProvider := oidc.Provider{
				IssuerURL: uuid.NewString(),
				JWKSURIs:  []string{jwksURI},
				Audiences: []string{requestURI},
			}

			wrapper := &RepoWrapper{Repo: repo}
			noOfCalls := 0
			wrapper.MockUpdate = func(ctx context.Context, tenantID string, provider oidc.Provider) error {
				assert.Equal(t, expTenantID, tenantID)
				assert.Equal(t, expProvider, provider)
				noOfCalls++
				return assert.AnError
			}
			subj := oidc.NewService(wrapper)

			err := subj.ApplyMapping(ctx, expTenantID, expProvider)
			assert.NoError(t, err)
			err = subj.ApplyMapping(ctx, expTenantID, expProvider)

			assert.ErrorIs(t, err, assert.AnError)
			assert.Equal(t, 1, noOfCalls)
		})

		t.Run("the issuer URL has changed", func(t *testing.T) {
			expTenantID := uuid.NewString()
			expProvider := oidc.Provider{
				IssuerURL: uuid.NewString(),
				JWKSURIs:  []string{jwksURI},
				Audiences: []string{requestURI},
			}

			wrapper := &RepoWrapper{Repo: repo}
			subj := oidc.NewService(wrapper)

			err := subj.ApplyMapping(ctx, expTenantID, expProvider)
			assert.NoError(t, err)

			expUpdatedProvider := oidc.Provider{
				IssuerURL: uuid.NewString(), // changed issuer URL
				JWKSURIs:  []string{"http://updated-jwks.example.com"},
				Audiences: []string{requestURI, "http://new-aud.example.com"},
			}

			err = subj.ApplyMapping(ctx, expTenantID, expUpdatedProvider)
			assert.ErrorIs(t, err, serviceerr.ErrNotFound)
		})
	})
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
				JWKSURIs:  []string{jwksURI},
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
				JWKSURIs:  []string{jwksURI},
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
				JWKSURIs:  []string{jwksURI},
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
				JWKSURIs:  []string{jwksURI},
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
				JWKSURIs:  []string{jwksURI},
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
				JWKSURIs:  []string{jwksURI},
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
				JWKSURIs:  []string{jwksURI},
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
				JWKSURIs:  []string{jwksURI},
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
