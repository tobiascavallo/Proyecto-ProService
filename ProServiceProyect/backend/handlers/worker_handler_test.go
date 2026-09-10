package handlers

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"backend/models"
	"backend/services"
	"backend/utils"

	"github.com/gin-gonic/gin"
	"go.mongodb.org/mongo-driver/v2/bson"
)

// fakeWorkerService es un mock a mano de la interfaz WorkerService.
type fakeWorkerService struct {
	CreateProfileFunc  func(ctx context.Context, userID bson.ObjectID, input services.WorkerProfileInput) (*models.Worker, error)
	GetByIDFunc        func(ctx context.Context, id bson.ObjectID) (*models.Worker, error)
	GetOwnFunc         func(ctx context.Context, userID bson.ObjectID) (*models.Worker, error)
	UpdateOwnFunc      func(ctx context.Context, userID bson.ObjectID, input services.WorkerProfileInput) (*models.Worker, error)
	DeactivateOwnFunc  func(ctx context.Context, userID bson.ObjectID) error
	DeactivateByIDFunc func(ctx context.Context, id bson.ObjectID) error
	SearchFunc         func(ctx context.Context, specialtyID *bson.ObjectID, page, pageSize int) ([]models.Worker, error)
}

func (f *fakeWorkerService) CreateProfile(ctx context.Context, userID bson.ObjectID, input services.WorkerProfileInput) (*models.Worker, error) {
	return f.CreateProfileFunc(ctx, userID, input)
}
func (f *fakeWorkerService) GetByID(ctx context.Context, id bson.ObjectID) (*models.Worker, error) {
	return f.GetByIDFunc(ctx, id)
}
func (f *fakeWorkerService) GetOwn(ctx context.Context, userID bson.ObjectID) (*models.Worker, error) {
	return f.GetOwnFunc(ctx, userID)
}
func (f *fakeWorkerService) UpdateOwn(ctx context.Context, userID bson.ObjectID, input services.WorkerProfileInput) (*models.Worker, error) {
	return f.UpdateOwnFunc(ctx, userID, input)
}
func (f *fakeWorkerService) DeactivateOwn(ctx context.Context, userID bson.ObjectID) error {
	return f.DeactivateOwnFunc(ctx, userID)
}
func (f *fakeWorkerService) DeactivateByID(ctx context.Context, id bson.ObjectID) error {
	return f.DeactivateByIDFunc(ctx, id)
}
func (f *fakeWorkerService) Search(ctx context.Context, specialtyID *bson.ObjectID, page, pageSize int) ([]models.Worker, error) {
	return f.SearchFunc(ctx, specialtyID, page, pageSize)
}

// newWorkerRouter monta las rutas de worker. Si userIDCtx no está vacío, un
// middleware falso lo deja en el contexto simulando RequireAuth.
func newWorkerRouter(svc *fakeWorkerService, userIDCtx string) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()

	group := r.Group("/api/v1")
	if userIDCtx != "" {
		group.Use(func(c *gin.Context) {
			c.Set("userID", userIDCtx)
			c.Next()
		})
	}

	h := NewWorkerHandler(svc)
	h.RegisterPublicRoutes(group)
	h.RegisterProtectedRoutes(group)
	h.RegisterAdminRoutes(group)

	return r
}

