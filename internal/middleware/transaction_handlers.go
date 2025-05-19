package middleware

import (
	"github.com/gin-gonic/gin"
	"github.com/AgusMolinaCode/DCA_Api.git/internal/models"
	"github.com/AgusMolinaCode/DCA_Api.git/internal/repository"
	"log"
	"net/http"
	"strconv"
)

// CreateTransaction crea una nueva transacción para el usuario autenticado
func CreateTransaction(c *gin.Context) {
	var transaction models.CryptoTransaction
	if err := c.ShouldBindJSON(&transaction); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Validar que el usuario esté autenticado
	user, exists := c.Get("user")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Usuario no autenticado"})
		return
	}

	// Convertir el usuario a modelo
	userModel := user.(*models.User)
	transaction.UserID = userModel.ID

	// Validar que el ticker exista
	if !repository.CryptoExists(transaction.Ticker) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Criptomoneda no encontrada"})
		return
	}

	// Crear la transacción
	if err := repository.CreateTransaction(&transaction); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// Actualizar el balance del usuario
	if err := repository.UpdateUserBalance(userModel.ID, transaction.Amount*transaction.PurchasePrice); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Error al actualizar balance"})
		return
	}

	// Crear snapshot automático (versión simplificada)
	// TODO: Implementar la creación real del snapshot
	log.Printf("Creando snapshot para usuario %s", userModel.ID)

	c.JSON(http.StatusCreated, gin.H{"message": "Transacción creada exitosamente", "transaction": transaction})
}

// GetUserTransactions obtiene todas las transacciones del usuario con detalles adicionales
func GetUserTransactions(c *gin.Context) {
	// Validar que el usuario esté autenticado
	user, exists := c.Get("user")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Usuario no autenticado"})
		return
	}

	// Convertir el usuario a modelo
	userModel := user.(*models.User)

	// Obtener transacciones con detalles
	transactions, err := repository.GetUserTransactionsWithDetails(userModel.ID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, transactions)
}

// GetTransactionDetails obtiene los detalles de una transacción específica
func GetTransactionDetails(c *gin.Context) {
	// Validar que el usuario esté autenticado
	user, exists := c.Get("user")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Usuario no autenticado"})
		return
	}

	// Convertir el usuario a modelo
	userModel := user.(*models.User)

	// El ID de la transacción se obtiene directamente del parámetro de la URL
	transactionID := c.Param("id")

	// Obtener detalles de la transacción
	transaction, err := repository.GetTransactionWithDetails(userModel.ID, transactionID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, transaction)
}

// UpdateTransaction actualiza una transacción existente
func UpdateTransaction(c *gin.Context) {
	// Validar que el usuario esté autenticado
	user, exists := c.Get("user")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Usuario no autenticado."})
		return
	}

	// Convertir el usuario a modelo
	userModel := user.(*models.User)

	// El ID de la transacción se obtiene directamente del parámetro de la URL
	transactionID := c.Param("id")

	// Verificar que la transacción pertenezca al usuario
	transaction, err := repository.GetTransaction(transactionID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	if transaction.UserID != userModel.ID {
		c.JSON(http.StatusForbidden, gin.H{"error": "No tienes permiso para modificar esta transacción"})
		return
	}

	// Obtener datos actualizados
	var updatedTransaction models.CryptoTransaction
	if err := c.ShouldBindJSON(&updatedTransaction); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Actualizar la transacción
	updatedTransaction.ID = transactionID
	updatedTransaction.UserID = userModel.ID
	if err := repository.UpdateTransaction(&updatedTransaction); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// Crear snapshot automático (versión simplificada)
	// TODO: Implementar la creación real del snapshot
	log.Printf("Creando snapshot para usuario %s", userModel.ID)

	c.JSON(http.StatusOK, gin.H{"message": "Transacción actualizada exitosamente", "transaction": updatedTransaction})
}

// DeleteTransaction elimina una transacción existente
func DeleteTransaction(c *gin.Context) {
	// Validar que el usuario esté autenticado
	user, exists := c.Get("user")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Usuario no autenticado"})
		return
	}

	// Convertir el usuario a modelo
	userModel := user.(*models.User)

	// El ID de la transacción se obtiene directamente del parámetro de la URL
	transactionID := c.Param("id")

	// Verificar que la transacción pertenezca al usuario
	transaction, err := repository.GetTransaction(transactionID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	if transaction.UserID != userModel.ID {
		c.JSON(http.StatusForbidden, gin.H{"error": "No tienes permiso para eliminar esta transacción"})
		return
	}

	// Eliminar la transacción
	if err := repository.DeleteTransaction(userModel.ID, transactionID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// Crear snapshot automático (versión simplificada)
	// TODO: Implementar la creación real del snapshot
	log.Printf("Creando snapshot para usuario %s", userModel.ID)

	c.JSON(http.StatusOK, gin.H{"message": "Transacción eliminada exitosamente"})
}

// DeleteTransactionsByTicker elimina todas las transacciones de un ticker específico
func DeleteTransactionsByTicker(c *gin.Context) {
	// Validar que el usuario esté autenticado
	user, exists := c.Get("user")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Usuario no autenticado"})
		return
	}

	// Convertir el usuario a modelo
	userModel := user.(*models.User)

	// Obtener ticker de la URL
	ticker := c.Param("ticker")

	// Eliminar todas las transacciones del ticker
	if err := repository.DeleteTransactionsByTicker(userModel.ID, ticker); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// Crear snapshot automático (versión simplificada)
	// TODO: Implementar la creación real del snapshot
	log.Printf("Creando snapshot para usuario %s", userModel.ID)

	c.JSON(http.StatusOK, gin.H{"message": "Todas las transacciones de " + ticker + " han sido eliminadas"})
}

// GetRecentTransactions obtiene las transacciones más recientes del usuario
func GetRecentTransactions(c *gin.Context) {
	// Validar que el usuario esté autenticado
	user, exists := c.Get("user")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Usuario no autenticado"})
		return
	}

	// Convertir el usuario a modelo
	userModel := user.(*models.User)

	// Obtener límite de la URL o usar valor predeterminado
	limitStr := c.DefaultQuery("limit", "5")
	limit, err := strconv.Atoi(limitStr)
	if err != nil || limit <= 0 {
		limit = 5 // Valor predeterminado
	}

	// Obtener transacciones recientes
	transactions, err := repository.GetRecentTransactions(userModel.ID, limit)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, transactions)
}
