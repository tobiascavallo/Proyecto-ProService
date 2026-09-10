package services

import (
	"context"
	"log"
	"strings"
	"time"

	"backend/models"
	"backend/utils"

	"go.mongodb.org/mongo-driver/v2/bson"
)

// GoogleVerifier valida un ID token de Google y devuelve la identidad. La
// implementación concreta (utils.GoogleTokenVerifier) hace la llamada de red;
// el service solo conoce esta interfaz para poder testear con un mock.
type GoogleVerifier interface {
	Verify(ctx context.Context, idToken string) (utils.GoogleIdentity, error)
}

// Iterfaz
// UserProvider es lo que AuthService necesita de la gestión de usuarios.
// Lo cumple *UserService.
type UserProvider interface {
	FindOrCreateByGoogle(ctx context.Context, googleID, email, name, pictureURL string) (*models.User, error)
	GetByID(ctx context.Context, id bson.ObjectID) (*models.User, error)
}

// TokenPair es el par de tokens propios que emite el backend.
type TokenPair struct {
	AccessToken  string
	RefreshToken string
}

// AuthService orquesta el login con Google y el refresh de tokens. No sabe de
// HTTP ni de Mongo.
type AuthService struct {
	google     GoogleVerifier
	users      UserProvider
	jwtSecret  string
	accessTTL  time.Duration
	refreshTTL time.Duration
}

// NewAuthService recibe las interfaces que consume y los parámetros de firma
// (secret y expiraciones salen de la config).
func NewAuthService(google GoogleVerifier, users UserProvider, jwtSecret string, accessTTL, refreshTTL time.Duration) *AuthService {
	return &AuthService{
		google:     google,
		users:      users,
		jwtSecret:  jwtSecret,
		accessTTL:  accessTTL,
		refreshTTL: refreshTTL,
	}
}

// LoginWithGoogle valida el ID token de Google, busca o crea el usuario y emite
// el par de tokens propios. Es el paso 2 al 5 del flujo de autenticación.
func (s *AuthService) LoginWithGoogle(ctx context.Context, idToken string) (*models.User, TokenPair, error) {
	if strings.TrimSpace(idToken) == "" {
		return nil, TokenPair{}, utils.ErrValidation
	}

	identity, err := s.google.Verify(ctx, idToken)
	if err != nil {
		return nil, TokenPair{}, err
	}

	user, err := s.users.FindOrCreateByGoogle(ctx, identity.GoogleID, identity.Email, identity.Name, identity.PictureURL)
	if err != nil {
		return nil, TokenPair{}, err
	}

	pair, err := s.issueTokens(user)
	if err != nil {
		return nil, TokenPair{}, err
	}

	return user, pair, nil
}

// Refresh valida un refresh token propio y emite un par nuevo. Rechaza tokens
// que no sean de tipo refresh y vuelve a leer el usuario de la DB, para que un
// cambio de rol (client -> worker) o una baja se reflejen en el token nuevo.
func (s *AuthService) Refresh(ctx context.Context, refreshToken string) (*models.User, TokenPair, error) {
	if strings.TrimSpace(refreshToken) == "" {
		return nil, TokenPair{}, utils.ErrValidation
	}

	claims, err := utils.ValidateToken(refreshToken, s.jwtSecret)
	if err != nil {
		return nil, TokenPair{}, err
	}
	if claims.Type != utils.TokenTypeRefresh {
		return nil, TokenPair{}, utils.ErrUnauthorized
	}

	id, err := bson.ObjectIDFromHex(claims.UserID)
	if err != nil {
		return nil, TokenPair{}, utils.ErrUnauthorized
	}

	user, err := s.users.GetByID(ctx, id)
	if err != nil {
		return nil, TokenPair{}, err
	}

	pair, err := s.issueTokens(user)
	if err != nil {
		return nil, TokenPair{}, err
	}

	return user, pair, nil
}

// issueTokens firma el access y el refresh para un usuario ya resuelto.
func (s *AuthService) issueTokens(user *models.User) (TokenPair, error) {
	userID := user.ID.Hex()
	role := string(user.Role)

	access, err := utils.GenerateAccessToken(userID, role, s.jwtSecret, s.accessTTL)
	if err != nil {
		log.Printf("auth service: access token generation failed: %v", err)
		return TokenPair{}, err
	}

	refresh, err := utils.GenerateRefreshToken(userID, role, s.jwtSecret, s.refreshTTL)
	if err != nil {
		log.Printf("auth service: refresh token generation failed: %v", err)
		return TokenPair{}, err
	}

	return TokenPair{AccessToken: access, RefreshToken: refresh}, nil
}
