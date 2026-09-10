package dtos

import "backend/models"

// SpecialtyResponse es la vista pública de una especialidad del catálogo. Se
// usa tanto en el formulario de alta del trabajador como en los filtros de
// búsqueda; created_at no aporta nada ahí y se omite.
type SpecialtyResponse struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	Slug string `json:"slug"`
}

// NewSpecialtyResponse mapea una Specialty de dominio a su DTO de salida.
func NewSpecialtyResponse(s models.Specialty) SpecialtyResponse {
	id := ""
	if !s.ID.IsZero() {
		id = s.ID.Hex()
	}

	return SpecialtyResponse{
		ID:   id,
		Name: s.Name,
		Slug: s.Slug,
	}
}

// NewSpecialtyListResponse mapea el catálogo completo. Devuelve un slice vacío,
// nunca nil, para que el JSON sea [] y no null.
func NewSpecialtyListResponse(specialties []models.Specialty) []SpecialtyResponse {
	out := make([]SpecialtyResponse, 0, len(specialties))
	for _, s := range specialties {
		out = append(out, NewSpecialtyResponse(s))
	}
	return out
}
