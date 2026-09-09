package dtos

import (
	"time"

	"backend/models"
)

// UpdateUserRequest es el body de PATCH /users/me. Solo el nombre y la foto de
// perfil son editables por el usuario; google_id, email y role cambian por
// otros caminos. La validación (nombre no vacío) vive en el service.
type UpdateUserRequest struct {
	Name       string `json:"name"`
	PictureURL string `json:"pictureUrl"`
}

// UserResponse es la vista pública de un usuario que devuelve la API. El modelo
// nunca se expone directo: así un campo interno como google_id no se filtra sin
// querer y el contrato de la API queda independiente del esquema de la base.
type UserResponse struct {
	ID         string `json:"id"`
	Email      string `json:"email"`
	Name       string `json:"name"`
	PictureURL string `json:"pictureUrl"`
	Role       string `json:"role"`
	CreatedAt  string `json:"createdAt"`
}

// NewUserResponse mapea un User de dominio a su DTO de salida. Tolera documentos
// incompletos: un ObjectID o un CreatedAt en cero se devuelven como string
// vacío en vez de romper.
func NewUserResponse(u models.User) UserResponse {
	id := ""
	if !u.ID.IsZero() {
		id = u.ID.Hex()
	}

	createdAt := ""
	if !u.CreatedAt.IsZero() {
		createdAt = u.CreatedAt.UTC().Format(time.RFC3339)
	}

	return UserResponse{
		ID:         id,
		Email:      u.Email,
		Name:       u.Name,
		PictureURL: u.PictureURL,
		Role:       string(u.Role),
		CreatedAt:  createdAt,
	}
}
