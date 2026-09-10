package middleware

import (
	"net/http"

	"backend/dtos"

	"github.com/gin-gonic/gin"
)

// RequireRole corta con 403 si el rol que dejó RequireAuth en el contexto no
// está entre los permitidos. Se encadena SIEMPRE después de RequireAuth: si no
// hay rol en el contexto, responde 401.
func RequireRole(allowed ...string) gin.HandlerFunc {
	permitted := make(map[string]struct{}, len(allowed))
	for _, role := range allowed {
		permitted[role] = struct{}{}
	}

	return func(c *gin.Context) {
		role, ok := GetRole(c)
		if !ok {
			c.AbortWithStatusJSON(http.StatusUnauthorized, dtos.NewErrorResponse("UNAUTHORIZED", "missing or invalid token"))
			return
		}

		if _, allowedRole := permitted[role]; !allowedRole {
			c.AbortWithStatusJSON(http.StatusForbidden, dtos.NewErrorResponse("FORBIDDEN", "insufficient role"))
			return
		}

		c.Next()
	}
}
