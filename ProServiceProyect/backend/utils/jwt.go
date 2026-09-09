package utils

import (
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// Tipos de token propio. Van en el claim "type" para que un refresh token no
// pueda usarse como access token ni al revés.
const (
	TokenTypeAccess  = "access"
	TokenTypeRefresh = "refresh"
)

// Claims son los claims de los JWT propios del backend. Access y refresh
// comparten estructura: ambos identifican al usuario y su rol; cambian la
// expiración con la que se firman y el campo Type.
type Claims struct {
	UserID string `json:"user_id"`
	Role   string `json:"role"`
	Type   string `json:"type"`
	jwt.RegisteredClaims
}

// GenerateAccessToken firma un access token de expiración corta. El secret y la
// duración los pasa el caller (salen de la config): utils no los conoce por su
// cuenta, para no acoplarse a ninguna otra capa.
func GenerateAccessToken(userID, role, secret string, ttl time.Duration) (string, error) {
	return generateToken(userID, role, TokenTypeAccess, secret, ttl)
}

// GenerateRefreshToken firma un refresh token de expiración larga.
func GenerateRefreshToken(userID, role, secret string, ttl time.Duration) (string, error) {
	return generateToken(userID, role, TokenTypeRefresh, secret, ttl)
}

// generateToken arma y firma el token con HS256. Los tres tipos de Generate
// solo difieren en el ttl y el Type, así que la firma es una sola.
func generateToken(userID, role, tokenType, secret string, ttl time.Duration) (string, error) {
	now := time.Now()
	claims := Claims{
		UserID: userID,
		Role:   role,
		Type:   tokenType,
		RegisteredClaims: jwt.RegisteredClaims{
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(ttl)),
		},
	}

	return jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString([]byte(secret))
}

// ValidateToken valida la firma HS256 y la expiración del token y devuelve sus
// claims tipados. Rechaza explícitamente cualquier método de firma que no sea
// HMAC para cortar el ataque de algoritmo "none" y la confusión RS256/HS256.
// No discrimina access de refresh: eso lo decide el caller mirando claims.Type.
// Ante cualquier problema devuelve ErrUnauthorized.
func ValidateToken(tokenString, secret string) (*Claims, error) {
	claims := &Claims{}

	token, err := jwt.ParseWithClaims(tokenString, claims, func(t *jwt.Token) (any, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, ErrUnauthorized
		}
		return []byte(secret), nil
	})
	if err != nil || !token.Valid {
		return nil, ErrUnauthorized
	}

	// Un token bien firmado pero sin user_id no identifica a nadie: no sirve.
	if claims.UserID == "" {
		return nil, ErrUnauthorized
	}

	return claims, nil
}
