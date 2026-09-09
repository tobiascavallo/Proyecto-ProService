package main

import (
	"context"
	"log"

	"backend/config"
	"backend/handlers"
	"backend/middleware"
	"backend/repositories"
	"backend/services"
	"backend/utils"

	"github.com/gin-gonic/gin"
)

// main arma el wiring de dependencias y levanta el servidor HTTP. Es el único
// lugar donde se instancian implementaciones concretas: de acá para abajo todo
// viaja por interfaces.
func main() {
	cfg := config.Load()

	// Se corta el arranque si Mongo no responde: preferible fallar acá a levantar
	// el servidor y recién enterarse del problema con la primera petición real.
	mongoClient, err := config.ConnectMongo(context.Background(), cfg.MongoURI)
	if err != nil {
		log.Fatalf("failed to connect to mongo: %v", err)
	}
	defer func() {
		if err := mongoClient.Disconnect(context.Background()); err != nil {
			log.Printf("error disconnecting from mongo: %v", err)
		}
	}()

	router := gin.New()

	// Middlewares obligatorios, en orden: recovery primero para atrapar cualquier
	// panic de los siguientes, logger para registrar toda petición, y CORS para
	// habilitar al frontend.
	router.Use(gin.Recovery())
	router.Use(gin.Logger())
	router.Use(middleware.CORS(cfg.CORSOrigin))

	// Healthcheck: además de que el servidor responda, confirma que Mongo esté
	// disponible, porque sin base el sistema no puede servir ninguna ruta real.
	router.GET("/health", func(c *gin.Context) {
		if err := mongoClient.Ping(c.Request.Context(), nil); err != nil {
			c.JSON(503, gin.H{"status": "degraded", "mongo": "unreachable"})
			return
		}
		c.JSON(200, gin.H{"status": "ok"})
	})

	db := mongoClient.Database(cfg.MongoDB)

	// Wiring por entidad: repository -> service -> handler. Cada capa recibe la
	// de abajo como interfaz.
	userRepository := repositories.NewUserRepository(db)
	userService := services.NewUserService(userRepository)
	userHandler := handlers.NewUserHandler(userService)

	// El verificador de Google baja y cachea las claves públicas al arrancar;
	// se corta si eso falla.
	googleVerifier, err := utils.NewGoogleTokenVerifier(context.Background(), cfg.GoogleClientID)
	if err != nil {
		log.Fatalf("failed to init google token verifier: %v", err)
	}
	authService := services.NewAuthService(googleVerifier, userService, cfg.JWTSecret, cfg.AccessTokenTTL, cfg.RefreshTokenTTL)
	authHandler := handlers.NewAuthHandler(authService)

	// Rutas pre-login: sin RequireAuth, porque son el paso previo a tener token.
	public := router.Group("/api/v1")
	authHandler.RegisterRoutes(public)

	// Rutas que exigen JWT válido. Las rutas públicas del directorio de workers
	// irán en otro grupo, también fuera de este middleware.
	protected := router.Group("/api/v1")
	protected.Use(middleware.RequireAuth(cfg.JWTSecret))
	userHandler.RegisterRoutes(protected)

	addr := ":" + cfg.Port
	log.Printf("server listening on %s", addr)
	if err := router.Run(addr); err != nil {
		log.Fatalf("server failed to start: %v", err)
	}
}
