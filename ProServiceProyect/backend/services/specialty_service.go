package services

import (
	"context"
	"log"

	"backend/models"
)

// SpecialtyRepository define lo que el service necesita de la colección
// specialties. La implementación concreta vive en repositories/.
type SpecialtyRepository interface {
	List(ctx context.Context) ([]models.Specialty, error)
}

// SpecialtyService expone el catálogo de especialidades. Es fino a propósito:
// el catálogo es cerrado y se siembra al arrancar, no hay reglas de negocio ni
// ABM sobre él en la V1.
type SpecialtyService struct {
	repo SpecialtyRepository
}

// NewSpecialtyService crea el service a partir de la interfaz del repository.
func NewSpecialtyService(repo SpecialtyRepository) *SpecialtyService {
	return &SpecialtyService{repo: repo}
}

// List devuelve el catálogo completo. Loguea el error crudo del repository y lo
// propaga para que el handler responda 500 genérico sin filtrar detalles de
// Mongo.
func (s *SpecialtyService) List(ctx context.Context) ([]models.Specialty, error) {
	specialties, err := s.repo.List(ctx)
	if err != nil {
		log.Printf("specialty service: list failed: %v", err)
		return nil, err
	}
	return specialties, nil
}
