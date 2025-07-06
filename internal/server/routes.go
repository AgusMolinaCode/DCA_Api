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
	middleware.InitCrypto()
	middleware.InitBolsa() // Inicializar el repositorio de bolsas
	middleware.InitClerk() // Inicializar Clerk

	// Health check endpoint
	router.GET("/health", func(c *gin.Context) {
		c.JSON(200, gin.H{"status": "ok"})
	})

	// Clerk authentication routes
	router.GET("/user", middleware.ClerkAuthMiddleware(), middleware.GetUserFromClerk)
	router.POST("/clerk/webhook", middleware.ClerkWebhookHandler)
	
	// Development endpoint to create test user
	router.POST("/dev/create-user", middleware.CreateTestUser)


	protected := router.Group("/")
	protected.Use(middleware.SimpleAPIKeyMiddleware())
	{

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
		protected.DELETE("/bolsas/:id/assets/:assetId", middleware.RemoveAssetFromBolsa)
		protected.PUT("/bolsas/:id", middleware.UpdateBolsa)
		protected.DELETE("/bolsas/:id", middleware.DeleteBolsa)
		protected.POST("/bolsas/:id/complete", middleware.CompleteBolsaAndTransfer)

		// Rutas para etiquetas de bolsas
		protected.POST("/bolsas/:id/tags", middleware.ManageBolsaTags)
		protected.GET("/bolsas/tags/:tag", middleware.GetBolsasByTag)

		// Agregar la ruta para balance en tiempo real
		protected.GET("/live-balance", middleware.GetDashboardLiveBalance)

		// Ruta para eliminar snapshots de inversión
		protected.DELETE("/investment/snapshots/:id", middleware.DeleteInvestmentSnapshot)
	}


}
