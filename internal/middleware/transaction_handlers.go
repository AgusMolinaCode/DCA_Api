package middleware

import (
	"github.com/gin-gonic/gin"
	"github.com/AgusMolinaCode/DCA_Api.git/internal/models"
	"github.com/AgusMolinaCode/DCA_Api.git/internal/repository"
	"log"
	"net/http"
	"strconv"
)

// CreateTransaction crea una nueva transacciu00f3n para el usuario autenticado
func CreateTransaction(c *gin.Context) {
	var transaction models.CryptoTransaction
	if err := c.ShouldBindJSON(&transaction); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Obtener el ID del usuario del contexto
	userID, exists := c.Get("userId")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Usuario no autenticado"})
		return
	}

	// Convertir el ID a string
	userIDStr := userID.(string)
	transaction.UserID = userIDStr

	// Validar que el ticker exista
	if !repository.CryptoExists(transaction.Ticker) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Criptomoneda no encontrada"})
		return
	}

	// Crear la transacciu00f3n
	if err := repository.CreateTransaction(&transaction); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// Actualizar el balance del usuario
	if err := repository.UpdateUserBalance(userIDStr, transaction.Amount*transaction.PurchasePrice); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Error al actualizar balance"})
		return
	}

	// Crear snapshot automu00e1tico (versiu00f3n simplificada)
	// TODO: Implementar la creaciu00f3n real del snapshot
	log.Printf("Creando snapshot para usuario %s", userIDStr)

	c.JSON(http.StatusCreated, gin.H{"message": "Transacciu00f3n creada exitosamente", "transaction": transaction})
}

// GetUserTransactions obtiene todas las transacciones del usuario con detalles adicionales
func GetUserTransactions(c *gin.Context) {
	// Obtener el ID del usuario del contexto
	userID, exists := c.Get("userId")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Usuario no autenticado"})
		return
	}

	// Convertir el ID a string
	userIDStr := userID.(string)

	// Obtener transacciones con detalles
	transactions, err := repository.GetUserTransactionsWithDetails(userIDStr)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, transactions)
}

// GetTransactionDetails obtiene los detalles de una transacciu00f3n especu00edfica
func GetTransactionDetails(c *gin.Context) {
	// Obtener el ID del usuario del contexto
	userID, exists := c.Get("userId")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Usuario no autenticado"})
		return
	}

	// Convertir el ID a string
	userIDStr := userID.(string)

	// El ID de la transacciu00f3n se obtiene directamente del paru00e1metro de la URL
	transactionID := c.Param("id")

	// Obtener detalles de la transacciu00f3n
	transaction, err := repository.GetTransactionWithDetails(userIDStr, transactionID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, transaction)
}

// UpdateTransaction actualiza una transacciu00f3n existente
func UpdateTransaction(c *gin.Context) {
	// Obtener el ID del usuario del contexto
	userID, exists := c.Get("userId")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Usuario no autenticado"})
		return
	}

	// Convertir el ID a string
	userIDStr := userID.(string)

	// El ID de la transacciu00f3n se obtiene directamente del paru00e1metro de la URL
	transactionID := c.Param("id")

	// Verificar que la transacciu00f3n pertenezca al usuario
	transaction, err := repository.GetTransaction(transactionID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	if transaction.UserID != userIDStr {
		c.JSON(http.StatusForbidden, gin.H{"error": "No tienes permiso para modificar esta transacciu00f3n"})
		return
	}

	// Obtener datos actualizados
	var updatedTransaction models.CryptoTransaction
	if err := c.ShouldBindJSON(&updatedTransaction); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Actualizar la transacciu00f3n
	updatedTransaction.ID = transactionID
	updatedTransaction.UserID = userIDStr
	if err := repository.UpdateTransaction(&updatedTransaction); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// Crear snapshot automu00e1tico (versiu00f3n simplificada)
	// TODO: Implementar la creaciu00f3n real del snapshot
	log.Printf("Creando snapshot para usuario %s", userIDStr)

	c.JSON(http.StatusOK, gin.H{"message": "Transacciu00f3n actualizada exitosamente", "transaction": updatedTransaction})
}

// DeleteTransaction elimina una transacciu00f3n existente
func DeleteTransaction(c *gin.Context) {
	// Obtener el ID del usuario del contexto
	userID, exists := c.Get("userId")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Usuario no autenticado"})
		return
	}

	// Convertir el ID a string
	userIDStr := userID.(string)

	// El ID de la transacciu00f3n se obtiene directamente del paru00e1metro de la URL
	transactionID := c.Param("id")

	// Verificar que la transacciu00f3n pertenezca al usuario
	transaction, err := repository.GetTransaction(transactionID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	if transaction.UserID != userIDStr {
		c.JSON(http.StatusForbidden, gin.H{"error": "No tienes permiso para eliminar esta transacciu00f3n"})
		return
	}

	// Eliminar la transacciu00f3n
	if err := repository.DeleteTransaction(userIDStr, transactionID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// Crear snapshot automu00e1tico (versiu00f3n simplificada)
	// TODO: Implementar la creaciu00f3n real del snapshot
	log.Printf("Creando snapshot para usuario %s", userIDStr)

	c.JSON(http.StatusOK, gin.H{"message": "Transacciu00f3n eliminada exitosamente"})
}

// DeleteTransactionsByTicker elimina todas las transacciones de un ticker especu00edfico
func DeleteTransactionsByTicker(c *gin.Context) {
	// Obtener el ID del usuario del contexto
	userID, exists := c.Get("userId")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Usuario no autenticado"})
		return
	}

	// Convertir el ID a string
	userIDStr := userID.(string)

	// Obtener ticker de la URL
	ticker := c.Param("ticker")

	// Eliminar todas las transacciones del ticker
	if err := repository.DeleteTransactionsByTicker(userIDStr, ticker); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// Crear snapshot automu00e1tico (versiu00f3n simplificada)
	// TODO: Implementar la creaciu00f3n real del snapshot
	log.Printf("Creando snapshot para usuario %s", userIDStr)

	c.JSON(http.StatusOK, gin.H{"message": "Todas las transacciones de " + ticker + " han sido eliminadas"})
}

// GetRecentTransactions obtiene las transacciones mu00e1s recientes del usuario
func GetRecentTransactions(c *gin.Context) {
	// Obtener el ID del usuario del contexto
	userID, exists := c.Get("userId")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Usuario no autenticado"})
		return
	}

	// Convertir el ID a string
	userIDStr := userID.(string)

	// Obtener lu00edmite de la URL o usar valor predeterminado
	limitStr := c.DefaultQuery("limit", "5")
	limit, err := strconv.Atoi(limitStr)
	if err != nil || limit <= 0 {
		limit = 5 // Valor predeterminado
	}

	// Obtener transacciones recientes
	transactions, err := repository.GetRecentTransactions(userIDStr, limit)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, transactions)
}
