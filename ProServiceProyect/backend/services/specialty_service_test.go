package services

import (
	"context"
	"errors"
	"testing"

	"backend/models"

	"go.mongodb.org/mongo-driver/v2/bson"
)

// fakeSpecialtyRepository es un mock a mano de SpecialtyRepository.
type fakeSpecialtyRepository struct {
	ListFunc func(ctx context.Context) ([]models.Specialty, error)
}

func (f *fakeSpecialtyRepository) List(ctx context.Context) ([]models.Specialty, error) {
	return f.ListFunc(ctx)
}

func TestSpecialtyService_List(t *testing.T) {
	catalog := []models.Specialty{
		{ID: bson.NewObjectID(), Name: "Plomero", Slug: "plomero"},
		{ID: bson.NewObjectID(), Name: "Electricista", Slug: "electricista"},
	}

	tests := []struct {
		name    string
		repo    *fakeSpecialtyRepository
		wantErr error
		wantLen int
	}{
		{
			name: "returns the full catalog",
			repo: &fakeSpecialtyRepository{
				ListFunc: func(ctx context.Context) ([]models.Specialty, error) {
					return catalog, nil
				},
			},
			wantLen: 2,
		},
		{
			name: "propagates repository error unchanged",
			repo: &fakeSpecialtyRepository{
				ListFunc: func(ctx context.Context) ([]models.Specialty, error) {
					return nil, errBoom
				},
			},
			wantErr: errBoom,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := NewSpecialtyService(tt.repo)

			got, err := svc.List(context.Background())

			if tt.wantErr != nil {
				if !errors.Is(err, tt.wantErr) {
					t.Fatalf("esperaba %v, recibí %v", tt.wantErr, err)
				}
				return
			}
			if err != nil {
				t.Fatalf("no esperaba error, recibí %v", err)
			}
			if len(got) != tt.wantLen {
				t.Fatalf("esperaba %d especialidades, recibí %d", tt.wantLen, len(got))
			}
		})
	}
}
