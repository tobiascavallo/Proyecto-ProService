package services

import (
	"context"
	"errors"
	"testing"

	"backend/models"
	"backend/utils"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
)

// --- mocks a mano de las tres interfaces que consume WorkerService ---

type fakeWorkerRepository struct {
	CreateFunc       func(ctx context.Context, worker *models.Worker) error
	FindByIDFunc     func(ctx context.Context, id bson.ObjectID) (*models.Worker, error)
	FindByUserIDFunc func(ctx context.Context, userID bson.ObjectID) (*models.Worker, error)
	UpdateFunc       func(ctx context.Context, worker *models.Worker) error
	SetActiveFunc    func(ctx context.Context, id bson.ObjectID, active bool) error
	DeleteFunc       func(ctx context.Context, id bson.ObjectID) error
	SearchFunc       func(ctx context.Context, specialtyID *bson.ObjectID, page, pageSize int) ([]models.Worker, error)
}

func (f *fakeWorkerRepository) Create(ctx context.Context, worker *models.Worker) error {
	return f.CreateFunc(ctx, worker)
}
func (f *fakeWorkerRepository) FindByID(ctx context.Context, id bson.ObjectID) (*models.Worker, error) {
	return f.FindByIDFunc(ctx, id)
}
func (f *fakeWorkerRepository) FindByUserID(ctx context.Context, userID bson.ObjectID) (*models.Worker, error) {
	return f.FindByUserIDFunc(ctx, userID)
}
func (f *fakeWorkerRepository) Update(ctx context.Context, worker *models.Worker) error {
	return f.UpdateFunc(ctx, worker)
}
func (f *fakeWorkerRepository) SetActive(ctx context.Context, id bson.ObjectID, active bool) error {
	return f.SetActiveFunc(ctx, id, active)
}
func (f *fakeWorkerRepository) Delete(ctx context.Context, id bson.ObjectID) error {
	return f.DeleteFunc(ctx, id)
}
func (f *fakeWorkerRepository) Search(ctx context.Context, specialtyID *bson.ObjectID, page, pageSize int) ([]models.Worker, error) {
	return f.SearchFunc(ctx, specialtyID, page, pageSize)
}

type fakeSpecialtyChecker struct {
	AllExistFunc func(ctx context.Context, ids []bson.ObjectID) (bool, error)
}

func (f *fakeSpecialtyChecker) AllExist(ctx context.Context, ids []bson.ObjectID) (bool, error) {
	return f.AllExistFunc(ctx, ids)
}

type fakeUserPromoter struct {
	PromoteToWorkerFunc func(ctx context.Context, userID bson.ObjectID) error
}

func (f *fakeUserPromoter) PromoteToWorker(ctx context.Context, userID bson.ObjectID) error {
	return f.PromoteToWorkerFunc(ctx, userID)
}

// --- helpers ---

// specialtiesOK es un checker que dice que todo id existe.
func specialtiesOK() *fakeSpecialtyChecker {
	return &fakeSpecialtyChecker{AllExistFunc: func(ctx context.Context, ids []bson.ObjectID) (bool, error) {
		return true, nil
	}}
}

// promoterOK es un promoter que siempre funciona.
func promoterOK() *fakeUserPromoter {
	return &fakeUserPromoter{PromoteToWorkerFunc: func(ctx context.Context, userID bson.ObjectID) error {
		return nil
	}}
}

func validWorkerInput() WorkerProfileInput {
	return WorkerProfileInput{
		Bio:                "hago plomería",
		SpecialtyIDs:       []bson.ObjectID{bson.NewObjectID(), bson.NewObjectID()},
		Phone:              "1122334455",
		ContactEmail:       "w@example.com",
		AvailabilityStatus: models.AvailabilityAvailable,
	}
}

