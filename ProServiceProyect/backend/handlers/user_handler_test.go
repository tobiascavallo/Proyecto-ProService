package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"backend/dtos"
	"backend/models"
	"backend/utils"

	"github.com/gin-gonic/gin"
	"go.mongodb.org/mongo-driver/v2/bson"
)

// fakeUserService es un mock a mano de la interfaz UserService que consume el
// handler: cada método delega en un campo función.
type fakeUserService struct {
	GetByIDFunc       func(ctx context.Context, id bson.ObjectID) (*models.User, error)
	UpdateProfileFunc func(ctx context.Context, id bson.ObjectID, name, pictureURL string) (*models.User, error)
	DeleteFunc        func(ctx context.Context, id bson.ObjectID) error
}

func (f *fakeUserService) GetByID(ctx context.Context, id bson.ObjectID) (*models.User, error) {
	return f.GetByIDFunc(ctx, id)
}

func (f *fakeUserService) UpdateProfile(ctx context.Context, id bson.ObjectID, name, pictureURL string) (*models.User, error) {
	return f.UpdateProfileFunc(ctx, id, name, pictureURL)
}

func (f *fakeUserService) Delete(ctx context.Context, id bson.ObjectID) error {
	return f.DeleteFunc(ctx, id)
}

var errUnexpected = errors.New("boom")

// newRouter arma un router con el handler montado. Si userIDCtx no está vacío,
// un middleware falso lo deja en el contexto simulando lo que hace RequireAuth.
func newRouter(svc *fakeUserService, userIDCtx string) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()

	group := r.Group("/api/v1")
	if userIDCtx != "" {
		group.Use(func(c *gin.Context) {
			c.Set("userID", userIDCtx)
			c.Next()
		})
	}
	NewUserHandler(svc).RegisterRoutes(group)

	return r
}

func decodeErrorCode(t *testing.T, body []byte) string {
	t.Helper()
	var resp dtos.ErrorResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		t.Fatalf("respuesta de error no parseable: %v (%s)", err, body)
	}
	return resp.Error.Code
}

func TestUserHandler_GetMe(t *testing.T) {
	validID := bson.NewObjectID()
	existing := &models.User{ID: validID, Email: "a@b.com", Name: "Ana", Role: models.RoleClient}

	tests := []struct {
		name       string
		userIDCtx  string
		svc        *fakeUserService
		wantStatus int
		wantCode   string
	}{
		{
			name:      "returns the authenticated user",
			userIDCtx: validID.Hex(),
			svc: &fakeUserService{
				GetByIDFunc: func(ctx context.Context, id bson.ObjectID) (*models.User, error) {
					if id != validID {
						t.Fatalf("esperaba id %s, recibí %s", validID.Hex(), id.Hex())
					}
					return existing, nil
				},
			},
			wantStatus: http.StatusOK,
		},
		{
			name:       "missing auth context is unauthorized",
			userIDCtx:  "",
			svc:        &fakeUserService{},
			wantStatus: http.StatusUnauthorized,
			wantCode:   "UNAUTHORIZED",
		},
		{
			name:       "malformed user id in context is unauthorized",
			userIDCtx:  "not-a-hex-id",
			svc:        &fakeUserService{},
			wantStatus: http.StatusUnauthorized,
			wantCode:   "UNAUTHORIZED",
		},
		{
			name:      "not found maps to 404",
			userIDCtx: validID.Hex(),
			svc: &fakeUserService{
				GetByIDFunc: func(ctx context.Context, id bson.ObjectID) (*models.User, error) {
					return nil, utils.ErrNotFound
				},
			},
			wantStatus: http.StatusNotFound,
			wantCode:   "NOT_FOUND",
		},
		{
			name:      "unexpected error maps to 500 generic",
			userIDCtx: validID.Hex(),
			svc: &fakeUserService{
				GetByIDFunc: func(ctx context.Context, id bson.ObjectID) (*models.User, error) {
					return nil, errUnexpected
				},
			},
			wantStatus: http.StatusInternalServerError,
			wantCode:   "INTERNAL",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rec := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodGet, "/api/v1/users/me", nil)
			newRouter(tt.svc, tt.userIDCtx).ServeHTTP(rec, req)

			if rec.Code != tt.wantStatus {
				t.Fatalf("esperaba status %d, recibí %d (%s)", tt.wantStatus, rec.Code, rec.Body)
			}
			if tt.wantCode != "" && decodeErrorCode(t, rec.Body.Bytes()) != tt.wantCode {
				t.Fatalf("esperaba code %q, recibí %s", tt.wantCode, rec.Body)
			}
			if tt.wantStatus == http.StatusOK {
				var resp dtos.UserResponse
				if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
					t.Fatalf("respuesta no parseable: %v", err)
				}
				if resp.ID != validID.Hex() || resp.Email != "a@b.com" {
					t.Fatalf("DTO de salida inesperado: %+v", resp)
				}
			}
		})
	}
}

