package oidctrust_test

import (
	"context"
	"errors"
	"os"
	"testing"

	"github.com/gofrs/uuid/v5"
	"github.com/google/go-cmp/cmp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/testing/protocmp"

	oidcv1 "github.com/openkcm/api-sdk/proto/kms/api/cmk/trust/oidc/v1"
	trustv1 "github.com/openkcm/api-sdk/proto/kms/api/cmk/trust/v1"
	slogctx "github.com/veqryn/slog-context"

	"github.com/openkcm/session-manager/modules/oidctrust"
	mocktrust "github.com/openkcm/session-manager/modules/oidctrust/mocks"
)

var repo oidctrust.TrustRepository

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

func TestService_Apply(t *testing.T) {
	ctx := t.Context()

	t.Run("success if", func(t *testing.T) {
		t.Run("the trust does not exist", func(t *testing.T) {
			expTenantID := uuid.Must(uuid.NewV4()).String()
			expTrust := trustv1.Trust_builder{
				TenantId: new(expTenantID),
				Blocked:  new(false),
				Oidc: oidcv1.OIDC_builder{
					Issuer:    new(uuid.Must(uuid.NewV4()).String()),
					JwksUri:   new(jwksURI),
					Audiences: []string{requestURI},
				}.Build(),
			}.Build()

			wrapper := &RepoWrapper{Repo: repo}
			subj := oidctrust.NewModule(wrapper)

			err := subj.Apply(ctx, expTrust)
			assert.NoError(t, err)

			actTrust, err := wrapper.Repo.Get(ctx, expTenantID)
			assert.NoError(t, err)
			if diff := cmp.Diff(expTrust, actTrust, protocmp.Transform()); diff != "" {
				t.Fatalf("trust not equal:\n%s", diff)
			}
		})

		t.Run("the trust exists", func(t *testing.T) {
			expTenantID := uuid.Must(uuid.NewV4()).String()
			expTrust := trustv1.Trust_builder{
				TenantId: new(expTenantID),
				Blocked:  new(false),
				Oidc: oidcv1.OIDC_builder{
					Issuer:    new(uuid.Must(uuid.NewV4()).String()),
					JwksUri:   new(jwksURI),
					Audiences: []string{requestURI},
				}.Build(),
			}.Build()

			wrapper := &RepoWrapper{Repo: repo}
			subj := oidctrust.NewModule(wrapper)

			err := subj.Apply(ctx, expTrust)
			assert.NoError(t, err)

			expUpdatedTrust := trustv1.Trust_builder{
				TenantId: new(expTenantID),
				Blocked:  new(false),
				Oidc: oidcv1.OIDC_builder{
					Issuer:    new(expTrust.GetOidc().GetIssuer()),
					JwksUri:   new("http://updated-jwks.example.com"),
					Audiences: []string{requestURI, "http://new-aud.example.com"},
				}.Build(),
			}.Build()

			err = subj.Apply(ctx, expUpdatedTrust)
			assert.NoError(t, err)

			actTrust, err := wrapper.Repo.Get(ctx, expTenantID)
			assert.NoError(t, err)
			if diff := cmp.Diff(expUpdatedTrust, actTrust, protocmp.Transform()); diff != "" {
				t.Fatalf("trust not equal:\n%s", diff)
			}
		})
	})

	t.Run("should return error if", func(t *testing.T) {
		t.Run("Create returns an error", func(t *testing.T) {
			expTenantID := uuid.Must(uuid.NewV4()).String()
			expTrust := trustv1.Trust_builder{
				TenantId: new(expTenantID),
				Oidc: oidcv1.OIDC_builder{
					Issuer:    new(uuid.Must(uuid.NewV4()).String()),
					JwksUri:   new(jwksURI),
					Audiences: []string{requestURI},
				}.Build(),
			}.Build()

			wrapper := &RepoWrapper{Repo: repo}
			noOfCalls := 0
			wrapper.MockCreate = func(ctx context.Context, trust *trustv1.Trust) error {
				assert.Equal(t, expTenantID, trust.GetTenantId())
				assert.Equal(t, expTrust, trust)
				noOfCalls++
				return assert.AnError
			}

			subj := oidctrust.NewModule(wrapper)
			err := subj.Apply(ctx, expTrust)

			assert.ErrorIs(t, err, assert.AnError)
			assert.Equal(t, 1, noOfCalls)
		})

		t.Run("Update returns an error", func(t *testing.T) {
			expTenantID := uuid.Must(uuid.NewV4()).String()
			expTrust := trustv1.Trust_builder{
				TenantId: new(expTenantID),
				Oidc: oidcv1.OIDC_builder{
					Issuer:    new(uuid.Must(uuid.NewV4()).String()),
					JwksUri:   new(jwksURI),
					Audiences: []string{requestURI},
				}.Build(),
			}.Build()

			wrapper := &RepoWrapper{Repo: repo}
			noOfCalls := 0
			wrapper.MockUpdate = func(ctx context.Context, trust *trustv1.Trust) error {
				assert.Equal(t, expTenantID, trust.GetTenantId())
				assert.Equal(t, expTrust, trust)
				noOfCalls++
				return assert.AnError
			}
			subj := oidctrust.NewModule(wrapper)

			err := subj.Apply(ctx, expTrust)
			assert.NoError(t, err)
			err = subj.Apply(ctx, expTrust)

			assert.ErrorIs(t, err, assert.AnError)
			assert.Equal(t, 1, noOfCalls)
		})
	})
}

