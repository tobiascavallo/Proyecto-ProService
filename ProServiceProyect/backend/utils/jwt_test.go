package utils

import (
	"errors"
	"strings"
	"testing"
	"time"
)

const (
	testSecret = "test-secret-value"
	testTTL    = time.Hour
)

func TestValidateToken(t *testing.T) {
	validToken, err := GenerateAccessToken("507f1f77bcf86cd799439011", "client", testSecret, testTTL)
	if err != nil {
		t.Fatalf("no pude generar el token válido: %v", err)
	}

	expiredToken, err := GenerateAccessToken("u1", "client", testSecret, -time.Hour)
	if err != nil {
		t.Fatalf("no pude generar el token expirado: %v", err)
	}

	otherKeyToken, err := GenerateAccessToken("u1", "client", "another-secret", testTTL)
	if err != nil {
		t.Fatalf("no pude generar el token con otra clave: %v", err)
	}

	tamperedToken := tamperSignature(t, validToken)

	t.Run("valid token returns the claims", func(t *testing.T) {
		claims, err := ValidateToken(validToken, testSecret)
		if err != nil {
			t.Fatalf("no esperaba error, recibí %v", err)
		}
		if claims.UserID != "507f1f77bcf86cd799439011" {
			t.Errorf("user_id: esperaba %q, recibí %q", "507f1f77bcf86cd799439011", claims.UserID)
		}
		if claims.Role != "client" {
			t.Errorf("role: esperaba %q, recibí %q", "client", claims.Role)
		}
		if claims.Type != TokenTypeAccess {
			t.Errorf("type: esperaba %q, recibí %q", TokenTypeAccess, claims.Type)
		}
		if claims.ExpiresAt == nil || !claims.ExpiresAt.After(time.Now()) {
			t.Errorf("esperaba expiración futura, recibí %v", claims.ExpiresAt)
		}
	})

	t.Run("refresh token carries the refresh type", func(t *testing.T) {
		refresh, err := GenerateRefreshToken("u1", "client", testSecret, testTTL)
		if err != nil {
			t.Fatalf("no pude generar el refresh: %v", err)
		}
		claims, err := ValidateToken(refresh, testSecret)
		if err != nil {
			t.Fatalf("no esperaba error, recibí %v", err)
		}
		if claims.Type != TokenTypeRefresh {
			t.Errorf("type: esperaba %q, recibí %q", TokenTypeRefresh, claims.Type)
		}
	})

	rejected := []struct {
		name  string
		token string
	}{
		{name: "expired token is rejected", token: expiredToken},
		{name: "token signed with another key is rejected", token: otherKeyToken},
		{name: "token with tampered signature is rejected", token: tamperedToken},
	}

	for _, tt := range rejected {
		t.Run(tt.name, func(t *testing.T) {
			claims, err := ValidateToken(tt.token, testSecret)
			if claims != nil {
				t.Fatalf("esperaba claims nil, recibí %+v", claims)
			}
			if !errors.Is(err, ErrUnauthorized) {
				t.Fatalf("esperaba ErrUnauthorized, recibí %v", err)
			}
		})
	}
}

// tamperSignature cambia el primer carácter del segmento de firma por otro
// válido de base64url, manteniendo el formato header.payload.signature. El
// token resultante está bien formado pero su firma ya no verifica.
func tamperSignature(t *testing.T, token string) string {
	t.Helper()

	parts := strings.Split(token, ".")
	if len(parts) != 3 || parts[2] == "" {
		t.Fatalf("token de prueba con formato inesperado: %q", token)
	}

	sig := []byte(parts[2])
	if sig[0] == 'a' {
		sig[0] = 'b'
	} else {
		sig[0] = 'a'
	}
	parts[2] = string(sig)

	return strings.Join(parts, ".")
}
