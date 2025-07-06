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
	allowedOrigins := []string{"http://localhost:3000"}
	if frontendURL := os.Getenv("FRONTEND_URL"); frontendURL != "" {
		allowedOrigins = append(allowedOrigins, frontendURL)
	}
	config.AllowOrigins = allowedOrigins
	config.AllowMethods = []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"}
	config.AllowHeaders = []string{"Origin", "Content-Type", "Accept", "X-API-Key", "Admin-Key"}
	config.AllowCredentials = true
	config.ExposeHeaders = []string{"Content-Length"}
	router.Use(cors.New(config))

	// Inicializar base de datos
	if err := database.InitDB(); err != nil {
		log.Fatalf("Error al inicializar la base de datos: %v", err)
	}
	defer database.DB.Close()


	// Iniciar el servicio de actualización de precios (snapshots cada minuto)
	log.Println("Iniciando servicio de actualización de precios...")
	priceUpdater = services.NewPriceUpdater(time.Minute) // El intervalo se ignora internamente
	priceUpdater.Start()
	defer func() {
		log.Println("Deteniendo servicio de actualización de precios...")
		priceUpdater.Stop()
	}()
	log.Println("Servicio de actualización de precios iniciado correctamente")

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
