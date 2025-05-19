package middleware

import (
	"github.com/gin-gonic/gin"
	"github.com/AgusMolinaCode/DCA_Api.git/internal/database"
	"github.com/AgusMolinaCode/DCA_Api.git/internal/models"
	"github.com/AgusMolinaCode/DCA_Api.git/internal/repository"
	"net/http"
	"time"
)

// GetDashboard obtiene el dashboard del usuario con información de todas sus criptomonedas
func GetDashboard(c *gin.Context) {
	// Validar que el usuario esté autenticado
	user, exists := c.Get("user")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Usuario no autenticado"})
		return
	}

	// Convertir el usuario a modelo
	userModel := user.(*models.User)

	// Obtener el dashboard usando la conexión a la base de datos
	dashboard, err := repository.GetUserDashboard(database.DB, userModel.ID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, dashboard)
}

// GetPerformance obtiene el rendimiento de las inversiones del usuario
func GetPerformance(c *gin.Context) {
	// Validar que el usuario esté autenticado
	user, exists := c.Get("user")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Usuario no autenticado"})
		return
	}

	// Convertir el usuario a modelo
	userModel := user.(*models.User)

	// Obtener el periodo (opcional)
	period := c.DefaultQuery("period", "all")

	// Determinar la fecha de inicio según el periodo
	var startDate time.Time
	now := time.Now()

	switch period {
	case "week":
		startDate = now.AddDate(0, 0, -7)
	case "month":
		startDate = now.AddDate(0, -1, 0)
	case "quarter":
		startDate = now.AddDate(0, -3, 0)
	case "year":
		startDate = now.AddDate(-1, 0, 0)
	default:
		// "all" o cualquier otro valor, usar la fecha más antigua
		startDate = time.Time{} // Fecha cero
	}

	// Obtener el rendimiento usando la conexión a la base de datos
	performance, err := repository.GetUserPerformance(database.DB, userModel.ID, startDate)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, performance)
}

// GetHoldings obtiene las tenencias actuales del usuario
func GetHoldings(c *gin.Context) {
	// Validar que el usuario esté autenticado
	user, exists := c.Get("user")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Usuario no autenticado"})
		return
	}

	// Convertir el usuario a modelo
	userModel := user.(*models.User)

	// Obtener las tenencias usando la conexión a la base de datos
	holdings, err := repository.GetUserHoldings(database.DB, userModel.ID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, holdings)
}

// GetCurrentBalance obtiene el balance actual del usuario
func GetCurrentBalance(c *gin.Context) {
	// Validar que el usuario esté autenticado
	user, exists := c.Get("user")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Usuario no autenticado"})
		return
	}

	// Convertir el usuario a modelo
	userModel := user.(*models.User)

	// Obtener el balance actual usando la conexión a la base de datos
	balance, err := repository.GetUserCurrentBalance(database.DB, userModel.ID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"current_balance": balance})
}

// GetUserInvestmentHistory obtiene el historial de inversiones del usuario
// Esta función es específica para el dashboard y muestra el historial de inversiones del usuario
func GetUserInvestmentHistory(c *gin.Context) {
	// Validar que el usuario esté autenticado
	user, exists := c.Get("user")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Usuario no autenticado"})
		return
	}

	// Convertir el usuario a modelo
	userModel := user.(*models.User)

	// Obtener el periodo (opcional)
	period := c.DefaultQuery("period", "all")

	// Determinar la fecha de inicio según el periodo
	var startDate time.Time
	now := time.Now()

	switch period {
	case "week":
		startDate = now.AddDate(0, 0, -7)
	case "month":
		startDate = now.AddDate(0, -1, 0)
	case "quarter":
		startDate = now.AddDate(0, -3, 0)
	case "year":
		startDate = now.AddDate(-1, 0, 0)
	default:
		// "all" o cualquier otro valor, usar la fecha más antigua
		startDate = time.Time{} // Fecha cero
	}

	// Obtener el historial de inversiones usando la conexión a la base de datos
	history, err := repository.GetUserInvestmentHistory(database.DB, userModel.ID, startDate)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, history)
}

// GetLiveBalance obtiene el balance en tiempo real del usuario
// GetUserLiveBalance obtiene el balance en tiempo real del usuario
// Esta función es específica para el dashboard y muestra el balance actualizado
func GetUserLiveBalance(c *gin.Context) {
	// Validar que el usuario esté autenticado
	user, exists := c.Get("user")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Usuario no autenticado"})
		return
	}

	// Convertir el usuario a modelo
	userModel := user.(*models.User)

	// Obtener el balance en tiempo real usando la conexión a la base de datos
	balance, err := repository.GetUserLiveBalance(database.DB, userModel.ID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"live_balance": balance})
}