func TestService_Block(t *testing.T) {
	ctx := t.Context()

	t.Run("success if ", func(t *testing.T) {
		t.Run("the trust is unblocked", func(t *testing.T) {
			// given
			expTenantID := uuid.Must(uuid.NewV4()).String()
			expUnblockedTrust := trustv1.Trust_builder{
				TenantId: new(expTenantID),
				Blocked:  new(false),
				Oidc: oidcv1.OIDC_builder{
					Issuer:    new(uuid.Must(uuid.NewV4()).String()),
					JwksUri:   new(jwksURI),
					Audiences: []string{requestURI},
				}.Build(),
			}.Build()

			wrapper := &RepoWrapper{Repo: repo}
			err := wrapper.Repo.Create(ctx, expUnblockedTrust)
			require.NoError(t, err)
			subj := oidctrust.NewModule(wrapper)

			// when
			err = subj.Block(ctx, expTenantID)

			// then
			assert.NoError(t, err)

			actTrust, err := wrapper.Repo.Get(ctx, expTenantID)
			assert.NoError(t, err)
			assert.True(t, actTrust.GetBlocked())
			assert.Equal(t, expUnblockedTrust.GetOidc().GetIssuer(), actTrust.GetOidc().GetIssuer())
			assert.Equal(t, expUnblockedTrust.GetOidc().GetAudiences(), actTrust.GetOidc().GetAudiences())
			assert.Equal(t, expUnblockedTrust.GetOidc().GetJwksUri(), actTrust.GetOidc().GetJwksUri())
		})

		t.Run("the trust is blocked then it should not call Update", func(t *testing.T) {
			// given
			expTenantID := uuid.Must(uuid.NewV4()).String()
			expBlockedTrust := trustv1.Trust_builder{
				TenantId: new(expTenantID),
				Blocked:  new(true),
				Oidc: oidcv1.OIDC_builder{
					Issuer:    new(uuid.Must(uuid.NewV4()).String()),
					JwksUri:   new(jwksURI),
					Audiences: []string{requestURI},
				}.Build(),
			}.Build()
			repoWrapper := &RepoWrapper{Repo: repo}
			err := repoWrapper.Repo.Create(ctx, expBlockedTrust)
			require.NoError(t, err)

			noOfUpdateCalls := 0
			repoWrapper.MockUpdate = func(ctx context.Context, trust *trustv1.Trust) error {
				noOfUpdateCalls++
				return assert.AnError
			}
			subj := oidctrust.NewModule(repoWrapper)

			// when
			err = subj.Block(t.Context(), expTenantID)

			// then
			assert.NoError(t, err)
			assert.Equal(t, 0, noOfUpdateCalls)

			actTrust, err := repoWrapper.Repo.Get(ctx, expTenantID)
			assert.NoError(t, err)
			assert.Equal(t, expBlockedTrust, actTrust)
		})
		t.Run("the trust is not found during the Update", func(t *testing.T) {
			// given
			expTenantID := uuid.Must(uuid.NewV4()).String()
			expBlockedTrust := trustv1.Trust_builder{
				TenantId: new(expTenantID),
				Blocked:  new(false),
				Oidc: oidcv1.OIDC_builder{
					Issuer:    new(uuid.Must(uuid.NewV4()).String()),
					JwksUri:   new(jwksURI),
					Audiences: []string{requestURI},
				}.Build(),
			}.Build()
			repoWrapper := &RepoWrapper{Repo: repo}
			err := repoWrapper.Repo.Create(ctx, expBlockedTrust)
			require.NoError(t, err)

			noOfUpdateCalls := 0
			repoWrapper.MockUpdate = func(ctx context.Context, trust *trustv1.Trust) error {
				noOfUpdateCalls++
				// delete the trust before updating to return an error
				err := repoWrapper.Repo.Delete(ctx, expTenantID)
				assert.NoError(t, err)
				return nil
			}
			subj := oidctrust.NewModule(repoWrapper)

			// when
			err = subj.Block(t.Context(), expTenantID)

			// then
			assert.NoError(t, err)
			assert.Equal(t, 1, noOfUpdateCalls)
		})
		t.Run("the trust is not found", func(t *testing.T) {
			// given
			expTenantID := uuid.Must(uuid.NewV4()).String()
			repoWrapper := &RepoWrapper{Repo: repo}

			subj := oidctrust.NewModule(repoWrapper)

			// when
			err := subj.Block(t.Context(), expTenantID)

			// then
			assert.NoError(t, err)
		})
	})

	t.Run("should return error", func(t *testing.T) {
		t.Run("if Get returns an error", func(t *testing.T) {
			// given
			expTenantID := uuid.Must(uuid.NewV4()).String()
			repoWrapper := &RepoWrapper{Repo: repo}

			noOfGetCalls := 0
			repoWrapper.MockGet = func(ctx context.Context, tenantID string) (*trustv1.Trust, error) {
				assert.Equal(t, expTenantID, tenantID)
				noOfGetCalls++
				return trustv1.Trust_builder{
					Oidc: oidcv1.OIDC_builder{}.Build(),
				}.Build(), assert.AnError
			}
			subj := oidctrust.NewModule(repoWrapper)

			// when
			err := subj.Block(t.Context(), expTenantID)

			// then
			assert.ErrorIs(t, err, assert.AnError)
			assert.Equal(t, 1, noOfGetCalls)
		})

		t.Run("if Update returns an error", func(t *testing.T) {
			// given
			expTenantID := uuid.Must(uuid.NewV4()).String()
			expTrust := trustv1.Trust_builder{
				TenantId: new(expTenantID),
				Blocked:  new(false),
				Oidc: oidcv1.OIDC_builder{
					Issuer:    new(uuid.Must(uuid.NewV4()).String()),
					JwksUri:   new(jwksURI),
					Audiences: []string{requestURI},
				}.Build(),
			}.Build()
			repoWrapper := &RepoWrapper{Repo: repo}
			err := repoWrapper.Repo.Create(ctx, expTrust)
			require.NoError(t, err)

			noOfUpdateCalls := 0
			repoWrapper.MockUpdate = func(ctx context.Context, trust *trustv1.Trust) error {
				assert.Equal(t, expTenantID, trust.GetTenantId())
				noOfUpdateCalls++
				return assert.AnError
			}
			subj := oidctrust.NewModule(repoWrapper)

			// when
			err = subj.Block(t.Context(), expTenantID)

			// then
			assert.ErrorIs(t, err, assert.AnError)
			assert.Equal(t, 1, noOfUpdateCalls)

			actTrust, err := repoWrapper.Repo.Get(ctx, expTenantID)
			assert.NoError(t, err)
			assert.Equal(t, expTrust, actTrust)
		})
	})
}

