package models

import (
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
)

// Role enumera los roles posibles de un usuario. El rol se lee siempre del JWT
// en el backend y nunca viaja desde el frontend.
type Role string

const (
	RoleClient Role = "client"
	RoleWorker Role = "worker"
	RoleAdmin  Role = "admin"
)

// User es la identidad de una persona en el sistema. No guarda contraseña ni
// hash: toda la autenticación está delegada a Google y el vínculo estable es el
// google_id. Un campo de password acá sería un error de diseño.
type User struct {
	ID         bson.ObjectID `bson:"_id,omitempty"`
	GoogleID   string        `bson:"google_id"`
	Email      string        `bson:"email"`
	Name       string        `bson:"name"`
	PictureURL string        `bson:"picture_url"`
	Role       Role          `bson:"role"`
	CreatedAt  time.Time     `bson:"created_at"`
}
