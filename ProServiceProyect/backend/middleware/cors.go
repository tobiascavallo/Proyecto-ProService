package middleware

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// CORS habilita el intercambio de recursos con el frontend. El origen permitido
// llega por parámetro (viene de una variable de entorno, nunca hardcodeado) para
// que desarrollo y producción usen orígenes distintos sin tocar código.
func CORS(origin string) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Header("Access-Control-Allow-Origin", origin)
		c.Header("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE, OPTIONS")
		c.Header("Access-Control-Allow-Headers", "Origin, Content-Type, Authorization")
		c.Header("Access-Control-Allow-Credentials", "true")

		// El navegador manda un preflight OPTIONS antes de la request real:
		// se responde 204 y se corta acá porque no hay nada más que procesar.
		if c.Request.Method == http.MethodOptions {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}

		c.Next()
	}
}
