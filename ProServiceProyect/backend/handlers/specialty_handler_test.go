package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"backend/dtos"
	"backend/models"

	"github.com/gin-gonic/gin"
	"go.mongodb.org/mongo-driver/v2/bson"
)

// fakeSpecialtyService es un mock a mano de la interfaz SpecialtyService.
type fakeSpecialtyService struct {
	ListFunc func(ctx context.Context) ([]models.Specialty, error)
}

func (f *fakeSpecialtyService) List(ctx context.Context) ([]models.Specialty, error) {
	return f.ListFunc(ctx)
}

func newSpecialtyRouter(svc *fakeSpecialtyService) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	NewSpecialtyHandler(svc).RegisterRoutes(r.Group("/api/v1"))
	return r
}

func TestSpecialtyHandler_List(t *testing.T) {
	tests := []struct {
		name       string
		svc        *fakeSpecialtyService
		wantStatus int
		wantCode   string
	}{
		{
			name: "returns the catalog",
			svc: &fakeSpecialtyService{
				ListFunc: func(ctx context.Context) ([]models.Specialty, error) {
					return []models.Specialty{{ID: bson.NewObjectID(), Name: "Plomero", Slug: "plomero"}}, nil
				},
			},
			wantStatus: http.StatusOK,
		},
		{
			name: "unexpected error maps to 500 generic",
			svc: &fakeSpecialtyService{
				ListFunc: func(ctx context.Context) ([]models.Specialty, error) {
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
			req := httptest.NewRequest(http.MethodGet, "/api/v1/specialties", nil)
			newSpecialtyRouter(tt.svc).ServeHTTP(rec, req)

			if rec.Code != tt.wantStatus {
				t.Fatalf("esperaba status %d, recibí %d (%s)", tt.wantStatus, rec.Code, rec.Body)
			}
			if tt.wantCode != "" && decodeErrorCode(t, rec.Body.Bytes()) != tt.wantCode {
				t.Fatalf("esperaba code %q, recibí %s", tt.wantCode, rec.Body)
			}
			if tt.wantStatus == http.StatusOK {
				var resp []dtos.SpecialtyResponse
				if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
					t.Fatalf("respuesta no parseable: %v", err)
				}
				if len(resp) != 1 || resp[0].Slug != "plomero" {
					t.Fatalf("catálogo inesperado: %+v", resp)
				}
			}
		})
	}
}
