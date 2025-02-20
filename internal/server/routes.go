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
		protected.PUT("/users", middleware.UpdateUser)
		protected.DELETE("/users", middleware.DeleteUser)
		// protected.GET("/events", getEvents)
		// protected.POST("/events", createEvent)
		// ... otras rutas protegidas
	}

	// Rutas de admin
	admin := router.Group("/admin")
	admin.Use(middleware.AdminAuth())
	{
		admin.GET("/users", middleware.GetUsers)
		admin.GET("/users/:id", middleware.GetUser)
	}

	router.POST("/request-reset-password", middleware.RequestResetPassword)
	router.POST("/reset-password", middleware.ResetPassword)
}
