package main

import (
	"context"
	"log"

	"backend/config"
	"backend/database"
	"backend/handlers"
	"backend/middleware"
	"backend/models"
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

	// Arranque: conectar a Mongo con ping, crear índices y sembrar el catálogo,
	// y recién ahí levantar el server. Si algo falla, cortar con error claro en
	// vez de arrancar a medias.
	mongoClient, err := database.Connect(context.Background(), cfg.MongoURI)
	if err != nil {
		log.Fatalf("failed to connect to mongo: %v", err)
	}
	defer func() {
		if err := mongoClient.Disconnect(context.Background()); err != nil {
			log.Printf("error disconnecting from mongo: %v", err)
		}
	}()

	db := mongoClient.Database(cfg.MongoDB)

	if err := database.EnsureIndexes(context.Background(), db); err != nil {
		log.Fatalf("failed to ensure indexes: %v", err)
	}
	if err := database.SeedSpecialties(context.Background(), db); err != nil {
		log.Fatalf("failed to seed specialties: %v", err)
	}

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

	// Wiring por entidad: repository -> service -> handler. Cada capa recibe la
	// de abajo como interfaz.
	userRepository := repositories.NewUserRepository(db)
	userService := services.NewUserService(userRepository)
	userHandler := handlers.NewUserHandler(userService)

	specialtyRepository := repositories.NewSpecialtyRepository(db)
	specialtyService := services.NewSpecialtyService(specialtyRepository)
	specialtyHandler := handlers.NewSpecialtyHandler(specialtyService)

	workerRepository := repositories.NewWorkerRepository(db)
	workerService := services.NewWorkerService(workerRepository, specialtyRepository, userService)
	workerHandler := handlers.NewWorkerHandler(workerService)

	// El verificador de Google baja y cachea las claves públicas al arrancar;
	// se corta si eso falla.
	googleVerifier, err := utils.NewGoogleTokenVerifier(context.Background(), cfg.GoogleClientID)
	if err != nil {
		log.Fatalf("failed to init google token verifier: %v", err)
	}
	authService := services.NewAuthService(googleVerifier, userService, cfg.JWTSecret, cfg.AccessTokenTTL, cfg.RefreshTokenTTL)
	authHandler := handlers.NewAuthHandler(authService)

	// Rutas públicas: sin RequireAuth. Login (pre-token), catálogo de
	// especialidades y directorio de workers (los consulta el invitado).
	public := router.Group("/api/v1")
	authHandler.RegisterRoutes(public)
	specialtyHandler.RegisterRoutes(public)
	workerHandler.RegisterPublicRoutes(public)

	// Rutas que exigen JWT válido.
	protected := router.Group("/api/v1")
	protected.Use(middleware.RequireAuth(cfg.JWTSecret))
	userHandler.RegisterRoutes(protected)
	workerHandler.RegisterProtectedRoutes(protected)

	// Rutas de moderación: JWT válido + rol admin.
	admin := router.Group("/api/v1")
	admin.Use(middleware.RequireAuth(cfg.JWTSecret), middleware.RequireRole(string(models.RoleAdmin)))
	workerHandler.RegisterAdminRoutes(admin)

	addr := ":" + cfg.Port
	log.Printf("server listening on %s", addr)
	if err := router.Run(addr); err != nil {
		log.Fatalf("server failed to start: %v", err)
	}
}
