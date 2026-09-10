package services

import (
	"context"
	"errors"
	"log"
	"strings"
	"time"

	"backend/models"
	"backend/utils"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
)

// UserRepository define las operaciones sobre la colección users que necesita
// esta capa de servicio. La implementación concreta vive en
// repositories/user_repository.go; el service solo conoce esta interfaz, nunca
// el struct concreto, para poder testear con un mock.
type UserRepository interface {
	Create(ctx context.Context, user *models.User) error
	FindByID(ctx context.Context, id bson.ObjectID) (*models.User, error)
	FindByGoogleID(ctx context.Context, googleID string) (*models.User, error)
	Update(ctx context.Context, user *models.User) error
	SetRole(ctx context.Context, id bson.ObjectID, role models.Role) error
	Delete(ctx context.Context, id bson.ObjectID) error
}

// UserService concentra la lógica de negocio sobre usuarios. No sabe de HTTP
// ni de Mongo: solo conoce la interfaz UserRepository.
type UserService struct {
	repo UserRepository
}

// NewUserService crea el service a partir de la interfaz del repository, nunca
// de un struct concreto, para poder testear con un mock.
func NewUserService(repo UserRepository) *UserService {
	return &UserService{repo: repo}
}

// FindOrCreateByGoogle busca un usuario por su google_id y, si no existe, lo
// crea con rol client. Es el paso 4 del flujo de login: para cuando se llama
// acá, el ID token de Google ya fue validado por el caller.
func (s *UserService) FindOrCreateByGoogle(ctx context.Context, googleID, email, name, pictureURL string) (*models.User, error) {
	if strings.TrimSpace(googleID) == "" || strings.TrimSpace(email) == "" {
		return nil, utils.ErrValidation
	}

	existing, err := s.repo.FindByGoogleID(ctx, googleID)
	if err == nil {
		return existing, nil
	}
	if !errors.Is(err, mongo.ErrNoDocuments) {
		log.Printf("user service: find by google id failed: %v", err)
		return nil, err
	}

	user := &models.User{
		GoogleID:   googleID,
		Email:      email,
		Name:       name,
		PictureURL: pictureURL,
		Role:       models.RoleClient,
		CreatedAt:  time.Now().UTC(),
	}

	if err := s.repo.Create(ctx, user); err != nil {
		// Otro request pudo haber creado el mismo usuario entre el find y el
		// create: se trata como conflicto en vez de error genérico.
		if mongo.IsDuplicateKeyError(err) {
			return nil, utils.ErrConflict
		}
		log.Printf("user service: create failed: %v", err)
		return nil, err
	}

	return user, nil
}

// GetByID devuelve un usuario por su ID, traducido a ErrNotFound cuando no
// existe.
func (s *UserService) GetByID(ctx context.Context, id bson.ObjectID) (*models.User, error) {
	user, err := s.repo.FindByID(ctx, id)
	if err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return nil, utils.ErrNotFound
		}
		log.Printf("user service: find by id failed: %v", err)
		return nil, err
	}
	return user, nil
}

// UpdateProfile actualiza el nombre y la foto de perfil de un usuario
// existente. No permite tocar google_id, email ni role: esos campos cambian
// por otros caminos (login de Google, alta de perfil de trabajador).
func (s *UserService) UpdateProfile(ctx context.Context, id bson.ObjectID, name, pictureURL string) (*models.User, error) {
	if strings.TrimSpace(name) == "" {
		return nil, utils.ErrValidation
	}

	user, err := s.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}

	user.Name = name
	user.PictureURL = pictureURL

	if err := s.repo.Update(ctx, user); err != nil {
		log.Printf("user service: update failed: %v", err)
		return nil, err
	}

	return user, nil
}

// PromoteToWorker pone el rol del usuario en worker. Lo llama WorkerService al
// crear un perfil profesional: el rol vive en el user para que el próximo token
// emitido lo lleve. No valida que exista el perfil —de eso se encarga el caller.
func (s *UserService) PromoteToWorker(ctx context.Context, userID bson.ObjectID) error {
	if err := s.repo.SetRole(ctx, userID, models.RoleWorker); err != nil {
		log.Printf("user service: promote to worker failed: %v", err)
		return err
	}
	return nil
}

// Delete elimina un usuario existente. Devuelve ErrNotFound si ya no está,
// para que el handler responda 404 en vez de un 200 fantasma.
func (s *UserService) Delete(ctx context.Context, id bson.ObjectID) error {
	if _, err := s.GetByID(ctx, id); err != nil {
		return err
	}

	if err := s.repo.Delete(ctx, id); err != nil {
		log.Printf("user service: delete failed: %v", err)
		return err
	}

	return nil
}
