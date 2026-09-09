package services

import (
	"context"
	"errors"
	"testing"
	"time"

	"backend/models"
	"backend/utils"

	"go.mongodb.org/mongo-driver/v2/bson"
)

const (
	testJWTSecret  = "auth-service-test-secret"
	testAccessTTL  = 15 * time.Minute
	testRefreshTTL = 168 * time.Hour
)

// fakeGoogleVerifier es un mock a mano de GoogleVerifier.
type fakeGoogleVerifier struct {
	VerifyFunc func(ctx context.Context, idToken string) (utils.GoogleIdentity, error)
}

func (f *fakeGoogleVerifier) Verify(ctx context.Context, idToken string) (utils.GoogleIdentity, error) {
	return f.VerifyFunc(ctx, idToken)
}

// fakeUserProvider es un mock a mano de UserProvider.
type fakeUserProvider struct {
	FindOrCreateByGoogleFunc func(ctx context.Context, googleID, email, name, pictureURL string) (*models.User, error)
	GetByIDFunc              func(ctx context.Context, id bson.ObjectID) (*models.User, error)
}

func (f *fakeUserProvider) FindOrCreateByGoogle(ctx context.Context, googleID, email, name, pictureURL string) (*models.User, error) {
	return f.FindOrCreateByGoogleFunc(ctx, googleID, email, name, pictureURL)
}

func (f *fakeUserProvider) GetByID(ctx context.Context, id bson.ObjectID) (*models.User, error) {
	return f.GetByIDFunc(ctx, id)
}

func newAuthService(g GoogleVerifier, u UserProvider) *AuthService {
	return NewAuthService(g, u, testJWTSecret, testAccessTTL, testRefreshTTL)
}

// assertTokenPair valida que el par emitido sea usable y apunte al usuario y
// rol esperados, con el Type correcto en cada token.
func assertTokenPair(t *testing.T, pair TokenPair, wantUserID, wantRole string) {
	t.Helper()

	access, err := utils.ValidateToken(pair.AccessToken, testJWTSecret)
	if err != nil {
		t.Fatalf("access token inválido: %v", err)
	}
	if access.UserID != wantUserID || access.Role != wantRole || access.Type != utils.TokenTypeAccess {
		t.Fatalf("access claims inesperados: %+v", access)
	}

	refresh, err := utils.ValidateToken(pair.RefreshToken, testJWTSecret)
	if err != nil {
		t.Fatalf("refresh token inválido: %v", err)
	}
	if refresh.UserID != wantUserID || refresh.Role != wantRole || refresh.Type != utils.TokenTypeRefresh {
		t.Fatalf("refresh claims inesperados: %+v", refresh)
	}
}

func TestAuthService_LoginWithGoogle(t *testing.T) {
	user := &models.User{ID: bson.NewObjectID(), Email: "a@b.com", Role: models.RoleClient}

	tests := []struct {
		name     string
		idToken  string
		verifier *fakeGoogleVerifier
		users    *fakeUserProvider
		wantErr  error
		wantUser *models.User
	}{
		{
			name:     "blank id token is a validation error",
			idToken:  "   ",
			verifier: &fakeGoogleVerifier{},
			users:    &fakeUserProvider{},
			wantErr:  utils.ErrValidation,
		},
		{
			name:    "google rejection propagates as unauthorized",
			idToken: "bad-token",
			verifier: &fakeGoogleVerifier{
				VerifyFunc: func(ctx context.Context, idToken string) (utils.GoogleIdentity, error) {
					return utils.GoogleIdentity{}, utils.ErrUnauthorized
				},
			},
			users:   &fakeUserProvider{},
			wantErr: utils.ErrUnauthorized,
		},
		{
			name:    "find or create failure propagates",
			idToken: "good-token",
			verifier: &fakeGoogleVerifier{
				VerifyFunc: func(ctx context.Context, idToken string) (utils.GoogleIdentity, error) {
					return utils.GoogleIdentity{GoogleID: "g1", Email: "a@b.com"}, nil
				},
			},
			users: &fakeUserProvider{
				FindOrCreateByGoogleFunc: func(ctx context.Context, googleID, email, name, pictureURL string) (*models.User, error) {
					return nil, utils.ErrConflict
				},
			},
			wantErr: utils.ErrConflict,
		},
		{
			name:    "valid login returns user and token pair",
			idToken: "good-token",
			verifier: &fakeGoogleVerifier{
				VerifyFunc: func(ctx context.Context, idToken string) (utils.GoogleIdentity, error) {
					return utils.GoogleIdentity{GoogleID: "g1", Email: "a@b.com", Name: "Ana"}, nil
				},
			},
			users: &fakeUserProvider{
				FindOrCreateByGoogleFunc: func(ctx context.Context, googleID, email, name, pictureURL string) (*models.User, error) {
					if googleID != "g1" || email != "a@b.com" || name != "Ana" {
						t.Fatalf("identidad mal propagada: %s %s %s", googleID, email, name)
					}
					return user, nil
				},
			},
			wantUser: user,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := newAuthService(tt.verifier, tt.users)

			got, pair, err := svc.LoginWithGoogle(context.Background(), tt.idToken)

			if tt.wantErr != nil {
				if !errors.Is(err, tt.wantErr) {
					t.Fatalf("esperaba %v, recibí %v", tt.wantErr, err)
				}
				return
			}
			if err != nil {
				t.Fatalf("no esperaba error, recibí %v", err)
			}
			if got != tt.wantUser {
				t.Fatalf("usuario inesperado: %+v", got)
			}
			assertTokenPair(t, pair, user.ID.Hex(), string(models.RoleClient))
		})
	}
}

