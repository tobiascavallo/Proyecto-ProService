package handlers

import (
	"errors"
	"log"
	"net/http"

	"backend/dtos"
	"backend/utils"

	"github.com/gin-gonic/gin"
)

// respondError es el único lugar donde un error de dominio se traduce a un
// status HTTP, según la tabla del CLAUDE.md. Los errores inesperados (500) se
// loguean completos del lado del servidor y devuelven un mensaje genérico para
// no filtrar detalles internos de Mongo o del driver.
func respondError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, utils.ErrValidation):
		writeError(c, http.StatusBadRequest, "VALIDATION", err.Error())
	case errors.Is(err, utils.ErrUnauthorized):
		writeError(c, http.StatusUnauthorized, "UNAUTHORIZED", err.Error())
	case errors.Is(err, utils.ErrForbidden):
		writeError(c, http.StatusForbidden, "FORBIDDEN", err.Error())
	case errors.Is(err, utils.ErrNotFound):
		writeError(c, http.StatusNotFound, "NOT_FOUND", err.Error())
	case errors.Is(err, utils.ErrConflict):
		writeError(c, http.StatusConflict, "CONFLICT", err.Error())
	default:
		log.Printf("handler: unexpected error: %v", err)
		writeError(c, http.StatusInternalServerError, "INTERNAL", "internal server error")
	}
}

// writeError escribe la respuesta de error con el formato único de la API y
// corta la ejecución del handler.
func writeError(c *gin.Context, status int, code, message string) {
	c.AbortWithStatusJSON(status, dtos.NewErrorResponse(code, message))
}
