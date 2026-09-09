package handlers

import (
	"context"
	"net/http"

	"backend/dtos"
	"backend/middleware"
	"backend/models"
	"backend/utils"

	"github.com/gin-gonic/gin"
	"go.mongodb.org/mongo-driver/v2/bson"
)

// UserService define lo que el handler necesita de la capa de servicio. El
// handler solo conoce esta interfaz, nunca el struct concreto, para poder
// testear con un mock.
type UserService interface {
	GetByID(ctx context.Context, id bson.ObjectID) (*models.User, error)
	UpdateProfile(ctx context.Context, id bson.ObjectID, name, pictureURL string) (*models.User, error)
	Delete(ctx context.Context, id bson.ObjectID) error
}

// UserHandler traduce HTTP <-> service para el perfil del usuario autenticado.
// No decide reglas de negocio: bindea, delega en el service y mapea el
// resultado a un DTO.
type UserHandler struct {
	service UserService
}

// NewUserHandler recibe la interfaz del service, nunca el struct concreto.
func NewUserHandler(service UserService) *UserHandler {
	return &UserHandler{service: service}
}

// RegisterRoutes engancha las rutas del usuario. rg tiene que ser un grupo ya
// protegido por el middleware de auth: los tres endpoints operan sobre el
// user_id del JWT.
func (h *UserHandler) RegisterRoutes(rg *gin.RouterGroup) {
	rg.GET("/users/me", h.GetMe)
	rg.PATCH("/users/me", h.UpdateMe)
	rg.DELETE("/users/me", h.DeleteMe)
}

// GetMe devuelve el perfil del usuario autenticado.
func (h *UserHandler) GetMe(c *gin.Context) {
	id, ok := currentUserID(c)
	if !ok {
		respondError(c, utils.ErrUnauthorized)
		return
	}

	user, err := h.service.GetByID(c.Request.Context(), id)
	if err != nil {
		respondError(c, err)
		return
	}

	c.JSON(http.StatusOK, dtos.NewUserResponse(*user))
}

// UpdateMe actualiza el nombre y la foto de perfil del usuario autenticado.
func (h *UserHandler) UpdateMe(c *gin.Context) {
	id, ok := currentUserID(c)
	if !ok {
		respondError(c, utils.ErrUnauthorized)
		return
	}

	var req dtos.UpdateUserRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, utils.ErrValidation)
		return
	}

	user, err := h.service.UpdateProfile(c.Request.Context(), id, req.Name, req.PictureURL)
	if err != nil {
		respondError(c, err)
		return
	}

	c.JSON(http.StatusOK, dtos.NewUserResponse(*user))
}

// DeleteMe elimina la cuenta del usuario autenticado.
func (h *UserHandler) DeleteMe(c *gin.Context) {
	id, ok := currentUserID(c)
	if !ok {
		respondError(c, utils.ErrUnauthorized)
		return
	}

	if err := h.service.Delete(c.Request.Context(), id); err != nil {
		respondError(c, err)
		return
	}

	c.Status(http.StatusNoContent)
}

// currentUserID lee el user_id que el middleware de auth dejó en el contexto y
// lo convierte a ObjectID. Un token válido con un uid ausente o mal formado se
// trata como no autorizado, no como error interno.
func currentUserID(c *gin.Context) (bson.ObjectID, bool) {
	raw, ok := middleware.GetUserID(c)
	if !ok {
		return bson.ObjectID{}, false
	}

	id, err := bson.ObjectIDFromHex(raw)
	if err != nil {
		return bson.ObjectID{}, false
	}

	return id, true
}
