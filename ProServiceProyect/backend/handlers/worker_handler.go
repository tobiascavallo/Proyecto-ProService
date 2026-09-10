package handlers

import (
	"context"
	"net/http"
	"strconv"

	"backend/dtos"
	"backend/models"
	"backend/services"
	"backend/utils"

	"github.com/gin-gonic/gin"
	"go.mongodb.org/mongo-driver/v2/bson"
)

// WorkerService define lo que el handler necesita de la capa de servicio.
type WorkerService interface {
	CreateProfile(ctx context.Context, userID bson.ObjectID, input services.WorkerProfileInput) (*models.Worker, error)
	GetByID(ctx context.Context, id bson.ObjectID) (*models.Worker, error)
	GetOwn(ctx context.Context, userID bson.ObjectID) (*models.Worker, error)
	UpdateOwn(ctx context.Context, userID bson.ObjectID, input services.WorkerProfileInput) (*models.Worker, error)
	DeactivateOwn(ctx context.Context, userID bson.ObjectID) error
	DeactivateByID(ctx context.Context, id bson.ObjectID) error
	Search(ctx context.Context, specialtyID *bson.ObjectID, page, pageSize int) ([]models.Worker, error)
}

// WorkerHandler traduce HTTP <-> service para el directorio y la gestión del
// perfil propio. No decide reglas de negocio.
type WorkerHandler struct {
	service WorkerService
}

// NewWorkerHandler recibe la interfaz del service, nunca el struct concreto.
func NewWorkerHandler(service WorkerService) *WorkerHandler {
	return &WorkerHandler{service: service}
}

// RegisterPublicRoutes engancha el directorio: búsqueda y ficha, sin auth.
func (h *WorkerHandler) RegisterPublicRoutes(rg *gin.RouterGroup) {
	rg.GET("/workers", h.Search)
	rg.GET("/workers/:id", h.GetByID)
}

// RegisterProtectedRoutes engancha la gestión del perfil propio (con JWT).
func (h *WorkerHandler) RegisterProtectedRoutes(rg *gin.RouterGroup) {
	rg.POST("/workers", h.CreateProfile)
	rg.GET("/workers/me", h.GetOwn)
	rg.PUT("/workers/me", h.UpdateOwn)
	rg.DELETE("/workers/me", h.DeactivateOwn)
}

// RegisterAdminRoutes engancha la moderación (JWT + rol admin en el grupo).
func (h *WorkerHandler) RegisterAdminRoutes(rg *gin.RouterGroup) {
	rg.DELETE("/workers/:id", h.Deactivate)
}

// Search devuelve la página del directorio que matchea el filtro.
func (h *WorkerHandler) Search(c *gin.Context) {
	var specialtyID *bson.ObjectID
	if raw := c.Query("specialtyId"); raw != "" {
		id, err := bson.ObjectIDFromHex(raw)
		if err != nil {
			respondError(c, utils.ErrValidation)
			return
		}
		specialtyID = &id
	}

	page, _ := strconv.Atoi(c.Query("page"))
	pageSize, _ := strconv.Atoi(c.Query("pageSize"))

	workers, err := h.service.Search(c.Request.Context(), specialtyID, page, pageSize)
	if err != nil {
		respondError(c, err)
		return
	}

	c.JSON(http.StatusOK, dtos.NewWorkerListResponse(workers))
}

// GetByID devuelve la ficha pública de un perfil.
func (h *WorkerHandler) GetByID(c *gin.Context) {
	id, err := bson.ObjectIDFromHex(c.Param("id"))
	if err != nil {
		// Un id mal formado no puede identificar ningún recurso.
		respondError(c, utils.ErrNotFound)
		return
	}

	worker, err := h.service.GetByID(c.Request.Context(), id)
	if err != nil {
		respondError(c, err)
		return
	}

	c.JSON(http.StatusOK, dtos.NewWorkerResponse(*worker))
}

