package routes

import (
	"github.com/AgusMolinaCode/DCA_Api.git/internal/middleware"
	"github.com/gin-gonic/gin"
)

func RegisterRoutes(router *gin.Engine) {
	router.POST("/signup", middleware.Signup)
	router.POST("/login", middleware.Login)

	protected := router.Group("/")
	protected.Use(middleware.AuthMiddleware())
	{
		// protected.GET("/events", getEvents)
		// protected.POST("/events", createEvent)
		// ... otras rutas protegidas
	}
}
