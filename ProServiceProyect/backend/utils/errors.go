package utils

import "errors"

// Errores de dominio tipados de la aplicación. Viven en utils porque es el
// paquete transversal que cualquier capa puede importar: así los services y
// este mismo paquete (ValidateToken) devuelven exactamente el mismo sentinel y
// errors.Is funciona de punta a punta. Los handlers son los únicos que los
// traducen a un código HTTP.
var (
	ErrValidation   = errors.New("validation error")
	ErrUnauthorized = errors.New("unauthorized")
	ErrForbidden    = errors.New("forbidden")
	ErrNotFound     = errors.New("not found")
	ErrConflict     = errors.New("conflict")
)