// CreateProfile crea el perfil del usuario autenticado.
func (h *WorkerHandler) CreateProfile(c *gin.Context) {
	userID, ok := currentUserID(c)
	if !ok {
		respondError(c, utils.ErrUnauthorized)
		return
	}

	input, err := bindWorkerProfile(c)
	if err != nil {
		respondError(c, err)
		return
	}

	worker, err := h.service.CreateProfile(c.Request.Context(), userID, input)
	if err != nil {
		respondError(c, err)
		return
	}

	c.JSON(http.StatusCreated, dtos.NewWorkerResponse(*worker))
}

// GetOwn devuelve el perfil del usuario autenticado (activo o no).
func (h *WorkerHandler) GetOwn(c *gin.Context) {
	userID, ok := currentUserID(c)
	if !ok {
		respondError(c, utils.ErrUnauthorized)
		return
	}

	worker, err := h.service.GetOwn(c.Request.Context(), userID)
	if err != nil {
		respondError(c, err)
		return
	}

	c.JSON(http.StatusOK, dtos.NewWorkerResponse(*worker))
}

// UpdateOwn reemplaza los campos editables del perfil del usuario autenticado.
func (h *WorkerHandler) UpdateOwn(c *gin.Context) {
	userID, ok := currentUserID(c)
	if !ok {
		respondError(c, utils.ErrUnauthorized)
		return
	}

	input, err := bindWorkerProfile(c)
	if err != nil {
		respondError(c, err)
		return
	}

	worker, err := h.service.UpdateOwn(c.Request.Context(), userID, input)
	if err != nil {
		respondError(c, err)
		return
	}

	c.JSON(http.StatusOK, dtos.NewWorkerResponse(*worker))
}

// DeactivateOwn da de baja el perfil del usuario autenticado.
func (h *WorkerHandler) DeactivateOwn(c *gin.Context) {
	userID, ok := currentUserID(c)
	if !ok {
		respondError(c, utils.ErrUnauthorized)
		return
	}

	if err := h.service.DeactivateOwn(c.Request.Context(), userID); err != nil {
		respondError(c, err)
		return
	}

	c.Status(http.StatusNoContent)
}

// Deactivate da de baja un perfil por moderación del administrador.
func (h *WorkerHandler) Deactivate(c *gin.Context) {
	id, err := bson.ObjectIDFromHex(c.Param("id"))
	if err != nil {
		respondError(c, utils.ErrNotFound)
		return
	}

	if err := h.service.DeactivateByID(c.Request.Context(), id); err != nil {
		respondError(c, err)
		return
	}

	c.Status(http.StatusNoContent)
}

// bindWorkerProfile bindea el body y arma el input del service, parseando los
// specialtyIds a ObjectID. Un id mal formado es ErrValidation, igual que el
// resto de las reglas de la especialidad (que valida el service).
func bindWorkerProfile(c *gin.Context) (services.WorkerProfileInput, error) {
	var req dtos.WorkerProfileRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		return services.WorkerProfileInput{}, utils.ErrValidation
	}

	specialtyIDs := make([]bson.ObjectID, 0, len(req.SpecialtyIDs))
	for _, raw := range req.SpecialtyIDs {
		id, err := bson.ObjectIDFromHex(raw)
		if err != nil {
			return services.WorkerProfileInput{}, utils.ErrValidation
		}
		specialtyIDs = append(specialtyIDs, id)
	}

	return services.WorkerProfileInput{
		Bio:                  req.Bio,
		SpecialtyIDs:         specialtyIDs,
		Phone:                req.Phone,
		ContactEmail:         req.ContactEmail,
		SocialLinks:          req.SocialLinks,
		AvailabilityStatus:   models.AvailabilityStatus(req.AvailabilityStatus),
		AvailabilitySchedule: req.AvailabilitySchedule,
		Photos:               req.Photos,
	}, nil
}
