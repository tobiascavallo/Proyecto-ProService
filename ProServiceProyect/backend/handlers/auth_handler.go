package handlers

import (
	"context"
	"net/http"

	"backend/dtos"
	"backend/models"
	"backend/services"
	"backend/utils"

	"github.com/gin-gonic/gin"
)

// AuthService define lo que el handler necesita de la capa de servicio.
type AuthService interface {
	LoginWithGoogle(ctx context.Context, idToken string) (*models.User, services.TokenPair, error)
	Refresh(ctx context.Context, refreshToken string) (*models.User, services.TokenPair, error)
}

// AuthHandler traduce HTTP <-> service para el login con Google y el refresh de
// tokens. No decide reglas de negocio.
type AuthHandler struct {
	service AuthService
}

// NewAuthHandler recibe la interfaz del service, nunca el struct concreto.
func NewAuthHandler(service AuthService) *AuthHandler {
	return &AuthHandler{service: service}
}

// RegisterRoutes engancha las rutas de auth. rg tiene que ser un grupo SIN el
// middleware de auth: estos endpoints son el paso previo a tener un token.
func (h *AuthHandler) RegisterRoutes(rg *gin.RouterGroup) {
	rg.POST("/auth/google", h.LoginWithGoogle)
	rg.POST("/auth/refresh", h.Refresh)
}

// LoginWithGoogle recibe el ID token de Google y devuelve el usuario más el par
// de tokens propios.
func (h *AuthHandler) LoginWithGoogle(c *gin.Context) {
	var req dtos.GoogleLoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, utils.ErrValidation)
		return
	}

	user, tokens, err := h.service.LoginWithGoogle(c.Request.Context(), req.IDToken)
	if err != nil {
		respondError(c, err)
		return
	}

	c.JSON(http.StatusOK, dtos.NewAuthResponse(*user, tokens.AccessToken, tokens.RefreshToken))
}

// Refresh canjea un refresh token válido por un par de tokens nuevo.
func (h *AuthHandler) Refresh(c *gin.Context) {
	var req dtos.RefreshRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, utils.ErrValidation)
		return
	}

	user, tokens, err := h.service.Refresh(c.Request.Context(), req.RefreshToken)
	if err != nil {
		respondError(c, err)
		return
	}

	c.JSON(http.StatusOK, dtos.NewAuthResponse(*user, tokens.AccessToken, tokens.RefreshToken))
}
