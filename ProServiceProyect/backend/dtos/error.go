package dtos

// ErrorResponse es el formato único de error de toda la API, sin excepción:
// { "error": { "code": "...", "message": "..." } }.
type ErrorResponse struct {
	Error ErrorBody `json:"error"`
}

// ErrorBody es el cuerpo del error: un código estable para que el frontend
// discrimine y un mensaje legible.
type ErrorBody struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

// NewErrorResponse arma la respuesta de error con el formato estándar.
func NewErrorResponse(code, message string) ErrorResponse {
	return ErrorResponse{Error: ErrorBody{Code: code, Message: message}}
}
