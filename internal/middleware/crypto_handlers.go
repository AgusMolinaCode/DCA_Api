package middleware

import (
	"fmt"
	"net/http"
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

	// Obtener el ID del usuario del contexto (establecido por AuthMiddleware)
	userID := c.GetString("userId")
	tx.UserID = userID
	tx.ID = fmt.Sprintf("%d", time.Now().UnixNano())

	// Calcular el total
	tx.Total = tx.Amount * tx.PurchasePrice

	// Si no se proporciona fecha, usar la actual
	if tx.Date.IsZero() {
		tx.Date = time.Now()
	}

	if err := cryptoRepo.CreateTransaction(&tx); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Error al crear la transacción"})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"message":     "Transacción creada exitosamente",
		"transaction": tx,
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