func TestService_Unblock(t *testing.T) {
	ctx := t.Context()

	t.Run("success if ", func(t *testing.T) {
		t.Run("the trust is blocked", func(t *testing.T) {
			// given
			expTenantID := uuid.Must(uuid.NewV4()).String()
			expBlockedTrust := trustv1.Trust_builder{
				TenantId: new(expTenantID),
				Blocked:  new(true),
				Oidc: oidcv1.OIDC_builder{
					Issuer:    new(uuid.Must(uuid.NewV4()).String()),
					JwksUri:   new(jwksURI),
					Audiences: []string{requestURI},
				}.Build(),
			}.Build()

			wrapper := &RepoWrapper{Repo: repo}
			err := wrapper.Repo.Create(ctx, expBlockedTrust)
			require.NoError(t, err)
			subj := oidctrust.NewModule(wrapper)

			// when
			err = subj.Unblock(t.Context(), expTenantID)

			// then
			assert.NoError(t, err)

			actTrust, err := wrapper.Repo.Get(ctx, expTenantID)
			assert.NoError(t, err)
			assert.False(t, actTrust.GetBlocked())
			assert.Equal(t, expBlockedTrust.GetOidc().GetIssuer(), actTrust.GetOidc().GetIssuer())
			assert.Equal(t, expBlockedTrust.GetOidc().GetAudiences(), actTrust.GetOidc().GetAudiences())
			assert.Equal(t, expBlockedTrust.GetOidc().GetJwksUri(), actTrust.GetOidc().GetJwksUri())
		})

		t.Run("the trust is unblocked then it should not call Update", func(t *testing.T) {
			// given
			expTenantID := uuid.Must(uuid.NewV4()).String()
			expUnblockedTrust := trustv1.Trust_builder{
				TenantId: new(expTenantID),
				Blocked:  new(false),
				Oidc: oidcv1.OIDC_builder{
					Issuer:    new(uuid.Must(uuid.NewV4()).String()),
					JwksUri:   new(jwksURI),
					Audiences: []string{requestURI},
				}.Build(),
			}.Build()
			repoWrapper := &RepoWrapper{Repo: repo}
			err := repoWrapper.Repo.Create(ctx, expUnblockedTrust)
			require.NoError(t, err)

			noOfUpdateCalls := 0
			repoWrapper.MockUpdate = func(ctx context.Context, trust *trustv1.Trust) error {
				noOfUpdateCalls++
				return assert.AnError
			}
			subj := oidctrust.NewModule(repoWrapper)

			// when
			err = subj.Unblock(t.Context(), expTenantID)

			// then
			assert.NoError(t, err)
			assert.Equal(t, 0, noOfUpdateCalls)

			actTrust, err := repoWrapper.Repo.Get(ctx, expTenantID)
			assert.NoError(t, err)
			assert.False(t, actTrust.GetBlocked())
		})
		t.Run("the trust is not found during the Update", func(t *testing.T) {
			// given
			expTenantID := uuid.Must(uuid.NewV4()).String()
			expUnblockedTrust := trustv1.Trust_builder{
				TenantId: new(expTenantID),
				Blocked:  new(true),
				Oidc: oidcv1.OIDC_builder{
					Issuer:    new(uuid.Must(uuid.NewV4()).String()),
					JwksUri:   new(jwksURI),
					Audiences: []string{requestURI},
				}.Build(),
			}.Build()
			repoWrapper := &RepoWrapper{Repo: repo}
			err := repoWrapper.Repo.Create(ctx, expUnblockedTrust)
			require.NoError(t, err)

			noOfUpdateCalls := 0
			repoWrapper.MockUpdate = func(ctx context.Context, trust *trustv1.Trust) error {
				noOfUpdateCalls++
				// delete the trust before updating to return an error
				err := repoWrapper.Repo.Delete(ctx, expTenantID)
				assert.NoError(t, err)
				return nil
			}
			subj := oidctrust.NewModule(repoWrapper)

			// when
			err = subj.Unblock(t.Context(), expTenantID)

			// then
			assert.NoError(t, err)
			assert.Equal(t, 1, noOfUpdateCalls)
		})
		t.Run("the trust is not found", func(t *testing.T) {
			// given
			expTenantID := uuid.Must(uuid.NewV4()).String()
			repoWrapper := &RepoWrapper{Repo: repo}

			subj := oidctrust.NewModule(repoWrapper)

			// when
			err := subj.Unblock(t.Context(), expTenantID)

			// then
			assert.NoError(t, err)
		})
	})
	t.Run("should return error", func(t *testing.T) {
		t.Run("if Get returns an error", func(t *testing.T) {
			// given
			expTenantID := uuid.Must(uuid.NewV4()).String()
			mockRepo := &RepoWrapper{Repo: repo}

			noOfGetTenantCalls := 0
			mockRepo.MockGet = func(ctx context.Context, tenantID string) (*trustv1.Trust, error) {
				assert.Equal(t, expTenantID, tenantID)
				noOfGetTenantCalls++
				return trustv1.Trust_builder{
					Oidc: oidcv1.OIDC_builder{}.Build(),
				}.Build(), assert.AnError
			}
			subj := oidctrust.NewModule(mockRepo)

			// when
			err := subj.Unblock(t.Context(), expTenantID)

			// then
			assert.ErrorIs(t, err, assert.AnError)
			assert.Equal(t, 1, noOfGetTenantCalls)
		})

		t.Run("if Update returns an error", func(t *testing.T) {
			// given
			expTenantIDtoUpdate := uuid.Must(uuid.NewV4()).String()
			expBlockedTrust := trustv1.Trust_builder{
				TenantId: new(expTenantIDtoUpdate),
				Blocked:  new(true),
				Oidc: oidcv1.OIDC_builder{
					Issuer:    new(uuid.Must(uuid.NewV4()).String()),
					JwksUri:   new(jwksURI),
					Audiences: []string{requestURI},
				}.Build(),
			}.Build()
			repoWrapper := &RepoWrapper{Repo: repo}
			err := repoWrapper.Repo.Create(ctx, expBlockedTrust)
			require.NoError(t, err)

			noOfUpdateCalls := 0
			repoWrapper.MockUpdate = func(ctx context.Context, trust *trustv1.Trust) error {
				assert.Equal(t, expTenantIDtoUpdate, trust.GetTenantId())
				noOfUpdateCalls++
				return assert.AnError
			}
			subj := oidctrust.NewModule(repoWrapper)

			// when
			err = subj.Unblock(t.Context(), expTenantIDtoUpdate)

			// then
			assert.ErrorIs(t, err, assert.AnError)
			assert.Equal(t, 1, noOfUpdateCalls)

			actTrust, err := repoWrapper.Repo.Get(ctx, expTenantIDtoUpdate)
			assert.NoError(t, err)
			assert.Equal(t, expBlockedTrust, actTrust)
		})
	})
}