func TestWorkerHandler_ErrorMapping(t *testing.T) {
	activeWorker := &models.Worker{ID: bson.NewObjectID(), UserID: bson.NewObjectID(), Active: true, AvailabilityStatus: models.AvailabilityAvailable}
	body := `{"bio":"x","specialtyIds":["` + bson.NewObjectID().Hex() + `"],"availabilityStatus":"available"}`
	userID := bson.NewObjectID().Hex()

	tests := []struct {
		name       string
		method     string
		path       string
		body       string
		userIDCtx  string
		svc        *fakeWorkerService
		wantStatus int
		wantCode   string
	}{
		{
			name:   "search ok",
			method: http.MethodGet, path: "/api/v1/workers",
			svc: &fakeWorkerService{SearchFunc: func(ctx context.Context, s *bson.ObjectID, p, ps int) ([]models.Worker, error) {
				return []models.Worker{*activeWorker}, nil
			}},
			wantStatus: http.StatusOK,
		},
		{
			name:   "search with malformed specialty id is a validation error",
			method: http.MethodGet, path: "/api/v1/workers?specialtyId=nope",
			svc:        &fakeWorkerService{},
			wantStatus: http.StatusBadRequest, wantCode: "VALIDATION",
		},
		{
			name:   "search repo failure is a generic 500",
			method: http.MethodGet, path: "/api/v1/workers",
			svc: &fakeWorkerService{SearchFunc: func(ctx context.Context, s *bson.ObjectID, p, ps int) ([]models.Worker, error) {
				return nil, errUnexpected
			}},
			wantStatus: http.StatusInternalServerError, wantCode: "INTERNAL",
		},
		{
			name:   "get by malformed id is not found",
			method: http.MethodGet, path: "/api/v1/workers/nope",
			svc:        &fakeWorkerService{},
			wantStatus: http.StatusNotFound, wantCode: "NOT_FOUND",
		},
		{
			name:   "get missing worker is not found",
			method: http.MethodGet, path: "/api/v1/workers/" + bson.NewObjectID().Hex(),
			svc: &fakeWorkerService{GetByIDFunc: func(ctx context.Context, id bson.ObjectID) (*models.Worker, error) {
				return nil, utils.ErrNotFound
			}},
			wantStatus: http.StatusNotFound, wantCode: "NOT_FOUND",
		},
		{
			name:   "create without auth context is unauthorized",
			method: http.MethodPost, path: "/api/v1/workers", body: body,
			svc:        &fakeWorkerService{},
			wantStatus: http.StatusUnauthorized, wantCode: "UNAUTHORIZED",
		},
		{
			name:   "create with malformed json is a validation error",
			method: http.MethodPost, path: "/api/v1/workers", body: `{`, userIDCtx: userID,
			svc:        &fakeWorkerService{},
			wantStatus: http.StatusBadRequest, wantCode: "VALIDATION",
		},
		{
			name:   "create when profile exists is a conflict",
			method: http.MethodPost, path: "/api/v1/workers", body: body, userIDCtx: userID,
			svc: &fakeWorkerService{CreateProfileFunc: func(ctx context.Context, uid bson.ObjectID, in services.WorkerProfileInput) (*models.Worker, error) {
				return nil, utils.ErrConflict
			}},
			wantStatus: http.StatusConflict, wantCode: "CONFLICT",
		},
		{
			name:   "create with invalid specialties is a validation error",
			method: http.MethodPost, path: "/api/v1/workers", body: body, userIDCtx: userID,
			svc: &fakeWorkerService{CreateProfileFunc: func(ctx context.Context, uid bson.ObjectID, in services.WorkerProfileInput) (*models.Worker, error) {
				return nil, utils.ErrValidation
			}},
			wantStatus: http.StatusBadRequest, wantCode: "VALIDATION",
		},
		{
			name:   "create ok returns 201",
			method: http.MethodPost, path: "/api/v1/workers", body: body, userIDCtx: userID,
			svc: &fakeWorkerService{CreateProfileFunc: func(ctx context.Context, uid bson.ObjectID, in services.WorkerProfileInput) (*models.Worker, error) {
				return activeWorker, nil
			}},
			wantStatus: http.StatusCreated,
		},
		{
			name:   "update ok returns 200",
			method: http.MethodPut, path: "/api/v1/workers/me", body: body, userIDCtx: userID,
			svc: &fakeWorkerService{UpdateOwnFunc: func(ctx context.Context, uid bson.ObjectID, in services.WorkerProfileInput) (*models.Worker, error) {
				return activeWorker, nil
			}},
			wantStatus: http.StatusOK,
		},
		{
			name:   "deactivate own ok returns 204",
			method: http.MethodDelete, path: "/api/v1/workers/me", userIDCtx: userID,
			svc: &fakeWorkerService{DeactivateOwnFunc: func(ctx context.Context, uid bson.ObjectID) error {
				return nil
			}},
			wantStatus: http.StatusNoContent,
		},
		{
			name:   "deactivate own without profile is not found",
			method: http.MethodDelete, path: "/api/v1/workers/me", userIDCtx: userID,
			svc: &fakeWorkerService{DeactivateOwnFunc: func(ctx context.Context, uid bson.ObjectID) error {
				return utils.ErrNotFound
			}},
			wantStatus: http.StatusNotFound, wantCode: "NOT_FOUND",
		},
		{
			name:   "admin deactivate missing worker is not found",
			method: http.MethodDelete, path: "/api/v1/workers/" + bson.NewObjectID().Hex(), userIDCtx: userID,
			svc: &fakeWorkerService{DeactivateByIDFunc: func(ctx context.Context, id bson.ObjectID) error {
				return utils.ErrNotFound
			}},
			wantStatus: http.StatusNotFound, wantCode: "NOT_FOUND",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rec := httptest.NewRecorder()
			var reqBody *strings.Reader
			if tt.body != "" {
				reqBody = strings.NewReader(tt.body)
			} else {
				reqBody = strings.NewReader("")
			}
			req := httptest.NewRequest(tt.method, tt.path, reqBody)
			req.Header.Set("Content-Type", "application/json")

			newWorkerRouter(tt.svc, tt.userIDCtx).ServeHTTP(rec, req)

			if rec.Code != tt.wantStatus {
				t.Fatalf("esperaba status %d, recibí %d (%s)", tt.wantStatus, rec.Code, rec.Body)
			}
			if tt.wantCode != "" && decodeErrorCode(t, rec.Body.Bytes()) != tt.wantCode {
				t.Fatalf("esperaba code %q, recibí %s", tt.wantCode, rec.Body)
			}
		})
	}
}
