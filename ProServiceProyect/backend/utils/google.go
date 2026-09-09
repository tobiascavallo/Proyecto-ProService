package utils

import (
	"context"

	"google.golang.org/api/idtoken"
	"google.golang.org/api/option"
)

// GoogleIdentity es la identidad que devuelve Google tras validar un ID token.
type GoogleIdentity struct {
	GoogleID   string
	Email      string
	Name       string
	PictureURL string
}

// GoogleTokenVerifier valida ID tokens de Google contra un Client ID. Guarda un
// idtoken.Validator para reusar el cache de claves públicas de Google entre
// llamadas en vez de bajarlas en cada login.
type GoogleTokenVerifier struct {
	clientID  string
	validator *idtoken.Validator
}

// NewGoogleTokenVerifier arma el verificador. option.WithoutAuthentication deja
// claro que no hacen falta credenciales: validar un ID token solo necesita las
// claves públicas de Google, que son públicas.
func NewGoogleTokenVerifier(ctx context.Context, clientID string) (*GoogleTokenVerifier, error) {
	validator, err := idtoken.NewValidator(ctx, option.WithoutAuthentication())
	if err != nil {
		return nil, err
	}
	return &GoogleTokenVerifier{clientID: clientID, validator: validator}, nil
}

// Verify valida firma, iss, aud (contra clientID) y expiración del ID token
// contra las claves públicas de Google y devuelve la identidad. Cualquier
// problema de validación se colapsa en ErrUnauthorized.
func (v *GoogleTokenVerifier) Verify(ctx context.Context, idToken string) (GoogleIdentity, error) {
	payload, err := v.validator.Validate(ctx, idToken, v.clientID)
	if err != nil {
		return GoogleIdentity{}, ErrUnauthorized
	}

	return GoogleIdentity{
		GoogleID:   payload.Subject,
		Email:      claimString(payload, "email"),
		Name:       claimString(payload, "name"),
		PictureURL: claimString(payload, "picture"),
	}, nil
}

// claimString lee un claim de texto del payload tolerando que falte o venga con
// otro tipo (documentos incompletos no deben romper el login).
func claimString(payload *idtoken.Payload, key string) string {
	if value, ok := payload.Claims[key].(string); ok {
		return value
	}
	return ""
}