func TestService_Remove(t *testing.T) {
	ctx := t.Context()

	t.Run("success if", func(t *testing.T) {
		t.Run("the trust exists", func(t *testing.T) {
			// given
			expTenantID := uuid.Must(uuid.NewV4()).String()
			expTrust := trustv1.Trust_builder{
				TenantId: new(expTenantID),
				Oidc: oidcv1.OIDC_builder{
					Issuer:    new(uuid.Must(uuid.NewV4()).String()),
					JwksUri:   new(jwksURI),
					Audiences: []string{requestURI},
				}.Build(),
			}.Build()

			wrapper := &RepoWrapper{Repo: repo}
			err := wrapper.Repo.Create(ctx, expTrust)
			require.NoError(t, err)

			subj := oidctrust.NewModule(wrapper)

			// when
			err = subj.Remove(ctx, expTenantID)

			// then
			assert.NoError(t, err)

			// verify the trust was deleted
			_, err = wrapper.Repo.Get(ctx, expTenantID)
			assert.Error(t, err)
		})
	})

	t.Run("should return error if", func(t *testing.T) {
		t.Run("the trust does not exist", func(t *testing.T) {
			// given
			expTenantID := uuid.Must(uuid.NewV4()).String()
			wrapper := &RepoWrapper{Repo: repo}
			subj := oidctrust.NewModule(wrapper)

			// when
			err := subj.Remove(ctx, expTenantID)

			// then
			assert.Error(t, err)
		})

		t.Run("Delete returns an error", func(t *testing.T) {
			// given
			expTenantID := uuid.Must(uuid.NewV4()).String()
			wrapper := &RepoWrapper{Repo: repo}

			noOfDeleteCalls := 0
			wrapper.MockDelete = func(ctx context.Context, tenantID string) error {
				assert.Equal(t, expTenantID, tenantID)
				noOfDeleteCalls++
				return assert.AnError
			}

			subj := oidctrust.NewModule(wrapper)

			// when
			err := subj.Remove(ctx, expTenantID)

			// then
			assert.ErrorIs(t, err, assert.AnError)
			assert.Equal(t, 1, noOfDeleteCalls)
		})
	})
}

