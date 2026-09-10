package handlers

import (
	"context"
	"net/http"

	"backend/dtos"
	"backend/models"

	"github.com/gin-gonic/gin"
)

// SpecialtyService define lo que el handler necesita de la capa de servicio.
type SpecialtyService interface {
	List(ctx context.Context) ([]models.Specialty, error)
}

// SpecialtyHandler expone el catálogo de especialidades por HTTP.
type SpecialtyHandler struct {
	service SpecialtyService
}

// NewSpecialtyHandler recibe la interfaz del service, nunca el struct concreto.
func NewSpecialtyHandler(service SpecialtyService) *SpecialtyHandler {
	return &SpecialtyHandler{service: service}
}

// RegisterRoutes engancha las rutas del catálogo. rg tiene que ser un grupo SIN
// el middleware de auth: el catálogo se consulta sin autenticación (formulario
// de alta del trabajador y filtros de búsqueda).
func (h *SpecialtyHandler) RegisterRoutes(rg *gin.RouterGroup) {
	rg.GET("/specialties", h.List)
}

// List devuelve el catálogo completo.
func (h *SpecialtyHandler) List(c *gin.Context) {
	specialties, err := h.service.List(c.Request.Context())
	if err != nil {
		respondError(c, err)
		return
	}

	c.JSON(http.StatusOK, dtos.NewSpecialtyListResponse(specialties))
}
