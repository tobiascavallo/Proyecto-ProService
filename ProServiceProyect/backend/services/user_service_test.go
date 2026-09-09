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

// fakeUserRepository es un mock de UserRepository escrito a mano: cada método
// delega en un campo función, así cada test setea solo el comportamiento que
// necesita. Si el service llama un método que el test no configuró, el campo
// queda nil y el panic señala justamente esa llamada inesperada.
type fakeUserRepository struct {
	CreateFunc         func(ctx context.Context, user *models.User) error
	FindByIDFunc       func(ctx context.Context, id bson.ObjectID) (*models.User, error)
	FindByGoogleIDFunc func(ctx context.Context, googleID string) (*models.User, error)
	UpdateFunc         func(ctx context.Context, user *models.User) error
	DeleteFunc         func(ctx context.Context, id bson.ObjectID) error
}

func (f *fakeUserRepository) Create(ctx context.Context, user *models.User) error {
	return f.CreateFunc(ctx, user)
}

func (f *fakeUserRepository) FindByID(ctx context.Context, id bson.ObjectID) (*models.User, error) {
	return f.FindByIDFunc(ctx, id)
}

func (f *fakeUserRepository) FindByGoogleID(ctx context.Context, googleID string) (*models.User, error) {
	return f.FindByGoogleIDFunc(ctx, googleID)
}

func (f *fakeUserRepository) Update(ctx context.Context, user *models.User) error {
	return f.UpdateFunc(ctx, user)
}

func (f *fakeUserRepository) Delete(ctx context.Context, id bson.ObjectID) error {
	return f.DeleteFunc(ctx, id)
}

// errBoom simula un error inesperado del repository (timeout, conexión caída,
// etc.): ni "no encontrado" ni clave duplicada, así que el service debe
// devolverlo sin traducirlo a ningún error de dominio.
var errBoom = errors.New("boom")

func TestUserService_FindOrCreateByGoogle(t *testing.T) {
	existingUser := &models.User{
		ID:       bson.NewObjectID(),
		GoogleID: "google-1",
		Email:    "existing@example.com",
		Role:     models.RoleWorker,
	}

	tests := []struct {
		name      string
		googleID  string
		email     string
		repo      *fakeUserRepository
		wantErr   error
		checkUser func(t *testing.T, user *models.User)
	}{
		{
			name:     "empty google id is a validation error",
			googleID: "",
			email:    "client@example.com",
			repo:     &fakeUserRepository{},
			wantErr:  utils.ErrValidation,
		},
		{
			name:     "blank email is a validation error",
			googleID: "google-1",
			email:    "   ",
			repo:     &fakeUserRepository{},
			wantErr:  utils.ErrValidation,
		},
		{
			name:     "existing user is returned without creating",
			googleID: "google-1",
			email:    "existing@example.com",
			repo: &fakeUserRepository{
				FindByGoogleIDFunc: func(ctx context.Context, googleID string) (*models.User, error) {
					return existingUser, nil
				},
				CreateFunc: func(ctx context.Context, user *models.User) error {
					t.Fatal("Create no debería llamarse si el usuario ya existe")
					return nil
				},
			},
			checkUser: func(t *testing.T, user *models.User) {
				if user != existingUser {
					t.Fatalf("esperaba el usuario existente, recibió %+v", user)
				}
			},
		},
		{
			name:     "new google id creates a client",
			googleID: "google-2",
			email:    "new@example.com",
			repo: &fakeUserRepository{
				FindByGoogleIDFunc: func(ctx context.Context, googleID string) (*models.User, error) {
					return nil, mongo.ErrNoDocuments
				},
				CreateFunc: func(ctx context.Context, user *models.User) error {
					user.ID = bson.NewObjectID()
					return nil
				},
			},
			checkUser: func(t *testing.T, user *models.User) {
				if user.Role != models.RoleClient {
					t.Errorf("esperaba role client, recibió %q", user.Role)
				}
				if user.GoogleID != "google-2" || user.Email != "new@example.com" {
					t.Errorf("datos del usuario creado no coinciden: %+v", user)
				}
				if user.ID.IsZero() {
					t.Error("esperaba que Create completara el ID")
				}
				if user.CreatedAt.IsZero() {
					t.Error("esperaba CreatedAt seteado")
				}
			},
		},
		{
			name:     "unexpected error on find propagates unchanged",
			googleID: "google-3",
			email:    "err@example.com",
			repo: &fakeUserRepository{
				FindByGoogleIDFunc: func(ctx context.Context, googleID string) (*models.User, error) {
					return nil, errBoom
				},
			},
			wantErr: errBoom,
		},
		{
			name:     "duplicate key on create returns conflict",
			googleID: "google-4",
			email:    "conflict@example.com",
			repo: &fakeUserRepository{
				FindByGoogleIDFunc: func(ctx context.Context, googleID string) (*models.User, error) {
					return nil, mongo.ErrNoDocuments
				},
				CreateFunc: func(ctx context.Context, user *models.User) error {
					return mongo.CommandError{Code: 11000, Message: "duplicate key"}
				},
			},
			wantErr: utils.ErrConflict,
		},
		{
			name:     "unexpected error on create propagates unchanged",
			googleID: "google-5",
			email:    "err2@example.com",
			repo: &fakeUserRepository{
				FindByGoogleIDFunc: func(ctx context.Context, googleID string) (*models.User, error) {
					return nil, mongo.ErrNoDocuments
				},
				CreateFunc: func(ctx context.Context, user *models.User) error {
					return errBoom
				},
			},
			wantErr: errBoom,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := NewUserService(tt.repo)

			user, err := svc.FindOrCreateByGoogle(context.Background(), tt.googleID, tt.email, "Nombre", "http://pic")

			if tt.wantErr != nil {
				if !errors.Is(err, tt.wantErr) {
					t.Fatalf("esperaba error %v, recibió %v", tt.wantErr, err)
				}
				return
			}
			if err != nil {
				t.Fatalf("no esperaba error, recibió %v", err)
			}
			if tt.checkUser != nil {
				tt.checkUser(t, user)
			}
		})
	}
}

