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
	middleware.InitBolsa() // Inicializar el repositorio de bolsas

	router.POST("/signup", middleware.Signup)
	router.POST("/login", middleware.Login)

	// Configurar ruta de logout con opciones
	router.OPTIONS("/logout", func(c *gin.Context) {
		c.Header("Access-Control-Allow-Origin", "http://localhost:3000")
		c.Header("Access-Control-Allow-Methods", "POST, OPTIONS")
		c.Header("Access-Control-Allow-Headers", "Authorization, Content-Type")
		c.Header("Access-Control-Allow-Credentials", "true")
		c.Status(200)
	})
	router.POST("/logout", middleware.AuthMiddleware(), middleware.Logout)

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
		protected.DELETE("/transactions/ticker/:ticker", middleware.DeleteTransactionsByTicker)
		protected.GET("/recent-transactions", middleware.GetRecentTransactions)
		protected.GET("/dashboard", middleware.GetDashboard)
		protected.GET("/performance", middleware.GetPerformance)
		protected.GET("/holdings", middleware.GetHoldings)
		protected.GET("/current-balance", middleware.GetCurrentBalance)
		protected.GET("/investment-history", middleware.GetInvestmentHistory)

		// Nuevas rutas para bolsas
		protected.POST("/bolsas", middleware.CreateBolsa)
		protected.GET("/bolsas", middleware.GetUserBolsas)
		protected.GET("/bolsas/:id", middleware.GetBolsaDetails)
		protected.POST("/bolsas/:id/assets", middleware.AddAssetsToBolsa)
		protected.PUT("/bolsas/:id", middleware.UpdateBolsa)
		protected.POST("/bolsas/:id/complete", middleware.CompleteBolsaAndTransfer)

		// Rutas para etiquetas de bolsas
		protected.POST("/bolsas/:id/tags", middleware.ManageBolsaTags)
		protected.GET("/bolsas/tags/:tag", middleware.GetBolsasByTag)

		// Agregar la ruta para balance en tiempo real
		protected.GET("/live-balance", middleware.GetDashboardLiveBalance)
		
		// Ruta para eliminar snapshots de inversión
		protected.DELETE("/investment/snapshots/:id", middleware.DeleteInvestmentSnapshot) 
	}

	// Configurar opciones para rutas de administración
	router.OPTIONS("/admin/*path", func(c *gin.Context) {
		c.Header("Access-Control-Allow-Origin", "http://localhost:3000")
		c.Header("Access-Control-Allow-Methods", "GET, POST, DELETE, OPTIONS")
		c.Header("Access-Control-Allow-Headers", "Authorization, Content-Type, Admin-Key")
		c.Header("Access-Control-Allow-Credentials", "true")
		c.Status(200)
	})

	// Rutas de admin
	admin := router.Group("/admin")
	admin.Use(middleware.AdminAuth())
	{
		admin.GET("/users", middleware.GetUsers)
		admin.GET("/users/:id", middleware.GetUser)
		admin.DELETE("/users/:id", middleware.DeleteUserByAdmin)
		admin.GET("/users/email/:email", middleware.GetUserByEmail)
	}

	router.POST("/request-reset-password", middleware.RequestResetPassword)
	router.POST("/reset-password", middleware.ResetPassword)
}
