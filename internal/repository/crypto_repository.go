package repository

import (
	"database/sql"
	"errors"
	"fmt"
	"log"

	"github.com/AgusMolinaCode/DCA_Api.git/internal/models"
	"github.com/AgusMolinaCode/DCA_Api.git/internal/services"
)

// CryptoRepository maneja las operaciones de base de datos para criptomonedas
type CryptoRepository struct {
	db           *sql.DB
	holdingsRepo *HoldingsRepository
}

// NewCryptoRepository crea un nuevo repositorio de criptomonedas
func NewCryptoRepository(db *sql.DB) *CryptoRepository {
	return &CryptoRepository{
		db:           db,
		holdingsRepo: NewHoldingsRepository(db),
	}
}

// CreateTransaction crea una nueva transacción de criptomoneda
func (r *CryptoRepository) CreateTransaction(transaction models.CryptoTransaction) error {
	// Iniciar una transacción SQL
	tx, err := r.db.Begin()
	if err != nil {
		return err
	}
	defer func() {
		if err != nil {
			tx.Rollback()
			return
		}
		err = tx.Commit()
	}()

	// Insertar la transacción
	query := `
		INSERT INTO crypto_transactions (
			id, user_id, crypto_name, ticker, amount, purchase_price, 
			total, date, note, created_at, type, usdt_received
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`

	_, err = tx.Exec(
		query,
		transaction.ID,
		transaction.UserID,
		transaction.CryptoName,
		transaction.Ticker,
		transaction.Amount,
		transaction.PurchasePrice,
		transaction.Total,
		transaction.Date,
		transaction.Note,
		transaction.CreatedAt,
		transaction.Type,
		transaction.USDTReceived,
	)
	if err != nil {
		return err
	}

	// Si es una venta, verificar si hay suficiente criptomoneda para vender
	if transaction.Type == models.TransactionTypeSell {
		err = r.holdingsRepo.UpdateHoldingsAfterSale(tx, transaction.UserID, transaction.Ticker, transaction.Amount)
		if err != nil {
			return err
		}

		// Si se recibió USDT, crear una transacción de compra de USDT
		if transaction.USDTReceived > 0 {
			// Crear una nueva transacción para la compra de USDT
			usdtTransaction := models.CryptoTransaction{
				ID:            fmt.Sprintf("%s-usdt", transaction.ID),
				UserID:        transaction.UserID,
				CryptoName:    "Tether",
				Ticker:        "USDT",
				Amount:        transaction.USDTReceived,
				PurchasePrice: 1.0, // USDT está anclado a 1 USD
				Total:         transaction.USDTReceived,
				Date:          transaction.Date,
				Note:          fmt.Sprintf("Compra automática de USDT por venta de %s", transaction.Ticker),
				CreatedAt:     transaction.CreatedAt,
				Type:          models.TransactionTypeBuy,
				USDTReceived:  0,
			}

			// Insertar la transacción de USDT
			_, err = tx.Exec(
				query,
				usdtTransaction.ID,
				usdtTransaction.UserID,
				usdtTransaction.CryptoName,
				usdtTransaction.Ticker,
				usdtTransaction.Amount,
				usdtTransaction.PurchasePrice,
				usdtTransaction.Total,
				usdtTransaction.Date,
				usdtTransaction.Note,
				usdtTransaction.CreatedAt,
				usdtTransaction.Type,
				usdtTransaction.USDTReceived,
			)
			if err != nil {
				// Loguear el error pero no detener el flujo
				log.Printf("Error al crear transacción de USDT: %v", err)
			}
		}
	}

	return nil
}

// UpdateTransaction actualiza una transacción existente
func (r *CryptoRepository) UpdateTransaction(transaction models.CryptoTransaction) error {
	// Verificar que la transacción pertenezca al usuario
	var count int
	err := r.db.QueryRow("SELECT COUNT(*) FROM crypto_transactions WHERE id = ? AND user_id = ?",
		transaction.ID, transaction.UserID).Scan(&count)
	if err != nil {
		return err
	}
	if count == 0 {
		return errors.New("transacción no encontrada o no tienes permiso para modificarla")
	}

	// Actualizar la transacción
	query := `
		UPDATE crypto_transactions
		SET crypto_name = ?, ticker = ?, amount = ?, purchase_price = ?, 
			total = ?, date = ?, note = ?, type = ?, usdt_received = ?
		WHERE id = ? AND user_id = ?
	`

	_, err = r.db.Exec(
		query,
		transaction.CryptoName,
		transaction.Ticker,
		transaction.Amount,
		transaction.PurchasePrice,
		transaction.Total,
		transaction.Date,
		transaction.Note,
		transaction.Type,
		transaction.USDTReceived,
		transaction.ID,
		transaction.UserID,
	)
	return err
}

