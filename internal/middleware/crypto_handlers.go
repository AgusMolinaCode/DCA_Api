package middleware

import (
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/AgusMolinaCode/DCA_Api.git/internal/database"
	"github.com/AgusMolinaCode/DCA_Api.git/internal/models"
	"github.com/AgusMolinaCode/DCA_Api.git/internal/repository"
	"github.com/gin-gonic/gin"
)

var cryptoRepo *repository.CryptoRepository

func InitCrypto() {
	cryptoRepo = repository.NewCryptoRepository(database.DB)
}

func CreateTransaction(c *gin.Context) {
	var tx models.CryptoTransaction
	if err := c.ShouldBindJSON(&tx); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	userID := c.GetString("userId")
	tx.UserID = userID
	tx.ID = fmt.Sprintf("%d", time.Now().UnixNano())
	tx.Total = tx.Amount * tx.PurchasePrice

	if tx.Date.IsZero() {
		tx.Date = time.Now()
	}

	if err := cryptoRepo.CreateTransaction(&tx); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Error al crear la transacción"})
		return
	}

	// Obtener los detalles de la transacción recién creada
	details, err := cryptoRepo.GetTransactionDetails(userID, tx.ID)
	if err != nil {
		c.JSON(http.StatusCreated, gin.H{
			"message":     "Transacción creada exitosamente",
			"transaction": tx,
		})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"message": "Transacción creada exitosamente",
		"details": details,
	})
}

func GetUserTransactions(c *gin.Context) {
	userID := c.GetString("userId")

	transactions, err := cryptoRepo.GetUserTransactions(userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Error al obtener las transacciones"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"transactions": transactions})
}

func GetDashboard(c *gin.Context) {
	userID := c.GetString("userId")

	dashboard, err := cryptoRepo.GetCryptoDashboard(userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Error al obtener el dashboard"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"dashboard": dashboard})
}

func GetTransactionDetails(c *gin.Context) {
	userID := c.GetString("userId")
	transactionID := c.Param("id")

	details, err := cryptoRepo.GetTransactionDetails(userID, transactionID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Error al obtener los detalles de la transacción"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"details": details})
}

func GetRecentTransactions(c *gin.Context) {
	userID := c.GetString("userId")

	// Por defecto mostrar las últimas 10 transacciones
	limit := 10
	if limitStr := c.Query("limit"); limitStr != "" {
		if l, err := strconv.Atoi(limitStr); err == nil && l > 0 {
			limit = l
		}
	}

	details, err := cryptoRepo.GetRecentTransactions(userID, limit)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Error al obtener las transacciones recientes"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"recent_transactions": details,
	})
}

func GetPerformance(c *gin.Context) {
	userID := c.GetString("userId")

	performance, err := cryptoRepo.GetPerformance(userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Error al obtener el rendimiento"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"performance": performance})
}

func GetHoldings(c *gin.Context) {
	userID := c.GetString("userId")

	holdings, err := cryptoRepo.GetHoldings(userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Error al obtener los holdings"})
		return
	}

	c.JSON(http.StatusOK, holdings)
}
