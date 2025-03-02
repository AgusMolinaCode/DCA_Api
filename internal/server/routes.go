package routes

import (
	"github.com/AgusMolinaCode/DCA_Api.git/internal/database"
	"github.com/AgusMolinaCode/DCA_Api.git/internal/middleware"
	"github.com/gin-gonic/gin"
)

func RegisterRoutes(router *gin.Engine) {
	// Inicializar base de datos primero
	if err := database.InitDB(); err != nil {
		panic(err)
	}

	// Luego inicializar repositorios
	middleware.InitAuth()
	middleware.InitCrypto()

	router.POST("/signup", middleware.Signup)
	router.POST("/login", middleware.Login)

	protected := router.Group("/")
	protected.Use(middleware.AuthMiddleware())
	{
		protected.PUT("/users", middleware.UpdateUser)
		protected.DELETE("/users", middleware.DeleteUser)

		protected.POST("/transactions", middleware.CreateTransaction)
		protected.GET("/transactions", middleware.GetUserTransactions)
		protected.GET("/transactions/:id", middleware.GetTransactionDetails)
		protected.PUT("/transactions/:id", middleware.UpdateTransaction)
		protected.DELETE("/transactions/:id", middleware.DeleteTransaction)
		protected.GET("/recent-transactions", middleware.GetRecentTransactions)
		protected.GET("/dashboard", middleware.GetDashboard)
		protected.GET("/performance", middleware.GetPerformance)
		protected.GET("/holdings", middleware.GetHoldings)
		protected.GET("/current-balance", middleware.GetCurrentBalance)
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
