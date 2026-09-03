package main

import (
	"log"

	"backend/config"
	"backend/middleware"

	"github.com/gin-gonic/gin"
)

// main arma el wiring de dependencias y levanta el servidor HTTP. Es el único
// lugar donde se instancian implementaciones concretas: de acá para abajo todo
// viaja por interfaces.
func main() {
	cfg := config.Load()

	router := gin.New()

	// Middlewares obligatorios, en orden: recovery primero para atrapar cualquier
	// panic de los siguientes, logger para registrar toda petición, y CORS para
	// habilitar al frontend.
	router.Use(gin.Recovery())
	router.Use(gin.Logger())
	router.Use(middleware.CORS(cfg.CORSOrigin))

	// Healthcheck para verificar de forma barata que el servidor responde.
	router.GET("/health", func(c *gin.Context) {
		c.JSON(200, gin.H{"status": "ok"})
	})

	// A medida que se implementa cada entidad se registran sus rutas acá.

	addr := ":" + cfg.Port
	log.Printf("server listening on %s", addr)
	if err := router.Run(addr); err != nil {
		log.Fatalf("server failed to start: %v", err)
	}
}