func TestService_Get(t *testing.T) {
	ctx := t.Context()

	repoErr := errors.New("repository error")

	tests := []struct {
		name      string
		trust     *trustv1.Trust
		repoErr   error
		wantErr   bool
		wantErrIs error
	}{
		{
			name: "returns trust",
			trust: trustv1.Trust_builder{
				TenantId: new(uuid.Must(uuid.NewV4()).String()),
				Blocked:  new(false),
				Oidc: oidcv1.OIDC_builder{
					Issuer:    new(uuid.Must(uuid.NewV4()).String()),
					JwksUri:   new(jwksURI),
					Audiences: []string{requestURI},
				}.Build(),
			}.Build(),
		},
		{
			name:    "returns error when trust does not exist",
			wantErr: true,
		},
		{
			name:      "wraps repository error",
			repoErr:   repoErr,
			wantErr:   true,
			wantErrIs: repoErr,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var opts []mocktrust.RepositoryOption
			if tt.trust != nil {
				opts = append(opts, mocktrust.WithTrust(tt.trust))
			}
			if tt.repoErr != nil {
				opts = append(opts, mocktrust.WithGetError(tt.repoErr))
			}

			mockRepo := mocktrust.NewInMemRepository(opts...)
			subj := oidctrust.NewModule(mockRepo)

			var tenantID string
			if tt.trust != nil {
				tenantID = tt.trust.GetTenantId()
			} else {
				tenantID = uuid.Must(uuid.NewV4()).String()
			}

			got, err := subj.Get(ctx, tenantID)

			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				if tt.wantErrIs != nil && !errors.Is(err, tt.wantErrIs) {
					t.Fatalf("error = %v, want to wrap %v", err, tt.wantErrIs)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if diff := cmp.Diff(tt.trust, got, protocmp.Transform()); diff != "" {
				t.Fatalf("trust not equal:\n%s", diff)
			}
		})
	}
}
