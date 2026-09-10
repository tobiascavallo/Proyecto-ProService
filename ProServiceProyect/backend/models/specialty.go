package models

import (
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
)

// Specialty es un ítem del catálogo cerrado de oficios. El catálogo se siembra
// al arrancar y no tiene ABM en la V1: no hay estado ni autor porque nadie lo
// crea desde la aplicación.
type Specialty struct {
	ID        bson.ObjectID `bson:"_id,omitempty"`
	Name      string        `bson:"name"`
	Slug      string        `bson:"slug"`
	CreatedAt time.Time     `bson:"created_at"`
}