func TestUserService_GetByID(t *testing.T) {
	userID := bson.NewObjectID()
	foundUser := &models.User{ID: userID, Name: "Ana"}

	tests := []struct {
		name    string
		repo    *fakeUserRepository
		wantErr error
	}{
		{
			name: "user found",
			repo: &fakeUserRepository{
				FindByIDFunc: func(ctx context.Context, id bson.ObjectID) (*models.User, error) {
					if id != userID {
						t.Fatalf("esperaba id %s, recibió %s", userID.Hex(), id.Hex())
					}
					return foundUser, nil
				},
			},
		},
		{
			name: "not found translates to ErrNotFound",
			repo: &fakeUserRepository{
				FindByIDFunc: func(ctx context.Context, id bson.ObjectID) (*models.User, error) {
					return nil, mongo.ErrNoDocuments
				},
			},
			wantErr: utils.ErrNotFound,
		},
		{
			name: "unexpected error propagates unchanged",
			repo: &fakeUserRepository{
				FindByIDFunc: func(ctx context.Context, id bson.ObjectID) (*models.User, error) {
					return nil, errBoom
				},
			},
			wantErr: errBoom,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := NewUserService(tt.repo)

			user, err := svc.GetByID(context.Background(), userID)

			if tt.wantErr != nil {
				if !errors.Is(err, tt.wantErr) {
					t.Fatalf("esperaba error %v, recibió %v", tt.wantErr, err)
				}
				return
			}
			if err != nil {
				t.Fatalf("no esperaba error, recibió %v", err)
			}
			if user != foundUser {
				t.Fatalf("esperaba %+v, recibió %+v", foundUser, user)
			}
		})
	}
}

