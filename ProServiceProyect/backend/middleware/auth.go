package middleware

import (
	"net/http"
	"strings"

	"backend/dtos"
	"backend/utils"

	"github.com/gin-gonic/gin"
)

// Claves con las que RequireAuth deja los datos del token en el contexto. Se
// leen siempre de acá, nunca de nada que venga del frontend.
const (
	contextUserID = "userID"
	contextRole   = "role"
)

// RequireAuth valida el JWT propio del header "Authorization: Bearer <token>"
// y deja userID y role en el contexto. Si falta el token o es inválido corta
// la cadena con 401 en el formato de error estándar.
func RequireAuth(jwtSecret string) gin.HandlerFunc {
	return func(c *gin.Context) {
		token, ok := bearerToken(c.GetHeader("Authorization"))
		if !ok {
			abortUnauthorized(c)
			return
		}

		claims, err := utils.ValidateToken(token, jwtSecret)
		if err != nil {
			abortUnauthorized(c)
			return
		}

		// Un refresh token no autentica peticiones normales: solo sirve en
		// /auth/refresh.
		if claims.Type != utils.TokenTypeAccess {
			abortUnauthorized(c)
			return
		}

		c.Set(contextUserID, claims.UserID)
		c.Set(contextRole, claims.Role)
		c.Next()
	}
}

// GetUserID devuelve el user_id que RequireAuth puso en el contexto.
func GetUserID(c *gin.Context) (string, bool) {
	value, exists := c.Get(contextUserID)
	if !exists {
		return "", false
	}
	id, ok := value.(string)
	return id, ok
}

// GetRole devuelve el rol que RequireAuth puso en el contexto.
func GetRole(c *gin.Context) (string, bool) {
	value, exists := c.Get(contextRole)
	if !exists {
		return "", false
	}
	role, ok := value.(string)
	return role, ok
}

// bearerToken extrae el token de un header "Bearer <token>", tolerando
// mayúsculas/minúsculas en el esquema.
func bearerToken(header string) (string, bool) {
	const prefix = "bearer "
	if len(header) <= len(prefix) || !strings.EqualFold(header[:len(prefix)], prefix) {
		return "", false
	}
	token := strings.TrimSpace(header[len(prefix):])
	return token, token != ""
}

// abortUnauthorized responde 401 con el formato de error único de la API.
func abortUnauthorized(c *gin.Context) {
	c.AbortWithStatusJSON(http.StatusUnauthorized, dtos.NewErrorResponse("UNAUTHORIZED", "missing or invalid token"))
}
