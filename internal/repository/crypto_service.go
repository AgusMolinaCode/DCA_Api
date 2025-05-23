package repository

import (
	"database/sql"
	"errors"
	"github.com/AgusMolinaCode/DCA_Api.git/internal/models"
	"github.com/AgusMolinaCode/DCA_Api.git/internal/services"
)

// Variables globales para mantener instancias de los repositorios
var (
	dbInstance *sql.DB
	cryptoRepo *CryptoRepository
)

// InitRepositories inicializa los repositorios con la conexión a la base de datos
func InitRepositories(db *sql.DB) {
	dbInstance = db
	cryptoRepo = NewCryptoRepository(db)
}

// CryptoExists verifica si una criptomoneda existe consultando su precio actual
func CryptoExists(ticker string) bool {
	// Utilizar el servicio GetCryptoPrice para verificar si la criptomoneda existe
	cryptoData, err := services.GetCryptoPrice(ticker)
	
	// Si hay un error o no se obtienen datos, la criptomoneda no existe
	if err != nil || cryptoData == nil {
		return false
	}
	
	// Verificar si el ticker existe en los datos obtenidos
	_, exists := cryptoData.Raw[ticker]
	return exists
}

// CreateTransaction crea una nueva transacción de criptomoneda
func CreateTransaction(transaction *models.CryptoTransaction) error {
	if cryptoRepo == nil {
		return ErrRepositoryNotInitialized
	}
	return cryptoRepo.CreateTransaction(*transaction)
}

// UpdateUserBalance actualiza el balance del usuario
func UpdateUserBalance(userID string, amount float64) error {
	// Esta función debería implementarse según la lógica de tu aplicación
	// Por ahora, simplemente registramos la operación y no hacemos nada
	return nil
}

// GetUserTransactionsWithDetails obtiene todas las transacciones del usuario con detalles adicionales
func GetUserTransactionsWithDetails(userID string) ([]models.TransactionDetails, error) {
	if cryptoRepo == nil {
		return nil, ErrRepositoryNotInitialized
	}
	return cryptoRepo.GetUserTransactionsWithDetails(userID)
}

// GetTransactionWithDetails obtiene una transacción específica con detalles adicionales
func GetTransactionWithDetails(userID string, transactionID string) (*models.TransactionDetails, error) {
	if cryptoRepo == nil {
		return nil, ErrRepositoryNotInitialized
	}
	return cryptoRepo.GetTransactionDetails(userID, transactionID)
}

// GetTransaction obtiene una transacción por su ID
func GetTransaction(transactionID string) (*models.CryptoTransaction, error) {
	if cryptoRepo == nil {
		return nil, ErrRepositoryNotInitialized
	}
	return cryptoRepo.GetTransaction(transactionID)
}

// UpdateTransaction actualiza una transacción existente
func UpdateTransaction(transaction *models.CryptoTransaction) error {
	if cryptoRepo == nil {
		return ErrRepositoryNotInitialized
	}
	return cryptoRepo.UpdateTransaction(*transaction)
}

// DeleteTransaction elimina una transacción
func DeleteTransaction(userID, transactionID string) error {
	if cryptoRepo == nil {
		return ErrRepositoryNotInitialized
	}
	return cryptoRepo.DeleteTransaction(userID, transactionID)
}

// DeleteTransactionsByTicker elimina todas las transacciones de una criptomoneda específica para un usuario
func DeleteTransactionsByTicker(userID, ticker string) error {
	if cryptoRepo == nil {
		return ErrRepositoryNotInitialized
	}
	return cryptoRepo.DeleteTransactionsByTicker(userID, ticker)
}

// GetRecentTransactions obtiene las transacciones más recientes de un usuario
func GetRecentTransactions(userID string, limit int) ([]models.TransactionDetails, error) {
	if cryptoRepo == nil {
		return nil, ErrRepositoryNotInitialized
	}
	return cryptoRepo.GetRecentTransactions(userID, limit)
}

// Errores comunes
var (
	ErrRepositoryNotInitialized = errors.New("el repositorio no ha sido inicializado")
	ErrNotImplemented = errors.New("función no implementada")
)