// DeleteTransaction elimina una transacción
func (r *CryptoRepository) DeleteTransaction(userID, transactionID string) error {
	// Verificar que la transacción pertenezca al usuario
	var count int
	err := r.db.QueryRow("SELECT COUNT(*) FROM crypto_transactions WHERE id = ? AND user_id = ?",
		transactionID, userID).Scan(&count)
	if err != nil {
		return err
	}
	if count == 0 {
		return errors.New("transacción no encontrada o no tienes permiso para eliminarla")
	}

	// Eliminar la transacción
	_, err = r.db.Exec("DELETE FROM crypto_transactions WHERE id = ? AND user_id = ?",
		transactionID, userID)
	return err
}

func (r *CryptoRepository) GetUserTransactions(userID string) ([]models.CryptoTransaction, error) {
	query := `
		SELECT id, user_id, crypto_name, ticker, amount, purchase_price, total, date, note, created_at, type, usdt_received
		FROM crypto_transactions 
		WHERE user_id = ?
		ORDER BY date DESC`

	rows, err := r.db.Query(query, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var transactions []models.CryptoTransaction
	for rows.Next() {
		var tx models.CryptoTransaction
		err := rows.Scan(
			&tx.ID,
			&tx.UserID,
			&tx.CryptoName,
			&tx.Ticker,
			&tx.Amount,
			&tx.PurchasePrice,
			&tx.Total,
			&tx.Date,
			&tx.Note,
			&tx.CreatedAt,
			&tx.Type,
			&tx.USDTReceived,
		)
		if err != nil {
			return nil, err
		}
		transactions = append(transactions, tx)
	}
	return transactions, nil
}

func (r *CryptoRepository) GetCryptoDashboard(userID string) ([]models.CryptoDashboard, error) {
	// Obtener todas las transacciones del usuario
	query := `
		SELECT ticker, crypto_name, amount, purchase_price, total, type
		FROM crypto_transactions
		WHERE user_id = ?
		ORDER BY date DESC`

	rows, err := r.db.Query(query, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	// Mapa para acumular datos por criptomoneda
	cryptoMap := make(map[string]*models.CryptoDashboard)

	// Procesar cada transacción
	for rows.Next() {
		var ticker, cryptoName, txType string
		var amount, purchasePrice, total float64

		err := rows.Scan(&ticker, &cryptoName, &amount, &purchasePrice, &total, &txType)
		if err != nil {
			return nil, err
		}

		// Si es USDT, tratarlo de manera especial
		if ticker == "USDT" {
			// Inicializar si no existe
			if _, exists := cryptoMap[ticker]; !exists {
				cryptoMap[ticker] = &models.CryptoDashboard{
					Ticker:        ticker,
					TotalInvested: 0,
					Holdings:      0,
					AvgPrice:      1.0,
					CurrentPrice:  1.0,
				}
			}

			// Actualizar tenencias
			if txType == models.TransactionTypeBuy {
				cryptoMap[ticker].Holdings += amount
				cryptoMap[ticker].TotalInvested += amount
			} else if txType == models.TransactionTypeSell {
				cryptoMap[ticker].Holdings -= amount
			}

			// Para USDT, siempre mantener TotalInvested = Holdings
			cryptoMap[ticker].TotalInvested = cryptoMap[ticker].Holdings
			cryptoMap[ticker].CurrentProfit = 0
			cryptoMap[ticker].ProfitPercent = 0
			continue
		}

		// Para otras criptomonedas
		if _, exists := cryptoMap[ticker]; !exists {
			cryptoMap[ticker] = &models.CryptoDashboard{
				Ticker:        ticker,
				TotalInvested: 0,
				Holdings:      0,
			}
		}

		// Actualizar tenencias según el tipo de transacción
		if txType == models.TransactionTypeBuy {
			cryptoMap[ticker].Holdings += amount
			// Si el precio de compra es 0, intentar obtener el precio actual
			if purchasePrice <= 0 {
				// Obtener precio actual para calcular el total
				cryptoData, err := services.GetCryptoPrice(ticker)
				if err == nil && cryptoData != nil {
					purchasePrice = cryptoData.Raw[ticker]["USD"].PRICE
					total = amount * purchasePrice
				} else {
					// Si no se puede obtener el precio, usar un valor predeterminado
					purchasePrice = 1.0
					total = amount
				}
			}
			cryptoMap[ticker].TotalInvested += total
		} else if txType == models.TransactionTypeSell {
			cryptoMap[ticker].Holdings -= amount
		}
	}

	// Convertir el mapa a un slice
	dashboard := make([]models.CryptoDashboard, 0, len(cryptoMap))
	for _, crypto := range cryptoMap {
		// Solo incluir criptomonedas con tenencias positivas
		if crypto.Holdings > 0 {
			// Calcular precio promedio
			if crypto.Holdings > 0 && crypto.TotalInvested > 0 {
				crypto.AvgPrice = crypto.TotalInvested / crypto.Holdings
			}

			// Obtener precio actual
			if crypto.Ticker != "USDT" {
				cryptoData, err := services.GetCryptoPrice(crypto.Ticker)
				if err == nil {
					crypto.CurrentPrice = cryptoData.Raw[crypto.Ticker]["USD"].PRICE
					crypto.CurrentProfit = (crypto.CurrentPrice - crypto.AvgPrice) * crypto.Holdings
					if crypto.TotalInvested > 0 {
						crypto.ProfitPercent = (crypto.CurrentProfit / crypto.TotalInvested) * 100
					}
				}
			}

			dashboard = append(dashboard, *crypto)
		}
	}

	return dashboard, nil
}

func (r *CryptoRepository) GetTransactionDetails(userID string, transactionID string) (*models.TransactionDetails, error) {
	query := `
		SELECT id, user_id, crypto_name, ticker, amount, purchase_price, total, date, note, created_at, type, usdt_received
		FROM crypto_transactions 
		WHERE user_id = ? AND id = ?`

	var tx models.CryptoTransaction
	err := r.db.QueryRow(query, userID, transactionID).Scan(
		&tx.ID,
		&tx.UserID,
		&tx.CryptoName,
		&tx.Ticker,
		&tx.Amount,
		&tx.PurchasePrice,
		&tx.Total,
		&tx.Date,
		&tx.Note,
		&tx.CreatedAt,
		&tx.Type,
		&tx.USDTReceived,
	)
	if err != nil {
		return nil, err
	}

	// Obtener precio actual
	cryptoData, err := services.GetCryptoPrice(tx.Ticker)
	if err != nil {
		return nil, err
	}

	details := &models.TransactionDetails{
		Transaction: tx,
	}

	if cryptoData.Raw[tx.Ticker]["USD"].PRICE > 0 {
		details.CurrentPrice = cryptoData.Raw[tx.Ticker]["USD"].PRICE
		details.CurrentValue = tx.Amount * details.CurrentPrice
		details.GainLoss = details.CurrentValue - tx.Total
		details.GainLossPercent = (details.GainLoss / tx.Total) * 100
	}

	return details, nil
}

func (r *CryptoRepository) GetRecentTransactions(userID string, limit int) ([]models.TransactionDetails, error) {
	query := `
		SELECT id, user_id, crypto_name, ticker, amount, purchase_price, total, date, note, created_at, type, usdt_received
		FROM crypto_transactions
		WHERE user_id = ?
		ORDER BY date DESC
		LIMIT ?`

	rows, err := r.db.Query(query, userID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var transactions []models.TransactionDetails

	for rows.Next() {
		var tx models.CryptoTransaction
		err := rows.Scan(
			&tx.ID,
			&tx.UserID,
			&tx.CryptoName,
			&tx.Ticker,
			&tx.Amount,
			&tx.PurchasePrice,
			&tx.Total,
			&tx.Date,
			&tx.Note,
			&tx.CreatedAt,
			&tx.Type,
			&tx.USDTReceived,
		)
		if err != nil {
			return nil, err
		}

		// Crear detalles de la transacción
		details := models.TransactionDetails{
			Transaction: tx,
		}

		// Si es USDT, el precio actual es siempre 1
		if tx.Ticker == "USDT" {
			details.CurrentPrice = 1.0
			details.CurrentValue = tx.Amount
			details.GainLoss = 0.0
			details.GainLossPercent = 0.0
		} else {
			// Obtener precio actual para otras criptomonedas
			cryptoData, err := services.GetCryptoPrice(tx.Ticker)
			if err == nil {
				details.CurrentPrice = cryptoData.Raw[tx.Ticker]["USD"].PRICE
				details.CurrentValue = tx.Amount * details.CurrentPrice

				// Calcular ganancia/pérdida solo para compras
				if tx.Type == models.TransactionTypeBuy {
					details.GainLoss = details.CurrentValue - tx.Total
					if tx.Total > 0 {
						details.GainLossPercent = (details.GainLoss / tx.Total) * 100
					}
				}
			}
		}

		transactions = append(transactions, details)
	}

	// Si no hay transacciones, devolver un slice vacío en lugar de error
	if len(transactions) == 0 {
		return []models.TransactionDetails{}, nil
	}

	return transactions, nil
}