func TestAuthService_Refresh(t *testing.T) {
	user := &models.User{ID: bson.NewObjectID(), Role: models.RoleWorker}

	validRefresh, err := utils.GenerateRefreshToken(user.ID.Hex(), string(models.RoleClient), testJWTSecret, testRefreshTTL)
	if err != nil {
		t.Fatalf("no pude generar el refresh de prueba: %v", err)
	}

	accessAsRefresh, err := utils.GenerateAccessToken(user.ID.Hex(), string(models.RoleClient), testJWTSecret, testAccessTTL)
	if err != nil {
		t.Fatalf("no pude generar el access de prueba: %v", err)
	}

	refreshOfMissingUser, err := utils.GenerateRefreshToken(bson.NewObjectID().Hex(), string(models.RoleClient), testJWTSecret, testRefreshTTL)
	if err != nil {
		t.Fatalf("no pude generar el refresh de prueba: %v", err)
	}

	tests := []struct {
		name    string
		token   string
		users   *fakeUserProvider
		wantErr error
	}{
		{
			name:    "blank token is a validation error",
			token:   "",
			users:   &fakeUserProvider{},
			wantErr: utils.ErrValidation,
		},
		{
			name:    "garbage token is unauthorized",
			token:   "not-a-jwt",
			users:   &fakeUserProvider{},
			wantErr: utils.ErrUnauthorized,
		},
		{
			name:    "access token is not accepted for refresh",
			token:   accessAsRefresh,
			users:   &fakeUserProvider{},
			wantErr: utils.ErrUnauthorized,
		},
		{
			name:  "refresh of a deleted user propagates not found",
			token: refreshOfMissingUser,
			users: &fakeUserProvider{
				GetByIDFunc: func(ctx context.Context, id bson.ObjectID) (*models.User, error) {
					return nil, utils.ErrNotFound
				},
			},
			wantErr: utils.ErrNotFound,
		},
		{
			name:  "valid refresh issues a new pair with the current role",
			token: validRefresh,
			users: &fakeUserProvider{
				GetByIDFunc: func(ctx context.Context, id bson.ObjectID) (*models.User, error) {
					if id != user.ID {
						t.Fatalf("esperaba id %s, recibí %s", user.ID.Hex(), id.Hex())
					}
					return user, nil
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := newAuthService(&fakeGoogleVerifier{}, tt.users)

			got, pair, err := svc.Refresh(context.Background(), tt.token)

			if tt.wantErr != nil {
				if !errors.Is(err, tt.wantErr) {
					t.Fatalf("esperaba %v, recibí %v", tt.wantErr, err)
				}
				return
			}
			if err != nil {
				t.Fatalf("no esperaba error, recibí %v", err)
			}
			if got != user {
				t.Fatalf("usuario inesperado: %+v", got)
			}
			// El token viejo decía client; el usuario ahora es worker: el par
			// nuevo tiene que reflejar el rol actual.
			assertTokenPair(t, pair, user.ID.Hex(), string(models.RoleWorker))
		})
	}
}
