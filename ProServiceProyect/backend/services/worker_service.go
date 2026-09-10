package services

import (
	"context"
	"errors"
	"log"
	"time"

	"backend/models"
	"backend/utils"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
)

const (
	minSpecialties  = 1
	maxSpecialties  = 3
	defaultPageSize = 20
	maxPageSize     = 100
)

// WorkerRepository define lo que WorkerService necesita de la colección workers.
type WorkerRepository interface {
	Create(ctx context.Context, worker *models.Worker) error
	FindByID(ctx context.Context, id bson.ObjectID) (*models.Worker, error)
	FindByUserID(ctx context.Context, userID bson.ObjectID) (*models.Worker, error)
	Update(ctx context.Context, worker *models.Worker) error
	SetActive(ctx context.Context, id bson.ObjectID, active bool) error
	Delete(ctx context.Context, id bson.ObjectID) error
	Search(ctx context.Context, specialtyID *bson.ObjectID, page, pageSize int) ([]models.Worker, error)
}

// SpecialtyChecker es lo que WorkerService necesita del catálogo: confirmar que
// los ids elegidos existen realmente (regla 5).
type SpecialtyChecker interface {
	AllExist(ctx context.Context, ids []bson.ObjectID) (bool, error)
}

// UserPromoter es lo que WorkerService necesita de la gestión de usuarios:
// pasar el rol a worker al crear un perfil.
type UserPromoter interface {
	PromoteToWorker(ctx context.Context, userID bson.ObjectID) error
}

// WorkerProfileInput son los campos editables de un perfil, ya con los
// specialty_ids parseados a ObjectID por el handler.
type WorkerProfileInput struct {
	Bio                  string
	SpecialtyIDs         []bson.ObjectID
	Phone                string
	ContactEmail         string
	SocialLinks          map[string]string
	AvailabilityStatus   models.AvailabilityStatus
	AvailabilitySchedule string
	Photos               []string
}

// WorkerService concentra la lógica del perfil profesional. No sabe de HTTP ni
// de Mongo.
type WorkerService struct {
	repo        WorkerRepository
	specialties SpecialtyChecker
	users       UserPromoter
}

// NewWorkerService recibe las interfaces que consume, nunca structs concretos.
func NewWorkerService(repo WorkerRepository, specialties SpecialtyChecker, users UserPromoter) *WorkerService {
	return &WorkerService{repo: repo, specialties: specialties, users: users}
}

// CreateProfile crea el perfil del usuario y lo promueve a worker. Mongo
// standalone no da transacciones, así que si la promoción del rol falla se
// borra el perfil recién creado: la consistencia se sostiene con una
// compensación, o queda todo (perfil + rol) o no queda nada.
func (s *WorkerService) CreateProfile(ctx context.Context, userID bson.ObjectID, input WorkerProfileInput) (*models.Worker, error) {
	if _, err := s.repo.FindByUserID(ctx, userID); err == nil {
		return nil, utils.ErrConflict
	} else if !errors.Is(err, mongo.ErrNoDocuments) {
		log.Printf("worker service: find by user id failed: %v", err)
		return nil, err
	}

	if err := s.validateProfile(ctx, input); err != nil {
		return nil, err
	}

	worker := &models.Worker{
		UserID:               userID,
		Bio:                  input.Bio,
		SpecialtyIDs:         input.SpecialtyIDs,
		Phone:                input.Phone,
		ContactEmail:         input.ContactEmail,
		SocialLinks:          input.SocialLinks,
		AvailabilityStatus:   input.AvailabilityStatus,
		AvailabilitySchedule: input.AvailabilitySchedule,
		Photos:               input.Photos,
		Active:               true,
		CreatedAt:            time.Now().UTC(),
	}

	if err := s.repo.Create(ctx, worker); err != nil {
		// El índice único en user_id puede saltar si dos requests entran juntos.
		if mongo.IsDuplicateKeyError(err) {
			return nil, utils.ErrConflict
		}
		log.Printf("worker service: create failed: %v", err)
		return nil, err
	}

	if err := s.users.PromoteToWorker(ctx, userID); err != nil {
		if delErr := s.repo.Delete(ctx, worker.ID); delErr != nil {
			log.Printf("worker service: CRITICAL compensating delete failed for worker %s: %v", worker.ID.Hex(), delErr)
		}
		return nil, err
	}

	return worker, nil
}

// GetByID devuelve un perfil por su id para la vista pública. Un perfil inactivo
// se trata como inexistente: dado de baja, no aparece.
func (s *WorkerService) GetByID(ctx context.Context, id bson.ObjectID) (*models.Worker, error) {
	worker, err := s.repo.FindByID(ctx, id)
	if err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return nil, utils.ErrNotFound
		}
		log.Printf("worker service: find by id failed: %v", err)
		return nil, err
	}
	if !worker.Active {
		return nil, utils.ErrNotFound
	}
	return worker, nil
}

