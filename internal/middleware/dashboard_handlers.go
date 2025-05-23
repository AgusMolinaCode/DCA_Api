package middleware

import (
	"github.com/gin-gonic/gin"
	"github.com/AgusMolinaCode/DCA_Api.git/internal/database"
	"github.com/AgusMolinaCode/DCA_Api.git/internal/repository"
	"net/http"
	"time"
)

// GetDashboard obtiene el dashboard del usuario con información de todas sus criptomonedas
func GetDashboard(c *gin.Context) {
	// Obtener el ID del usuario del contexto
	userID, exists := c.Get("userId")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Usuario no autenticado"})
		return
	}

	// Convertir el ID a string
	userIDStr := userID.(string)

	// Obtener el dashboard usando la conexión a la base de datos
	dashboard, err := repository.GetUserDashboard(database.DB, userIDStr)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, dashboard)
}

// GetPerformance obtiene el rendimiento de las inversiones del usuario
func GetPerformance(c *gin.Context) {
	// Obtener el ID del usuario del contexto
	userID, exists := c.Get("userId")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Usuario no autenticado"})
		return
	}

	// Convertir el ID a string
	userIDStr := userID.(string)

	// Obtener el periodo (opcional)
	period := c.DefaultQuery("period", "all")

	// Determinar la fecha de inicio según el periodo
	var startDate time.Time
	now := time.Now()

	switch period {
	case "day":
		startDate = now.AddDate(0, 0, -1)
	case "week":
		startDate = now.AddDate(0, 0, -7)
	case "month":
		startDate = now.AddDate(0, -1, 0)
	case "year":
		startDate = now.AddDate(-1, 0, 0)
	default: // "all"
		startDate = time.Date(2000, 1, 1, 0, 0, 0, 0, time.UTC)
	}

	// Obtener el rendimiento
	performance, err := repository.GetUserPerformance(database.DB, userIDStr, startDate)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, performance)
}

// GetHoldings obtiene las tenencias actuales del usuario
func GetHoldings(c *gin.Context) {
	// Obtener el ID del usuario del contexto
	userID, exists := c.Get("userId")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Usuario no autenticado"})
		return
	}

	// Convertir el ID a string
	userIDStr := userID.(string)

	// Obtener las tenencias
	holdingsRepo := repository.NewHoldingsRepository(database.DB)
	holdings, err := holdingsRepo.GetHoldings(userIDStr)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, holdings)
}

// GetCurrentBalance obtiene el balance actual del usuario
func GetCurrentBalance(c *gin.Context) {
	// Obtener el ID del usuario del contexto
	userID, exists := c.Get("userId")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Usuario no autenticado"})
		return
	}

	// Convertir el ID a string
	userIDStr := userID.(string)

	// Obtener el balance usando la funciu00f3n existente
	balance, err := repository.GetUserCurrentBalance(database.DB, userIDStr)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, balance)
}

// GetUserInvestmentHistory obtiene el historial de inversiones del usuario
// Esta función es específica para el dashboard y muestra el historial de inversiones del usuario
func GetUserInvestmentHistory(c *gin.Context) {
	// Obtener el ID del usuario del contexto
	userID, exists := c.Get("userId")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Usuario no autenticado"})
		return
	}

	// Convertir el ID a string
	userIDStr := userID.(string)

	// Obtener el periodo (opcional)
	period := c.DefaultQuery("period", "all")

	// Determinar la fecha de inicio según el periodo
	var startDate time.Time
	now := time.Now()

	switch period {
	case "day":
		startDate = now.AddDate(0, 0, -1)
	case "week":
		startDate = now.AddDate(0, 0, -7)
	case "month":
		startDate = now.AddDate(0, -1, 0)
	case "year":
		startDate = now.AddDate(-1, 0, 0)
	default: // "all"
		startDate = time.Date(2000, 1, 1, 0, 0, 0, 0, time.UTC)
	}

	// Obtener el historial de inversiones
	cryptoRepo := repository.NewCryptoRepository(database.DB)
	history, err := cryptoRepo.GetInvestmentSnapshotsWithMaxMin(userIDStr, startDate)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, history)
}

// GetDashboardLiveBalance obtiene el balance en tiempo real del usuario para el dashboard
// Esta función es específica para el dashboard y muestra el balance actualizado
func GetDashboardLiveBalance(c *gin.Context) {
	// Obtener el ID del usuario del contexto
	userID, exists := c.Get("userId")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Usuario no autenticado"})
		return
	}

	// Convertir el ID a string
	userIDStr := userID.(string)

	// Obtener el balance en tiempo real usando la función existente
	balance, err := repository.GetUserLiveBalance(database.DB, userIDStr)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, balance)
}