func TestWorkerService_CreateProfile(t *testing.T) {
	userID := bson.NewObjectID()

	tests := []struct {
		name     string
		input    WorkerProfileInput
		repo     *fakeWorkerRepository
		checker  *fakeSpecialtyChecker
		promoter *fakeUserPromoter
		wantErr  error
	}{
		{
			name:  "valid profile is created and the user is promoted",
			input: validWorkerInput(),
			repo: &fakeWorkerRepository{
				FindByUserIDFunc: notFoundWorker,
				CreateFunc: func(ctx context.Context, worker *models.Worker) error {
					worker.ID = bson.NewObjectID()
					return nil
				},
			},
			checker:  specialtiesOK(),
			promoter: promoterOK(),
		},
		{
			name:  "user already has a profile",
			input: validWorkerInput(),
			repo: &fakeWorkerRepository{
				FindByUserIDFunc: func(ctx context.Context, id bson.ObjectID) (*models.Worker, error) {
					return &models.Worker{ID: bson.NewObjectID()}, nil
				},
			},
			checker:  specialtiesOK(),
			promoter: promoterOK(),
			wantErr:  utils.ErrConflict,
		},
		{
			name:     "no specialties",
			input:    withSpecialties(),
			repo:     &fakeWorkerRepository{FindByUserIDFunc: notFoundWorker},
			checker:  specialtiesOK(),
			promoter: promoterOK(),
			wantErr:  utils.ErrValidation,
		},
		{
			name:     "more than three specialties",
			input:    withSpecialties(bson.NewObjectID(), bson.NewObjectID(), bson.NewObjectID(), bson.NewObjectID()),
			repo:     &fakeWorkerRepository{FindByUserIDFunc: notFoundWorker},
			checker:  specialtiesOK(),
			promoter: promoterOK(),
			wantErr:  utils.ErrValidation,
		},
		{
			name: "duplicate specialties",
			input: func() WorkerProfileInput {
				id := bson.NewObjectID()
				return withSpecialties(id, id)
			}(),
			repo:     &fakeWorkerRepository{FindByUserIDFunc: notFoundWorker},
			checker:  specialtiesOK(),
			promoter: promoterOK(),
			wantErr:  utils.ErrValidation,
		},
		{
			name:  "a specialty does not exist",
			input: validWorkerInput(),
			repo:  &fakeWorkerRepository{FindByUserIDFunc: notFoundWorker},
			checker: &fakeSpecialtyChecker{AllExistFunc: func(ctx context.Context, ids []bson.ObjectID) (bool, error) {
				return false, nil
			}},
			promoter: promoterOK(),
			wantErr:  utils.ErrValidation,
		},
		{
			name: "invalid availability status",
			input: func() WorkerProfileInput {
				in := validWorkerInput()
				in.AvailabilityStatus = "sometimes"
				return in
			}(),
			repo:     &fakeWorkerRepository{FindByUserIDFunc: notFoundWorker},
			checker:  specialtiesOK(),
			promoter: promoterOK(),
			wantErr:  utils.ErrValidation,
		},
		{
			name:  "specialty check fails unexpectedly",
			input: validWorkerInput(),
			repo:  &fakeWorkerRepository{FindByUserIDFunc: notFoundWorker},
			checker: &fakeSpecialtyChecker{AllExistFunc: func(ctx context.Context, ids []bson.ObjectID) (bool, error) {
				return false, errBoom
			}},
			promoter: promoterOK(),
			wantErr:  errBoom,
		},
		{
			name:  "create hits the unique index",
			input: validWorkerInput(),
			repo: &fakeWorkerRepository{
				FindByUserIDFunc: notFoundWorker,
				CreateFunc: func(ctx context.Context, worker *models.Worker) error {
					return mongo.CommandError{Code: 11000, Message: "duplicate key"}
				},
			},
			checker:  specialtiesOK(),
			promoter: promoterOK(),
			wantErr:  utils.ErrConflict,
		},
		{
			name:  "promotion failure rolls the profile back",
			input: validWorkerInput(),
			repo: &fakeWorkerRepository{
				FindByUserIDFunc: notFoundWorker,
				CreateFunc: func(ctx context.Context, worker *models.Worker) error {
					worker.ID = bson.NewObjectID()
					return nil
				},
				DeleteFunc: func(ctx context.Context, id bson.ObjectID) error { return nil },
			},
			checker: specialtiesOK(),
			promoter: &fakeUserPromoter{PromoteToWorkerFunc: func(ctx context.Context, userID bson.ObjectID) error {
				return errBoom
			}},
			wantErr: errBoom,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var deleted bool
			if tt.repo.DeleteFunc == nil {
				tt.repo.DeleteFunc = func(ctx context.Context, id bson.ObjectID) error { return nil }
			}
			originalDelete := tt.repo.DeleteFunc
			tt.repo.DeleteFunc = func(ctx context.Context, id bson.ObjectID) error {
				deleted = true
				return originalDelete(ctx, id)
			}

			svc := NewWorkerService(tt.repo, tt.checker, tt.promoter)
			worker, err := svc.CreateProfile(context.Background(), userID, tt.input)

			if tt.wantErr != nil {
				if !errors.Is(err, tt.wantErr) {
					t.Fatalf("esperaba %v, recibí %v", tt.wantErr, err)
				}
				if tt.name == "promotion failure rolls the profile back" && !deleted {
					t.Fatal("esperaba que se compensara borrando el perfil")
				}
				return
			}
			if err != nil {
				t.Fatalf("no esperaba error, recibí %v", err)
			}
			if worker.UserID != userID || !worker.Active || worker.AverageRating != 0 || worker.ReviewCount != 0 {
				t.Fatalf("worker mal inicializado: %+v", worker)
			}
		})
	}
}

