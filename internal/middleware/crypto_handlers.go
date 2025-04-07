package middleware

import (
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/AgusMolinaCode/DCA_Api.git/internal/database"
	"github.com/AgusMolinaCode/DCA_Api.git/internal/models"
	"github.com/AgusMolinaCode/DCA_Api.git/internal/repository"
	"github.com/AgusMolinaCode/DCA_Api.git/internal/services"
	"github.com/gin-gonic/gin"
)

var cryptoRepo *repository.CryptoRepository

func InitCrypto() {
	cryptoRepo = repository.NewCryptoRepository(database.DB)
}

func CreateTransaction(c *gin.Context) {
	var tx models.CryptoTransaction
	if err := c.ShouldBindJSON(&tx); err != nil {
		// Verificar si el error es por falta de purchase_price en una venta
		if tx.Type == models.TransactionTypeSell && tx.PurchasePrice <= 0 {
			// Obtener el precio actual para la venta
			cryptoData, err := services.GetCryptoPrice(tx.Ticker)
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Error obteniendo precio actual: %v", err)})
				return
			}

			// Usar el precio actual de la API
			tx.PurchasePrice = cryptoData.Raw[tx.Ticker]["USD"].PRICE

			// Continuar con la creación de la transacción
		} else {
			// Si es otro tipo de error, devolver el error
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
	}

	userID := c.GetString("userId")
	tx.UserID = userID
	tx.ID = fmt.Sprintf("%d", time.Now().UnixNano())

	// Validar el tipo de transacción
	if tx.Type == "" {
		tx.Type = models.TransactionTypeBuy // Por defecto es compra
	}

	if tx.Type != models.TransactionTypeBuy && tx.Type != models.TransactionTypeSell {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Tipo de transacción inválido. Debe ser 'compra' o 'venta'"})
		return
	}

	// Obtener la URL de la imagen del ticker
	imageURL, err := services.GetCryptoImageURL(tx.Ticker)
	if err != nil {
		// Si hay un error, solo lo registramos pero continuamos con la creación de la transacción
		log.Printf("Error al obtener la URL de la imagen para %s: %v", tx.Ticker, err)
	} else {
		// Guardar la URL de la imagen en la transacción
		tx.ImageURL = imageURL
	}

	// Si es una venta, verificar si el usuario tiene suficientes fondos
	if tx.Type == models.TransactionTypeSell {
		// Obtener el dashboard para verificar las tenencias actuales
		dashboard, err := cryptoRepo.GetCryptoDashboard(userID)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Error al verificar tenencias: %v", err)})
			return
		}

		// Buscar la criptomoneda en el dashboard
		var holdings float64 = 0
		cryptoFound := false
		for _, crypto := range dashboard {
			if crypto.Ticker == tx.Ticker {
				holdings = crypto.Holdings
				cryptoFound = true
				break
			}
		}

		// Si no se encontró la criptomoneda o no hay suficientes fondos
		if !cryptoFound {
			c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("No tienes %s en tu cartera", tx.Ticker)})
			return
		}

		if holdings < tx.Amount {
			c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("No tienes suficiente %s. Disponible: %f, Solicitado: %f", tx.Ticker, holdings, tx.Amount)})
			return
		}

		// Si no se especificó el precio, obtener precio actual
		if tx.PurchasePrice <= 0 {
			cryptoData, err := services.GetCryptoPrice(tx.Ticker)
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Error obteniendo precio actual: %v", err)})
				return
			}

			// Usar el precio actual de la API
			tx.PurchasePrice = cryptoData.Raw[tx.Ticker]["USD"].PRICE
		}
	}

	// Calcular el total
	tx.Total = tx.Amount * tx.PurchasePrice

	// Para ventas, calcular automáticamente los USDT recibidos
	if tx.Type == models.TransactionTypeSell {
		tx.USDTReceived = tx.Total
	}

	if tx.Date.IsZero() {
		tx.Date = time.Now()
	}

	if err := cryptoRepo.CreateTransaction(tx); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Error al crear la transacción: %v", err)})
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

	transactionsWithDetails, err := cryptoRepo.GetUserTransactionsWithDetails(userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Error al obtener las transacciones"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"transactions": transactionsWithDetails})
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

	// Crear un repositorio de tenencias
	holdingsRepo := repository.NewHoldingsRepository(database.DB)

	// Obtener las tenencias
	holdings, err := holdingsRepo.GetHoldings(userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Error al obtener las tenencias"})
		return
	}

	c.JSON(http.StatusOK, holdings)
}