func TestUserService_UpdateProfile(t *testing.T) {
	userID := bson.NewObjectID()

	tests := []struct {
		name      string
		newName   string
		repo      *fakeUserRepository
		wantErr   error
		checkUser func(t *testing.T, user *models.User)
	}{
		{
			name:    "blank name is a validation error",
			newName: "   ",
			repo:    &fakeUserRepository{},
			wantErr: utils.ErrValidation,
		},
		{
			name:    "user not found translates to ErrNotFound",
			newName: "Nuevo Nombre",
			repo: &fakeUserRepository{
				FindByIDFunc: func(ctx context.Context, id bson.ObjectID) (*models.User, error) {
					return nil, mongo.ErrNoDocuments
				},
			},
			wantErr: utils.ErrNotFound,
		},
		{
			name:    "unexpected error on get propagates unchanged",
			newName: "Nuevo Nombre",
			repo: &fakeUserRepository{
				FindByIDFunc: func(ctx context.Context, id bson.ObjectID) (*models.User, error) {
					return nil, errBoom
				},
			},
			wantErr: errBoom,
		},
		{
			name:    "update succeeds",
			newName: "Nuevo Nombre",
			repo: &fakeUserRepository{
				FindByIDFunc: func(ctx context.Context, id bson.ObjectID) (*models.User, error) {
					return &models.User{ID: id, Name: "Viejo", PictureURL: "old.jpg"}, nil
				},
				UpdateFunc: func(ctx context.Context, user *models.User) error {
					return nil
				},
			},
			checkUser: func(t *testing.T, user *models.User) {
				if user.Name != "Nuevo Nombre" {
					t.Errorf("esperaba nombre actualizado, recibió %q", user.Name)
				}
				if user.PictureURL != "new.jpg" {
					t.Errorf("esperaba picture actualizada, recibió %q", user.PictureURL)
				}
			},
		},
		{
			name:    "unexpected error on update propagates unchanged",
			newName: "Nuevo Nombre",
			repo: &fakeUserRepository{
				FindByIDFunc: func(ctx context.Context, id bson.ObjectID) (*models.User, error) {
					return &models.User{ID: id}, nil
				},
				UpdateFunc: func(ctx context.Context, user *models.User) error {
					return errBoom
				},
			},
			wantErr: errBoom,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := NewUserService(tt.repo)

			user, err := svc.UpdateProfile(context.Background(), userID, tt.newName, "new.jpg")

			if tt.wantErr != nil {
				if !errors.Is(err, tt.wantErr) {
					t.Fatalf("esperaba error %v, recibió %v", tt.wantErr, err)
				}
				return
			}
			if err != nil {
				t.Fatalf("no esperaba error, recibió %v", err)
			}
			if tt.checkUser != nil {
				tt.checkUser(t, user)
			}
		})
	}
}

func TestUserService_Delete(t *testing.T) {
	userID := bson.NewObjectID()

	tests := []struct {
		name    string
		repo    *fakeUserRepository
		wantErr error
	}{
		{
			name: "user not found translates to ErrNotFound",
			repo: &fakeUserRepository{
				FindByIDFunc: func(ctx context.Context, id bson.ObjectID) (*models.User, error) {
					return nil, mongo.ErrNoDocuments
				},
			},
			wantErr: utils.ErrNotFound,
		},
		{
			name: "unexpected error on get propagates unchanged",
			repo: &fakeUserRepository{
				FindByIDFunc: func(ctx context.Context, id bson.ObjectID) (*models.User, error) {
					return nil, errBoom
				},
			},
			wantErr: errBoom,
		},
		{
			name: "delete succeeds",
			repo: &fakeUserRepository{
				FindByIDFunc: func(ctx context.Context, id bson.ObjectID) (*models.User, error) {
					return &models.User{ID: id}, nil
				},
				DeleteFunc: func(ctx context.Context, id bson.ObjectID) error {
					return nil
				},
			},
		},
		{
			name: "unexpected error on delete propagates unchanged",
			repo: &fakeUserRepository{
				FindByIDFunc: func(ctx context.Context, id bson.ObjectID) (*models.User, error) {
					return &models.User{ID: id}, nil
				},
				DeleteFunc: func(ctx context.Context, id bson.ObjectID) error {
					return errBoom
				},
			},
			wantErr: errBoom,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := NewUserService(tt.repo)

			err := svc.Delete(context.Background(), userID)

			if tt.wantErr != nil {
				if !errors.Is(err, tt.wantErr) {
					t.Fatalf("esperaba error %v, recibió %v", tt.wantErr, err)
				}
				return
			}
			if err != nil {
				t.Fatalf("no esperaba error, recibió %v", err)
			}
		})
	}
}