func TestWorkerService_GetByID(t *testing.T) {
	id := bson.NewObjectID()

	tests := []struct {
		name    string
		repo    *fakeWorkerRepository
		wantErr error
	}{
		{
			name: "active worker is returned",
			repo: &fakeWorkerRepository{FindByIDFunc: func(ctx context.Context, got bson.ObjectID) (*models.Worker, error) {
				return &models.Worker{ID: got, Active: true}, nil
			}},
		},
		{
			name: "inactive worker is treated as not found",
			repo: &fakeWorkerRepository{FindByIDFunc: func(ctx context.Context, got bson.ObjectID) (*models.Worker, error) {
				return &models.Worker{ID: got, Active: false}, nil
			}},
			wantErr: utils.ErrNotFound,
		},
		{
			name: "missing worker is not found",
			repo: &fakeWorkerRepository{FindByIDFunc: func(ctx context.Context, got bson.ObjectID) (*models.Worker, error) {
				return nil, mongo.ErrNoDocuments
			}},
			wantErr: utils.ErrNotFound,
		},
		{
			name: "unexpected error propagates",
			repo: &fakeWorkerRepository{FindByIDFunc: func(ctx context.Context, got bson.ObjectID) (*models.Worker, error) {
				return nil, errBoom
			}},
			wantErr: errBoom,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := NewWorkerService(tt.repo, specialtiesOK(), promoterOK())
			_, err := svc.GetByID(context.Background(), id)

			if tt.wantErr != nil {
				if !errors.Is(err, tt.wantErr) {
					t.Fatalf("esperaba %v, recibí %v", tt.wantErr, err)
				}
				return
			}
			if err != nil {
				t.Fatalf("no esperaba error, recibí %v", err)
			}
		})
	}
}

func TestWorkerService_UpdateOwn(t *testing.T) {
	userID := bson.NewObjectID()

	tests := []struct {
		name    string
		input   WorkerProfileInput
		repo    *fakeWorkerRepository
		checker *fakeSpecialtyChecker
		wantErr error
	}{
		{
			name:    "no profile to update",
			input:   validWorkerInput(),
			repo:    &fakeWorkerRepository{FindByUserIDFunc: notFoundWorker},
			checker: specialtiesOK(),
			wantErr: utils.ErrNotFound,
		},
		{
			name:  "invalid specialties are rejected",
			input: withSpecialties(),
			repo: &fakeWorkerRepository{FindByUserIDFunc: func(ctx context.Context, id bson.ObjectID) (*models.Worker, error) {
				return &models.Worker{ID: bson.NewObjectID(), UserID: id, Active: true}, nil
			}},
			checker: specialtiesOK(),
			wantErr: utils.ErrValidation,
		},
		{
			name:  "valid update is persisted",
			input: validWorkerInput(),
			repo: &fakeWorkerRepository{
				FindByUserIDFunc: func(ctx context.Context, id bson.ObjectID) (*models.Worker, error) {
					return &models.Worker{ID: bson.NewObjectID(), UserID: id, Bio: "viejo", Active: true}, nil
				},
				UpdateFunc: func(ctx context.Context, worker *models.Worker) error { return nil },
			},
			checker: specialtiesOK(),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := NewWorkerService(tt.repo, tt.checker, promoterOK())
			worker, err := svc.UpdateOwn(context.Background(), userID, tt.input)

			if tt.wantErr != nil {
				if !errors.Is(err, tt.wantErr) {
					t.Fatalf("esperaba %v, recibí %v", tt.wantErr, err)
				}
				return
			}
			if err != nil {
				t.Fatalf("no esperaba error, recibí %v", err)
			}
			if worker.Bio != tt.input.Bio {
				t.Fatalf("esperaba bio actualizada, recibí %q", worker.Bio)
			}
		})
	}
}

