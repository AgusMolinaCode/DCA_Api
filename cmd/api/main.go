package main

import (
	"log"
	"os"

	"github.com/AgusMolinaCode/DCA_Api.git/internal/server"
	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"
)

func main() {
	// Cargar variables de entorno
	if err := godotenv.Load(); err != nil {
		log.Printf("No se pudo cargar el archivo .env: %v", err)
	}

	// Crear el router de Gin
	router := gin.Default()

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