// GetOwn devuelve el perfil del usuario autenticado, activo o no: lo consume el
// formulario de edición.
func (s *WorkerService) GetOwn(ctx context.Context, userID bson.ObjectID) (*models.Worker, error) {
	worker, err := s.repo.FindByUserID(ctx, userID)
	if err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return nil, utils.ErrNotFound
		}
		log.Printf("worker service: find by user id failed: %v", err)
		return nil, err
	}
	return worker, nil
}

// UpdateOwn reemplaza los campos editables del perfil del usuario autenticado.
// La propiedad es implícita: el perfil se ubica por el user_id del token.
func (s *WorkerService) UpdateOwn(ctx context.Context, userID bson.ObjectID, input WorkerProfileInput) (*models.Worker, error) {
	worker, err := s.GetOwn(ctx, userID)
	if err != nil {
		return nil, err
	}

	if err := s.validateProfile(ctx, input); err != nil {
		return nil, err
	}

	worker.Bio = input.Bio
	worker.SpecialtyIDs = input.SpecialtyIDs
	worker.Phone = input.Phone
	worker.ContactEmail = input.ContactEmail
	worker.SocialLinks = input.SocialLinks
	worker.AvailabilityStatus = input.AvailabilityStatus
	worker.AvailabilitySchedule = input.AvailabilitySchedule
	worker.Photos = input.Photos

	if err := s.repo.Update(ctx, worker); err != nil {
		log.Printf("worker service: update failed: %v", err)
		return nil, err
	}
	return worker, nil
}

// DeactivateOwn da de baja lógica el perfil del usuario autenticado.
func (s *WorkerService) DeactivateOwn(ctx context.Context, userID bson.ObjectID) error {
	worker, err := s.GetOwn(ctx, userID)
	if err != nil {
		return err
	}
	return s.deactivate(ctx, worker.ID)
}

// DeactivateByID da de baja lógica cualquier perfil. Lo usa la moderación del
// administrador; el chequeo de rol vive en el middleware.
func (s *WorkerService) DeactivateByID(ctx context.Context, id bson.ObjectID) error {
	if _, err := s.GetByID(ctx, id); err != nil {
		return err
	}
	return s.deactivate(ctx, id)
}

func (s *WorkerService) deactivate(ctx context.Context, id bson.ObjectID) error {
	if err := s.repo.SetActive(ctx, id, false); err != nil {
		log.Printf("worker service: deactivate failed: %v", err)
		return err
	}
	return nil
}

// Search devuelve la página de perfiles activos que matchean, ordenados por
// calificación. Aplica los defaults y el tope de tamaño de página acá para que
// el repositorio no reciba valores fuera de rango.
func (s *WorkerService) Search(ctx context.Context, specialtyID *bson.ObjectID, page, pageSize int) ([]models.Worker, error) {
	if page < 1 {
		page = 1
	}
	switch {
	case pageSize < 1:
		pageSize = defaultPageSize
	case pageSize > maxPageSize:
		pageSize = maxPageSize
	}

	workers, err := s.repo.Search(ctx, specialtyID, page, pageSize)
	if err != nil {
		log.Printf("worker service: search failed: %v", err)
		return nil, err
	}
	return workers, nil
}

// validateProfile chequea las reglas comunes a crear y actualizar: cantidad,
// unicidad y existencia real de las especialidades (regla 5) y estado de
// disponibilidad dentro del enum.
func (s *WorkerService) validateProfile(ctx context.Context, input WorkerProfileInput) error {
	count := len(input.SpecialtyIDs)
	if count < minSpecialties || count > maxSpecialties {
		return utils.ErrValidation
	}
	if hasDuplicateObjectIDs(input.SpecialtyIDs) {
		return utils.ErrValidation
	}
	if !input.AvailabilityStatus.Valid() {
		return utils.ErrValidation
	}

	ok, err := s.specialties.AllExist(ctx, input.SpecialtyIDs)
	if err != nil {
		log.Printf("worker service: specialty check failed: %v", err)
		return err
	}
	if !ok {
		return utils.ErrValidation
	}
	return nil
}

func hasDuplicateObjectIDs(ids []bson.ObjectID) bool {
	seen := make(map[bson.ObjectID]struct{}, len(ids))
	for _, id := range ids {
		if _, dup := seen[id]; dup {
			return true
		}
		seen[id] = struct{}{}
	}
	return false
}
