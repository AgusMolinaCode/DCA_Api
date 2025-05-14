package main

import (
	"log"
	"os"
	"time"

	"github.com/AgusMolinaCode/DCA_Api.git/internal/database"
	"github.com/AgusMolinaCode/DCA_Api.git/internal/middleware"
	routes "github.com/AgusMolinaCode/DCA_Api.git/internal/server"
	"github.com/AgusMolinaCode/DCA_Api.git/internal/services"
	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"
)

// Instancia global del actualizador de precios
var priceUpdater *services.PriceUpdater

func main() {
	// Cargar variables de entorno
	if err := godotenv.Load(); err != nil {
		log.Printf("No se pudo cargar el archivo .env: %v", err)
	}

	// Crear el router de Gin
	router := gin.Default()

	// Configurar CORS
	config := cors.DefaultConfig()
	config.AllowOrigins = []string{"http://localhost:3000"}
	config.AllowMethods = []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"}
	config.AllowHeaders = []string{"Origin", "Content-Type", "Accept", "Authorization", "Admin-Key"}
	config.AllowCredentials = true
	config.ExposeHeaders = []string{"Content-Length"}
	router.Use(cors.New(config))

	// Inicializar base de datos
	if err := database.InitDB(); err != nil {
		log.Fatalf("Error al inicializar la base de datos: %v", err)
	}
	defer database.DB.Close()

	// Inicializar auth
	middleware.InitAuth()

	// Iniciar el servicio de actualización de precios (cada 15 segundos)
	priceUpdater = services.NewPriceUpdater(15 * time.Second)
	priceUpdater.Start()
	defer priceUpdater.Stop()

	// Hacer disponible el actualizador de precios para los handlers
	middleware.SetPriceUpdater(priceUpdater)

	// Configurar las rutas
	routes.RegisterRoutes(router)

	// Iniciar el servidor
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	if err := router.Run(":" + port); err != nil {
		log.Fatalf("Error al iniciar el servidor: %v", err)
	}
}
