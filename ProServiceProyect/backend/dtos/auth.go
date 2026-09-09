package dtos

import "backend/models"

// GoogleLoginRequest es el body de POST /auth/google: el ID token que el
// frontend obtuvo del login de Google.
type GoogleLoginRequest struct {
	IDToken string `json:"idToken"`
}

// RefreshRequest es el body de POST /auth/refresh.
type RefreshRequest struct {
	RefreshToken string `json:"refreshToken"`
}

// AuthResponse es la respuesta de un login o un refresh exitoso: el usuario y
// el par de tokens propios del backend. De acá en adelante el frontend usa el
// accessToken, no el de Google.
type AuthResponse struct {
	User         UserResponse `json:"user"`
	AccessToken  string       `json:"accessToken"`
	RefreshToken string       `json:"refreshToken"`
}

// NewAuthResponse mapea el resultado del service a la respuesta HTTP.
func NewAuthResponse(user models.User, accessToken, refreshToken string) AuthResponse {
	return AuthResponse{
		User:         NewUserResponse(user),
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
	}
}
