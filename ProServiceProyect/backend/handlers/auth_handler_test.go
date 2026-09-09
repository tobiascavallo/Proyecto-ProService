package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"backend/dtos"
	"backend/models"
	"backend/services"
	"backend/utils"

	"github.com/gin-gonic/gin"
	"go.mongodb.org/mongo-driver/v2/bson"
)

// fakeAuthService es un mock a mano de la interfaz AuthService que consume el
// handler.
type fakeAuthService struct {
	LoginWithGoogleFunc func(ctx context.Context, idToken string) (*models.User, services.TokenPair, error)
	RefreshFunc         func(ctx context.Context, refreshToken string) (*models.User, services.TokenPair, error)
}

func (f *fakeAuthService) LoginWithGoogle(ctx context.Context, idToken string) (*models.User, services.TokenPair, error) {
	return f.LoginWithGoogleFunc(ctx, idToken)
}

func (f *fakeAuthService) Refresh(ctx context.Context, refreshToken string) (*models.User, services.TokenPair, error) {
	return f.RefreshFunc(ctx, refreshToken)
}

func newAuthRouter(svc *fakeAuthService) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	NewAuthHandler(svc).RegisterRoutes(r.Group("/api/v1"))
	return r
}

func TestAuthHandler_LoginWithGoogle(t *testing.T) {
	user := &models.User{ID: bson.NewObjectID(), Email: "a@b.com", Name: "Ana", Role: models.RoleClient}

	tests := []struct {
		name       string
		body       string
		svc        *fakeAuthService
		wantStatus int
		wantCode   string
	}{
		{
			name: "valid login returns user and tokens",
			body: `{"idToken":"google-token"}`,
			svc: &fakeAuthService{
				LoginWithGoogleFunc: func(ctx context.Context, idToken string) (*models.User, services.TokenPair, error) {
					if idToken != "google-token" {
						t.Fatalf("id token mal propagado: %q", idToken)
					}
					return user, services.TokenPair{AccessToken: "access-1", RefreshToken: "refresh-1"}, nil
				},
			},
			wantStatus: http.StatusOK,
		},
		{
			name:       "malformed json is a validation error",
			body:       `{"idToken":`,
			svc:        &fakeAuthService{},
			wantStatus: http.StatusBadRequest,
			wantCode:   "VALIDATION",
		},
		{
			name: "google rejection maps to 401",
			body: `{"idToken":"bad"}`,
			svc: &fakeAuthService{
				LoginWithGoogleFunc: func(ctx context.Context, idToken string) (*models.User, services.TokenPair, error) {
					return nil, services.TokenPair{}, utils.ErrUnauthorized
				},
			},
			wantStatus: http.StatusUnauthorized,
			wantCode:   "UNAUTHORIZED",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rec := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/google", strings.NewReader(tt.body))
			req.Header.Set("Content-Type", "application/json")
			newAuthRouter(tt.svc).ServeHTTP(rec, req)

			if rec.Code != tt.wantStatus {
				t.Fatalf("esperaba status %d, recibí %d (%s)", tt.wantStatus, rec.Code, rec.Body)
			}
			if tt.wantCode != "" && decodeErrorCode(t, rec.Body.Bytes()) != tt.wantCode {
				t.Fatalf("esperaba code %q, recibí %s", tt.wantCode, rec.Body)
			}
			if tt.wantStatus == http.StatusOK {
				var resp dtos.AuthResponse
				if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
					t.Fatalf("respuesta no parseable: %v", err)
				}
				if resp.AccessToken != "access-1" || resp.RefreshToken != "refresh-1" {
					t.Fatalf("tokens inesperados: %+v", resp)
				}
				if resp.User.ID != user.ID.Hex() || resp.User.Email != "a@b.com" {
					t.Fatalf("user inesperado: %+v", resp.User)
				}
			}
		})
	}
}

func TestAuthHandler_Refresh(t *testing.T) {
	user := &models.User{ID: bson.NewObjectID(), Role: models.RoleWorker}

	tests := []struct {
		name       string
		body       string
		svc        *fakeAuthService
		wantStatus int
		wantCode   string
	}{
		{
			name: "valid refresh returns a new pair",
			body: `{"refreshToken":"refresh-old"}`,
			svc: &fakeAuthService{
				RefreshFunc: func(ctx context.Context, refreshToken string) (*models.User, services.TokenPair, error) {
					return user, services.TokenPair{AccessToken: "access-2", RefreshToken: "refresh-2"}, nil
				},
			},
			wantStatus: http.StatusOK,
		},
		{
			name: "invalid refresh token maps to 401",
			body: `{"refreshToken":"nope"}`,
			svc: &fakeAuthService{
				RefreshFunc: func(ctx context.Context, refreshToken string) (*models.User, services.TokenPair, error) {
					return nil, services.TokenPair{}, utils.ErrUnauthorized
				},
			},
			wantStatus: http.StatusUnauthorized,
			wantCode:   "UNAUTHORIZED",
		},
		{
			name:       "malformed json is a validation error",
			body:       `{`,
			svc:        &fakeAuthService{},
			wantStatus: http.StatusBadRequest,
			wantCode:   "VALIDATION",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rec := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/refresh", strings.NewReader(tt.body))
			req.Header.Set("Content-Type", "application/json")
			newAuthRouter(tt.svc).ServeHTTP(rec, req)

			if rec.Code != tt.wantStatus {
				t.Fatalf("esperaba status %d, recibí %d (%s)", tt.wantStatus, rec.Code, rec.Body)
			}
			if tt.wantCode != "" && decodeErrorCode(t, rec.Body.Bytes()) != tt.wantCode {
				t.Fatalf("esperaba code %q, recibí %s", tt.wantCode, rec.Body)
			}
			if tt.wantStatus == http.StatusOK {
				var resp dtos.AuthResponse
				if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
					t.Fatalf("respuesta no parseable: %v", err)
				}
				if resp.AccessToken != "access-2" || resp.RefreshToken != "refresh-2" {
					t.Fatalf("tokens inesperados: %+v", resp)
				}
			}
		})
	}
}