func GetCurrentBalance(c *gin.Context) {
	userID := c.GetString("userId")

	// Crear un repositorio de tenencias
	holdingsRepo := repository.NewHoldingsRepository(database.DB)

	// Obtener las tenencias
	holdings, err := holdingsRepo.GetHoldings(userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Error al obtener el balance actual"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"total_current_value": holdings.TotalCurrentValue,
		"total_invested":      holdings.TotalInvested,
		"total_profit":        holdings.TotalProfit,
		"profit_percentage":   holdings.ProfitPercentage,
	})
}

// UpdateTransaction actualiza una transacción existente
func UpdateTransaction(c *gin.Context) {
	var tx models.CryptoTransaction
	if err := c.ShouldBindJSON(&tx); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	userID := c.GetString("userId")
	transactionID := c.Param("id")

	// Verificar que la transacción exista y pertenezca al usuario
	existingTx, err := cryptoRepo.GetTransactionDetails(userID, transactionID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Transacción no encontrada"})
		return
	}

	// Actualizar solo los campos permitidos
	tx.ID = transactionID
	tx.UserID = userID
	tx.CreatedAt = existingTx.Transaction.CreatedAt

	// Si es una venta, verificar si el usuario tiene suficientes fondos
	if tx.Type == models.TransactionTypeSell {
		// Obtener el dashboard para verificar las tenencias actuales
		dashboard, err := cryptoRepo.GetCryptoDashboard(userID)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Error al verificar tenencias: %v", err)})
			return
		}

		// Buscar la criptomoneda en el dashboard
		var holdings float64 = 0
		cryptoFound := false
		for _, crypto := range dashboard {
			if crypto.Ticker == tx.Ticker {
				holdings = crypto.Holdings
				cryptoFound = true
				break
			}
		}

		// Si no se encontró la criptomoneda o no hay suficientes fondos
		if !cryptoFound {
			c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("No tienes %s en tu cartera", tx.Ticker)})
			return
		}

		// Si estamos actualizando una venta existente, debemos considerar la cantidad original
		if existingTx.Transaction.Type == models.TransactionTypeSell && existingTx.Transaction.Ticker == tx.Ticker {
			// Ajustar las tenencias para considerar la cantidad original que ya fue vendida
			holdings += existingTx.Transaction.Amount
		}

		if holdings < tx.Amount {
			c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("No tienes suficiente %s. Disponible: %f, Solicitado: %f", tx.Ticker, holdings, tx.Amount)})
			return
		}

		// Si no se especificó el precio, obtener precio actual
		if tx.PurchasePrice <= 0 {
			cryptoData, err := services.GetCryptoPrice(tx.Ticker)
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Error obteniendo precio actual: %v", err)})
				return
			}

			// Usar el precio actual de la API
			tx.PurchasePrice = cryptoData.Raw[tx.Ticker]["USD"].PRICE
		}
	}

	// Calcular el total
	tx.Total = tx.Amount * tx.PurchasePrice

	// Para ventas, calcular automáticamente los USDT recibidos
	if tx.Type == models.TransactionTypeSell {
		tx.USDTReceived = tx.Total
	}

	if tx.Date.IsZero() {
		tx.Date = time.Now()
	}

	if err := cryptoRepo.UpdateTransaction(tx); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Error al actualizar la transacción: %v", err)})
		return
	}

	// Obtener los detalles de la transacción actualizada
	details, err := cryptoRepo.GetTransactionDetails(userID, tx.ID)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{
			"message":     "Transacción actualizada exitosamente",
			"transaction": tx,
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "Transacción actualizada exitosamente",
		"details": details,
	})
}

// DeleteTransaction elimina una transacción
func DeleteTransaction(c *gin.Context) {
	userID := c.GetString("userId")
	transactionID := c.Param("id")

	// Verificar que la transacción exista y pertenezca al usuario
	_, err := cryptoRepo.GetTransactionDetails(userID, transactionID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Transacción no encontrada"})
		return
	}

	if err := cryptoRepo.DeleteTransaction(userID, transactionID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Error al eliminar la transacción: %v", err)})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "Transacción eliminada exitosamente",
	})
}

// DeleteTransactionsByTicker elimina todas las transacciones de una criptomoneda
func DeleteTransactionsByTicker(c *gin.Context) {
	userID := c.GetString("userId")
	ticker := c.Param("ticker")

	if ticker == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Se requiere un ticker válido"})
		return
	}

	// Convertir ticker a mayúsculas para asegurar consistencia
	ticker = strings.ToUpper(ticker)

	err := cryptoRepo.DeleteTransactionsByTicker(userID, ticker)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Error al eliminar transacciones: %v", err)})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": fmt.Sprintf("Todas las transacciones de %s han sido eliminadas exitosamente", ticker)})
}
