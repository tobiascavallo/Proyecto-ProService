package models

import (
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
)

// AvailabilityStatus enumera los estados de disponibilidad que el trabajador
// declara manualmente. El sistema no lo calcula ni lo cambia solo (regla 7).
type AvailabilityStatus string

const (
	AvailabilityAvailable   AvailabilityStatus = "available"
	AvailabilityBusy        AvailabilityStatus = "busy"
	AvailabilityUnavailable AvailabilityStatus = "unavailable"
)

// Valid indica si el estado es uno de los aceptados. Se usa en el service antes
// de guardar: un valor fuera del enum es ErrValidation.
func (s AvailabilityStatus) Valid() bool {
	switch s {
	case AvailabilityAvailable, AvailabilityBusy, AvailabilityUnavailable:
		return true
	default:
		return false
	}
}

// Worker es el perfil profesional de un usuario. average_rating y review_count
// están desnormalizados acá —los recalcula el service de reseñas en cada
// cambio— para que la búsqueda ordene por calificación con un find + sort sobre
// índice, sin agregación.
type Worker struct {
	ID                   bson.ObjectID      `bson:"_id,omitempty"`
	UserID               bson.ObjectID      `bson:"user_id"`
	Bio                  string             `bson:"bio"`
	SpecialtyIDs         []bson.ObjectID    `bson:"specialty_ids"`
	Phone                string             `bson:"phone"`
	ContactEmail         string             `bson:"contact_email"`
	SocialLinks          map[string]string  `bson:"social_links"`
	AvailabilityStatus   AvailabilityStatus `bson:"availability_status"`
	AvailabilitySchedule string             `bson:"availability_schedule"`
	Photos               []string           `bson:"photos"`
	AverageRating        float64            `bson:"average_rating"`
	ReviewCount          int                `bson:"review_count"`
	Active               bool               `bson:"active"`
	CreatedAt            time.Time          `bson:"created_at"`
}