func TestWorkerService_Deactivate(t *testing.T) {
	userID := bson.NewObjectID()
	workerID := bson.NewObjectID()

	t.Run("own: missing profile is not found", func(t *testing.T) {
		repo := &fakeWorkerRepository{FindByUserIDFunc: notFoundWorker}
		err := NewWorkerService(repo, specialtiesOK(), promoterOK()).DeactivateOwn(context.Background(), userID)
		if !errors.Is(err, utils.ErrNotFound) {
			t.Fatalf("esperaba ErrNotFound, recibí %v", err)
		}
	})

	t.Run("own: sets active to false", func(t *testing.T) {
		var gotActive bool = true
		repo := &fakeWorkerRepository{
			FindByUserIDFunc: func(ctx context.Context, id bson.ObjectID) (*models.Worker, error) {
				return &models.Worker{ID: workerID, UserID: id, Active: true}, nil
			},
			SetActiveFunc: func(ctx context.Context, id bson.ObjectID, active bool) error {
				gotActive = active
				return nil
			},
		}
		if err := NewWorkerService(repo, specialtiesOK(), promoterOK()).DeactivateOwn(context.Background(), userID); err != nil {
			t.Fatalf("no esperaba error, recibí %v", err)
		}
		if gotActive {
			t.Fatal("esperaba SetActive(false)")
		}
	})

	t.Run("admin: sets active to false by id", func(t *testing.T) {
		called := false
		repo := &fakeWorkerRepository{
			FindByIDFunc: func(ctx context.Context, id bson.ObjectID) (*models.Worker, error) {
				return &models.Worker{ID: id, Active: true}, nil
			},
			SetActiveFunc: func(ctx context.Context, id bson.ObjectID, active bool) error {
				called = true
				return nil
			},
		}
		if err := NewWorkerService(repo, specialtiesOK(), promoterOK()).DeactivateByID(context.Background(), workerID); err != nil {
			t.Fatalf("no esperaba error, recibí %v", err)
		}
		if !called {
			t.Fatal("esperaba SetActive")
		}
	})
}

func TestWorkerService_Search(t *testing.T) {
	t.Run("clamps page and page size before hitting the repo", func(t *testing.T) {
		var gotPage, gotSize int
		repo := &fakeWorkerRepository{SearchFunc: func(ctx context.Context, specialtyID *bson.ObjectID, page, pageSize int) ([]models.Worker, error) {
			gotPage, gotSize = page, pageSize
			return nil, nil
		}}

		if _, err := NewWorkerService(repo, specialtiesOK(), promoterOK()).Search(context.Background(), nil, 0, 5000); err != nil {
			t.Fatalf("no esperaba error, recibí %v", err)
		}
		if gotPage != 1 || gotSize != maxPageSize {
			t.Fatalf("esperaba page 1 y size %d, recibí page %d size %d", maxPageSize, gotPage, gotSize)
		}
	})

	t.Run("propagates repo error", func(t *testing.T) {
		repo := &fakeWorkerRepository{SearchFunc: func(ctx context.Context, specialtyID *bson.ObjectID, page, pageSize int) ([]models.Worker, error) {
			return nil, errBoom
		}}
		_, err := NewWorkerService(repo, specialtiesOK(), promoterOK()).Search(context.Background(), nil, 1, 20)
		if !errors.Is(err, errBoom) {
			t.Fatalf("esperaba errBoom, recibí %v", err)
		}
	})
}

// notFoundWorker es el FindByUserID/FindByID que simula "no hay perfil".
func notFoundWorker(ctx context.Context, id bson.ObjectID) (*models.Worker, error) {
	return nil, mongo.ErrNoDocuments
}

// withSpecialties arma un input válido salvo por la lista de especialidades,
// que se reemplaza por los ids dados (puede ser vacía).
func withSpecialties(ids ...bson.ObjectID) WorkerProfileInput {
	in := validWorkerInput()
	in.SpecialtyIDs = ids
	return in
}
