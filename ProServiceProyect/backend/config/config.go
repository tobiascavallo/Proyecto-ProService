package config

import (
	"log"
	"os"
	"time"

	"github.com/joho/godotenv"
)

// Config agrupa toda la configuración del backend leída del entorno. Se carga
// una sola vez al arrancar y se inyecta a quien la necesite, para evitar
// llamadas a os.Getenv dispersas por todo el código.
type Config struct {
	Port            string
	MongoURI        string
	MongoDB         string
	JWTSecret       string
	GoogleClientID  string
	CORSOrigin      string
	AccessTokenTTL  time.Duration
	RefreshTokenTTL time.Duration
}

// Load lee el archivo .env si existe y arma el Config. Corta la ejecución de
// entrada si falta una variable crítica: es preferible no arrancar a arrancar
// mal y descubrir el problema más tarde con una request en producción.
func Load() *Config {
	// El .env es opcional: en producción las variables vienen del entorno real.
	if err := godotenv.Load(); err != nil {
		log.Println("no .env file found, reading config from environment")
	}

	return &Config{
		Port:            getEnv("PORT", "8080"),
		MongoURI:        mustEnv("MONGO_URI"),
		MongoDB:         mustEnv("MONGO_DB"),
		JWTSecret:       mustEnv("JWT_SECRET"),
		GoogleClientID:  mustEnv("GOOGLE_CLIENT_ID"),
		CORSOrigin:      mustEnv("CORS_ORIGIN"),
		AccessTokenTTL:  durationEnv("ACCESS_TOKEN_TTL", 15*time.Minute),
		RefreshTokenTTL: durationEnv("REFRESH_TOKEN_TTL", 7*24*time.Hour),
	}
}

// mustEnv devuelve el valor de la variable o corta la ejecución. Se usa para
// las variables sin las cuales el sistema no tiene sentido (DB, secretos).
func mustEnv(key string) string {
	value := os.Getenv(key)
	if value == "" {
		log.Fatalf("required environment variable %s is not set", key)
	}
	return value
}

// getEnv devuelve el valor de la variable o un fallback cuando está vacía.
func getEnv(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}

// durationEnv parsea una duración en formato Go ("15m", "168h"). Si la variable
// está vacía usa el fallback; si está pero es inválida corta el arranque, para
// no emitir tokens con una expiración silenciosamente incorrecta.
func durationEnv(key string, fallback time.Duration) time.Duration {
	raw := os.Getenv(key)
	if raw == "" {
		return fallback
	}

	parsed, err := time.ParseDuration(raw)
	if err != nil {
		log.Fatalf("environment variable %s is not a valid duration: %v", key, err)
	}
	return parsed
}