func TestUserHandler_UpdateMe(t *testing.T) {
	validID := bson.NewObjectID()

	tests := []struct {
		name       string
		userIDCtx  string
		body       string
		svc        *fakeUserService
		wantStatus int
		wantCode   string
	}{
		{
			name:      "updates name and picture",
			userIDCtx: validID.Hex(),
			body:      `{"name":"Nuevo","pictureUrl":"http://pic/new.jpg"}`,
			svc: &fakeUserService{
				UpdateProfileFunc: func(ctx context.Context, id bson.ObjectID, name, pictureURL string) (*models.User, error) {
					return &models.User{ID: id, Name: name, PictureURL: pictureURL, Role: models.RoleClient}, nil
				},
			},
			wantStatus: http.StatusOK,
		},
		{
			name:       "malformed json is a validation error",
			userIDCtx:  validID.Hex(),
			body:       `{"name":`,
			svc:        &fakeUserService{},
			wantStatus: http.StatusBadRequest,
			wantCode:   "VALIDATION",
		},
		{
			name:      "empty name from service is a validation error",
			userIDCtx: validID.Hex(),
			body:      `{"name":"  "}`,
			svc: &fakeUserService{
				UpdateProfileFunc: func(ctx context.Context, id bson.ObjectID, name, pictureURL string) (*models.User, error) {
					return nil, utils.ErrValidation
				},
			},
			wantStatus: http.StatusBadRequest,
			wantCode:   "VALIDATION",
		},
		{
			name:       "missing auth context is unauthorized",
			userIDCtx:  "",
			body:       `{"name":"Nuevo"}`,
			svc:        &fakeUserService{},
			wantStatus: http.StatusUnauthorized,
			wantCode:   "UNAUTHORIZED",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rec := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodPatch, "/api/v1/users/me", strings.NewReader(tt.body))
			req.Header.Set("Content-Type", "application/json")
			newRouter(tt.svc, tt.userIDCtx).ServeHTTP(rec, req)

			if rec.Code != tt.wantStatus {
				t.Fatalf("esperaba status %d, recibí %d (%s)", tt.wantStatus, rec.Code, rec.Body)
			}
			if tt.wantCode != "" && decodeErrorCode(t, rec.Body.Bytes()) != tt.wantCode {
				t.Fatalf("esperaba code %q, recibí %s", tt.wantCode, rec.Body)
			}
		})
	}
}

func TestUserHandler_DeleteMe(t *testing.T) {
	validID := bson.NewObjectID()

	tests := []struct {
		name       string
		userIDCtx  string
		svc        *fakeUserService
		wantStatus int
		wantCode   string
	}{
		{
			name:      "deletes the account",
			userIDCtx: validID.Hex(),
			svc: &fakeUserService{
				DeleteFunc: func(ctx context.Context, id bson.ObjectID) error {
					return nil
				},
			},
			wantStatus: http.StatusNoContent,
		},
		{
			name:      "not found maps to 404",
			userIDCtx: validID.Hex(),
			svc: &fakeUserService{
				DeleteFunc: func(ctx context.Context, id bson.ObjectID) error {
					return utils.ErrNotFound
				},
			},
			wantStatus: http.StatusNotFound,
			wantCode:   "NOT_FOUND",
		},
		{
			name:       "missing auth context is unauthorized",
			userIDCtx:  "",
			svc:        &fakeUserService{},
			wantStatus: http.StatusUnauthorized,
			wantCode:   "UNAUTHORIZED",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rec := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodDelete, "/api/v1/users/me", nil)
			newRouter(tt.svc, tt.userIDCtx).ServeHTTP(rec, req)

			if rec.Code != tt.wantStatus {
				t.Fatalf("esperaba status %d, recibí %d (%s)", tt.wantStatus, rec.Code, rec.Body)
			}
			if tt.wantStatus == http.StatusNoContent && rec.Body.Len() != 0 {
				t.Fatalf("esperaba body vacío, recibí %s", rec.Body)
			}
			if tt.wantCode != "" && decodeErrorCode(t, rec.Body.Bytes()) != tt.wantCode {
				t.Fatalf("esperaba code %q, recibí %s", tt.wantCode, rec.Body)
			}
		})
	}
}
