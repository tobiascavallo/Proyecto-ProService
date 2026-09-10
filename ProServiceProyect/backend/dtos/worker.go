package dtos

import (
	"time"

	"backend/models"

	"go.mongodb.org/mongo-driver/v2/bson"
)

// WorkerProfileRequest es el body de POST /workers y PUT /workers/me. El PUT es
// reemplazo total: el cliente manda el objeto completo de campos editables.
// user_id, average_rating, review_count y active no se tocan desde acá.
type WorkerProfileRequest struct {
	Bio                  string            `json:"bio"`
	SpecialtyIDs         []string          `json:"specialtyIds"`
	Phone                string            `json:"phone"`
	ContactEmail         string            `json:"contactEmail"`
	SocialLinks          map[string]string `json:"socialLinks"`
	AvailabilityStatus   string            `json:"availabilityStatus"`
	AvailabilitySchedule string            `json:"availabilitySchedule"`
	Photos               []string          `json:"photos"`
}

// WorkerResponse es la vista pública de un perfil. Incluye los datos de
// contacto: son públicos por la regla 1. Las especialidades van como IDs; el
// frontend las mapea contra el catálogo de /specialties que ya tiene.
type WorkerResponse struct {
	ID                   string            `json:"id"`
	UserID               string            `json:"userId"`
	Bio                  string            `json:"bio"`
	SpecialtyIDs         []string          `json:"specialtyIds"`
	Phone                string            `json:"phone"`
	ContactEmail         string            `json:"contactEmail"`
	SocialLinks          map[string]string `json:"socialLinks"`
	AvailabilityStatus   string            `json:"availabilityStatus"`
	AvailabilitySchedule string            `json:"availabilitySchedule"`
	Photos               []string          `json:"photos"`
	AverageRating        float64           `json:"averageRating"`
	ReviewCount          int               `json:"reviewCount"`
	Active               bool              `json:"active"`
	CreatedAt            string            `json:"createdAt"`
}

// NewWorkerResponse mapea un Worker de dominio a su DTO de salida. Normaliza
// nil a colección vacía para que el JSON sea [] / {} y no null.
func NewWorkerResponse(w models.Worker) WorkerResponse {
	return WorkerResponse{
		ID:                   objectIDToString(w.ID),
		UserID:               objectIDToString(w.UserID),
		Bio:                  w.Bio,
		SpecialtyIDs:         objectIDsToStrings(w.SpecialtyIDs),
		Phone:                w.Phone,
		ContactEmail:         w.ContactEmail,
		SocialLinks:          nonNilMap(w.SocialLinks),
		AvailabilityStatus:   string(w.AvailabilityStatus),
		AvailabilitySchedule: w.AvailabilitySchedule,
		Photos:               nonNilStrings(w.Photos),
		AverageRating:        w.AverageRating,
		ReviewCount:          w.ReviewCount,
		Active:               w.Active,
		CreatedAt:            timeToRFC3339(w.CreatedAt),
	}
}

// NewWorkerListResponse mapea una página de resultados de búsqueda.
func NewWorkerListResponse(workers []models.Worker) []WorkerResponse {
	out := make([]WorkerResponse, 0, len(workers))
	for _, w := range workers {
		out = append(out, NewWorkerResponse(w))
	}
	return out
}

func objectIDToString(id bson.ObjectID) string {
	if id.IsZero() {
		return ""
	}
	return id.Hex()
}

func objectIDsToStrings(ids []bson.ObjectID) []string {
	out := make([]string, 0, len(ids))
	for _, id := range ids {
		out = append(out, id.Hex())
	}
	return out
}

func nonNilStrings(s []string) []string {
	if s == nil {
		return []string{}
	}
	return s
}

func nonNilMap(m map[string]string) map[string]string {
	if m == nil {
		return map[string]string{}
	}
	return m
}

func timeToRFC3339(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.UTC().Format(time.RFC3339)
}
