package repository

import (
	"database/sql"
	"errors"
	"fmt"
	"log"
	"sort"
	"time"

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

// Función para generar un ID único para transacciones
func generateTransactionId() string {
	return fmt.Sprintf("%d", time.Now().UnixNano())
}

// CreateTransaction crea una nueva transacción de criptomoneda
func (r *CryptoRepository) CreateTransaction(transaction models.CryptoTransaction) error {
	// Iniciar transacción SQL
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

	// Generar ID único para la transacción
	transaction.ID = generateTransactionId()

	// Si es una venta, verificar si el usuario tiene suficiente saldo
	if transaction.Type == models.TransactionTypeSell {
		err = r.holdingsRepo.UpdateHoldingsAfterSale(tx, transaction.UserID, transaction.Ticker, transaction.Amount)
		if err != nil {
			return err
		}
	}

	// Insertar la transacción en la base de datos
	query := `
		INSERT INTO crypto_transactions (
			id, user_id, crypto_name, ticker, amount, purchase_price, 
			total, date, note, created_at, type, usdt_received, image_url
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`

	// Si la fecha está vacía, usar la fecha actual
	if transaction.Date.IsZero() {
		transaction.Date = time.Now()
	}

	// Si el tipo está vacío, establecer como compra por defecto
	if transaction.Type == "" {
		transaction.Type = models.TransactionTypeBuy
	}

	// Si no se especificó el precio, obtener precio actual
	if transaction.PurchasePrice <= 0 {
		cryptoData, err := services.GetCryptoPrice(transaction.Ticker)
		if err != nil {
			return fmt.Errorf("error al obtener precio de %s: %v", transaction.Ticker, err)
		}
		// Usar el precio actual de la API
		transaction.PurchasePrice = cryptoData.Raw[transaction.Ticker]["USD"].PRICE
	}

	// Calcular el total si no se especificó
	if transaction.Total <= 0 {
		transaction.Total = transaction.Amount * transaction.PurchasePrice
	}

	// Establecer la fecha de creación
	transaction.CreatedAt = time.Now()

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
		transaction.ImageURL,
	)

	if err != nil {
		return err
	}

	// Si es una venta y se recibió USDT, crear automáticamente una transacción de compra de USDT
	if transaction.Type == models.TransactionTypeSell && transaction.USDTReceived > 0 {
		usdtTransaction := models.CryptoTransaction{
			UserID:        transaction.UserID,
			CryptoName:    "Tether",
			Ticker:        "USDT",
			Amount:        transaction.USDTReceived,
			PurchasePrice: 1.0, // USDT está anclado a 1 USD
			Total:         transaction.USDTReceived,
			Date:          transaction.Date,
			Note:          fmt.Sprintf("Compra automática de USDT por venta de %s", transaction.Ticker),
			Type:          models.TransactionTypeBuy,
		}

		// Generar ID único para la transacción de USDT
		usdtTransaction.ID = generateTransactionId()
		usdtTransaction.CreatedAt = time.Now()

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
			0,  // No hay USDT recibido en una compra
			"", // No hay imagen URL para la transacción automática
		)

		if err != nil {
			// Loguear el error pero no interrumpir el flujo principal
			log.Printf("Error al crear transacción automática de USDT: %v", err)
		}
	}

	return nil
}

// UpdateTransaction actualiza una transacción existente
func (r *CryptoRepository) UpdateTransaction(transaction models.CryptoTransaction) error {
	// Verificar que la transacción exista y pertenezca al usuario
	var existingUserId string
	err := r.db.QueryRow("SELECT user_id FROM crypto_transactions WHERE id = ?", transaction.ID).Scan(&existingUserId)
	if err != nil {
		if err == sql.ErrNoRows {
			return fmt.Errorf("transacción no encontrada")
		}
		return err
	}

	if existingUserId != transaction.UserID {
		return fmt.Errorf("no tienes permiso para modificar esta transacción")
	}

	// Iniciar transacción SQL
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

	// Actualizar la transacción
	query := `
		UPDATE crypto_transactions 
		SET crypto_name = ?, ticker = ?, amount = ?, purchase_price = ?, 
			total = ?, date = ?, note = ?, type = ?, usdt_received = ?, image_url = ?
		WHERE id = ? AND user_id = ?
	`

	// Calcular el total si no se especificó
	if transaction.Total <= 0 {
		transaction.Total = transaction.Amount * transaction.PurchasePrice
	}

	_, err = tx.Exec(
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
		transaction.ImageURL,
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

// DeleteTransactionsByTicker elimina todas las transacciones de una criptomoneda específica para un usuario
func (r *CryptoRepository) DeleteTransactionsByTicker(userID, ticker string) error {
	// Verificar que el ticker exista para el usuario
	checkQuery := `SELECT COUNT(*) FROM crypto_transactions WHERE user_id = ? AND ticker = ?`
	var count int
	err := r.db.QueryRow(checkQuery, userID, ticker).Scan(&count)
	if err != nil {
		return err
	}

	if count == 0 {
		return fmt.Errorf("No se encontraron transacciones con el ticker %s", ticker)
	}

	// Iniciar una transacción para asegurar que todas las operaciones se completen o ninguna
	tx, err := r.db.Begin()
	if err != nil {
		return err
	}
	defer func() {
		if err != nil {
			tx.Rollback()
		}
	}()

	// Eliminar todas las transacciones con ese ticker
	deleteQuery := `DELETE FROM crypto_transactions WHERE user_id = ? AND ticker = ?`
	result, err := tx.Exec(deleteQuery, userID, ticker)
	if err != nil {
		return err
	}

	// Verificar cuántas filas se eliminaron
	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return err
	}

	// Si no se eliminó ninguna fila, devolver un error
	if rowsAffected == 0 {
		return fmt.Errorf("no se encontraron transacciones con el ticker %s", ticker)
	}

	// Confirmar la transacción
	if err := tx.Commit(); err != nil {
		return err
	}

	return nil
}

func (r *CryptoRepository) GetUserTransactionsWithDetails(userID string) ([]models.TransactionDetails, error) {
	query := `
		SELECT id, user_id, crypto_name, ticker, amount, purchase_price, 
			   total, date, note, created_at, type, usdt_received, image_url
		FROM crypto_transactions 
		WHERE user_id = ? 
		ORDER BY date DESC
	`

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
			&tx.ImageURL,
		)
		if err != nil {
			return nil, err
		}
		transactions = append(transactions, tx)
	}

	if err = rows.Err(); err != nil {
		return nil, err
	}

	var details []models.TransactionDetails
	for _, tx := range transactions {
		// Crear el objeto de detalles con la transacción base
		detail := models.TransactionDetails{
			Transaction: tx,
		}

		// Obtener el precio actual de la criptomoneda
		cryptoData, err := services.GetCryptoPrice(tx.Ticker)
		if err == nil && cryptoData.Raw[tx.Ticker]["USD"].PRICE > 0 {
			// Si se obtiene el precio actual correctamente
			currentPrice := cryptoData.Raw[tx.Ticker]["USD"].PRICE

			// Calcular ganancia/pérdida según el tipo de transacción
			if tx.Type == models.TransactionTypeBuy {
				// Para compras:
				// Precio: precio de compra
				// Precio actual: obtenido de la API
				detail.CurrentPrice = currentPrice

				// Asegurarse de que tx.Total tenga el valor correcto (precio * cantidad)
				if tx.Total <= 0 {
					tx.Total = tx.Amount * tx.PurchasePrice
				}

				// Valor actual: precio actual * cantidad
				detail.CurrentValue = tx.Amount * currentPrice

				// Ganancia/pérdida: valor actual - total
				detail.GainLoss = detail.CurrentValue - tx.Total

				// Porcentaje de ganancia/pérdida
				if tx.Total > 0 {
					detail.GainLossPercent = (detail.GainLoss / tx.Total) * 100
				}
			} else if tx.Type == models.TransactionTypeSell {
				// Para ventas: necesitamos obtener el precio promedio de compra para calcular la ganancia/pérdida
				// Obtener el precio promedio de compra de la criptomoneda
				avgPrice, err := r.getAveragePurchasePrice(tx.UserID, tx.Ticker, tx.Date)
				if err != nil || avgPrice <= 0 {
					// Si hay un error o el precio promedio es 0, usar el precio de compra de la transacción
					avgPrice = tx.PurchasePrice
				}
				// Calcular el costo base usando el precio promedio
				costBasis := avgPrice * tx.Amount

				// Asegurarse de que el total sea correcto (lo que se recibió por la venta)
				if tx.USDTReceived > 0 {
					tx.Total = tx.USDTReceived
				} else if tx.Total <= 0 {
					tx.Total = tx.Amount * tx.PurchasePrice
				}

				// El precio actual para mostrar
				detail.CurrentPrice = currentPrice
				// El valor actual es lo que valdría si aún tuviéramos la criptomoneda
				detail.CurrentValue = tx.Amount * currentPrice

				// La ganancia/pérdida es lo que se recibió menos lo que costó
				detail.GainLoss = tx.Total - costBasis

				// Calcular el porcentaje de ganancia/pérdida
				if costBasis > 0 {
					detail.GainLossPercent = (detail.GainLoss / costBasis) * 100
				}
			}
		} else {
			// Si hay un error, usar el precio de compra como respaldo
			detail.CurrentPrice = tx.PurchasePrice

			// Para ventas, aún podemos calcular la ganancia/pérdida
			if tx.Type == models.TransactionTypeSell {
				// Intentar obtener el precio promedio de compra
				avgPrice, err := r.getAveragePurchasePrice(tx.UserID, tx.Ticker, tx.Date)
				if err != nil || avgPrice <= 0 {
					// Si hay un error o el precio promedio es 0, usar el precio de compra de la transacción
					avgPrice = tx.PurchasePrice
				}
				costBasis := avgPrice * tx.Amount

				// Asegurarse de que el total sea correcto (lo que se recibió por la venta)
				if tx.USDTReceived > 0 {
					tx.Total = tx.USDTReceived
				} else if tx.Total <= 0 {
					tx.Total = tx.Amount * tx.PurchasePrice
				}

				// El valor actual es lo que valdría si aún tuviéramos la criptomoneda
				detail.CurrentValue = tx.Amount * tx.PurchasePrice

				// La ganancia/pérdida es lo que se recibió menos lo que costó
				detail.GainLoss = tx.Total - costBasis

				// Calcular el porcentaje de ganancia/pérdida
				if costBasis > 0 {
					detail.GainLossPercent = (detail.GainLoss / costBasis) * 100
				}
			} else {
				detail.CurrentValue = tx.Amount * tx.PurchasePrice
				detail.GainLoss = 0
				detail.GainLossPercent = 0
			}
		}

		details = append(details, detail)
	}

	return details, nil
}

func (r *CryptoRepository) GetCryptoDashboard(userID string) ([]models.CryptoDashboard, error) {
	// Obtener todas las transacciones del usuario ordenadas por fecha
	query := `
		SELECT id, ticker, crypto_name, amount, purchase_price, total, type, image_url, date, usdt_received
		FROM crypto_transactions
		WHERE user_id = ?
		ORDER BY date ASC` // Ordenamos por fecha ascendente para procesar cronológicamente

	rows, err := r.db.Query(query, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	// Mapa para acumular datos por criptomoneda
	cryptoMap := make(map[string]*models.CryptoDashboard)

	// Procesar cada transacción cronológicamente
	for rows.Next() {
		var id, ticker, cryptoName, txType string
		var amount, purchasePrice, total float64
		var imageURL sql.NullString
		var date time.Time
		var usdtReceived float64

		err := rows.Scan(&id, &ticker, &cryptoName, &amount, &purchasePrice, &total, &txType, &imageURL, &date, &usdtReceived)
		if err != nil {
			return nil, err
		}

		// Si es USDT, tratarlo de manera especial
		if ticker == "USDT" {
			// Inicializar si no existe
			if _, exists := cryptoMap[ticker]; !exists {
				cryptoMap[ticker] = &models.CryptoDashboard{
					Ticker:        ticker,
					CryptoName:    cryptoName,
					ImageURL:      imageURL.String,
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
				CryptoName:    cryptoName,
				ImageURL:      imageURL.String,
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
			// Calcular el costo promedio por unidad antes de la venta
			var costPerUnit float64
			if cryptoMap[ticker].Holdings > 0 {
				costPerUnit = cryptoMap[ticker].TotalInvested / cryptoMap[ticker].Holdings
			}

			// Calcular el costo base de las unidades vendidas
			costBasisSold := costPerUnit * amount

			// Ajustar el total invertido restando el costo base de las unidades vendidas
			cryptoMap[ticker].TotalInvested -= costBasisSold

			// Actualizar las tenencias
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
				if err == nil && cryptoData != nil {
					crypto.CurrentPrice = cryptoData.Raw[crypto.Ticker]["USD"].PRICE

					// Calcular el valor actual de las tenencias
					currentValue := crypto.CurrentPrice * crypto.Holdings

					// Calcular el profit basado en el valor actual vs total invertido
					crypto.CurrentProfit = currentValue - crypto.TotalInvested

					// Calcular el porcentaje de ganancia/pérdida
					if crypto.TotalInvested > 0 {
						crypto.ProfitPercent = (crypto.CurrentProfit / crypto.TotalInvested) * 100
					}
				} else {
					// Si no podemos obtener el precio actual, usamos el promedio como respaldo
					crypto.CurrentPrice = crypto.AvgPrice
					crypto.CurrentProfit = 0
					crypto.ProfitPercent = 0
				}
			}

			dashboard = append(dashboard, *crypto)
		}
	}

	return dashboard, nil
}

func (r *CryptoRepository) GetTransactionDetails(userID string, transactionID string) (*models.TransactionDetails, error) {
	query := `
		SELECT id, user_id, crypto_name, ticker, amount, purchase_price, 
			   total, date, note, created_at, type, usdt_received, image_url
		FROM crypto_transactions 
		WHERE id = ? AND user_id = ?
	`

	var tx models.CryptoTransaction
	err := r.db.QueryRow(query, transactionID, userID).Scan(
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
		&tx.ImageURL,
	)

	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("transacción no encontrada")
		}
		return nil, err
	}

	details := &models.TransactionDetails{
		Transaction: tx,
	}

	// Obtener el precio actual de la criptomoneda
	cryptoData, err := services.GetCryptoPrice(tx.Ticker)
	if err == nil && cryptoData.Raw[tx.Ticker]["USD"].PRICE > 0 {
		// Si se obtiene el precio actual correctamente
		currentPrice := cryptoData.Raw[tx.Ticker]["USD"].PRICE

		// Calcular ganancia/pérdida según el tipo de transacción
		if tx.Type == models.TransactionTypeBuy {
			// Para compras:
			// Precio: precio de compra
			// Precio actual: obtenido de la API
			details.CurrentPrice = currentPrice

			// Asegurarse de que tx.Total tenga el valor correcto (precio * cantidad)
			if tx.Total <= 0 {
				tx.Total = tx.Amount * tx.PurchasePrice
			}

			// Valor actual: precio actual * cantidad
			details.CurrentValue = tx.Amount * currentPrice

			// Ganancia/pérdida: valor actual - total
			details.GainLoss = details.CurrentValue - tx.Total

			// Porcentaje de ganancia/pérdida
			if tx.Total > 0 {
				details.GainLossPercent = (details.GainLoss / tx.Total) * 100
			}
		} else if tx.Type == models.TransactionTypeSell {
			// Para ventas: necesitamos obtener el precio promedio de compra para calcular la ganancia/pérdida
			// Obtener el precio promedio de compra de la criptomoneda
			avgPrice, err := r.getAveragePurchasePrice(tx.UserID, tx.Ticker, tx.Date)
			if err != nil || avgPrice <= 0 {
				// Si hay un error o el precio promedio es 0, usar el precio de compra de la transacción
				avgPrice = tx.PurchasePrice
			}
			// Calcular el costo base usando el precio promedio
			costBasis := avgPrice * tx.Amount

			// Asegurarse de que el total sea correcto (lo que se recibió por la venta)
			if tx.USDTReceived > 0 {
				tx.Total = tx.USDTReceived
			} else if tx.Total <= 0 {
				tx.Total = tx.Amount * tx.PurchasePrice
			}

			// El precio actual para mostrar
			details.CurrentPrice = currentPrice
			// El valor actual es lo que valdría si aún tuviéramos la criptomoneda
			details.CurrentValue = tx.Amount * currentPrice

			// La ganancia/pérdida es lo que se recibió menos lo que costó
			details.GainLoss = tx.Total - costBasis

			// Calcular el porcentaje de ganancia/pérdida
			if costBasis > 0 {
				details.GainLossPercent = (details.GainLoss / costBasis) * 100
			}
		}
	} else {
		// Si hay un error, usar el precio de compra como respaldo
		details.CurrentPrice = tx.PurchasePrice

		// Para ventas, aún podemos calcular la ganancia/pérdida
		if tx.Type == models.TransactionTypeSell {
			// Intentar obtener el precio promedio de compra
			avgPrice, err := r.getAveragePurchasePrice(tx.UserID, tx.Ticker, tx.Date)
			if err != nil || avgPrice <= 0 {
				// Si hay un error o el precio promedio es 0, usar el precio de compra de la transacción
				avgPrice = tx.PurchasePrice
			}
			// costBasis := avgPrice * tx.Amount

			// Asegurarse de que el total sea correcto (lo que se recibió por la venta)
			if tx.USDTReceived > 0 {
				tx.Total = tx.USDTReceived
			} else if tx.Total <= 0 {
				tx.Total = tx.Amount * tx.PurchasePrice
			}

			// El valor actual es lo que valdría si aún tuviéramos la criptomoneda
			details.CurrentValue = tx.Amount * tx.PurchasePrice

			// La ganancia/pérdida es lo que se recibió menos lo que valdría ahora
			details.GainLoss = tx.Total - details.CurrentValue

			// Calcular el porcentaje de ganancia/pérdida
			if details.CurrentValue > 0 {
				details.GainLossPercent = (details.GainLoss / details.CurrentValue) * 100
			}
		} else {
			details.CurrentValue = tx.Amount * tx.PurchasePrice
			details.GainLoss = 0
			details.GainLossPercent = 0
		}
	}

	return details, nil
}

func (r *CryptoRepository) GetRecentTransactions(userID string, limit int) ([]models.TransactionDetails, error) {
	if limit <= 0 {
		limit = 5 // Valor predeterminado
	}

	query := `
		SELECT id, user_id, crypto_name, ticker, amount, purchase_price, 
			   total, date, note, created_at, type, usdt_received, image_url
		FROM crypto_transactions 
		WHERE user_id = ? 
		ORDER BY date DESC
		LIMIT ?
	`

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
			&tx.ImageURL,
		)
		if err != nil {
			return nil, err
		}

		details := models.TransactionDetails{
			Transaction: tx,
		}

		// Obtener el precio actual
		cryptoData, err := services.GetCryptoPrice(tx.Ticker)
		if err == nil && cryptoData.Raw[tx.Ticker]["USD"].PRICE > 0 {
			currentPrice := cryptoData.Raw[tx.Ticker]["USD"].PRICE

			// Calcular ganancia/pérdida según el tipo de transacción
			if tx.Type == models.TransactionTypeBuy {
				// Para compras:
				// Precio: precio de compra
				// Precio actual: obtenido de la API
				details.CurrentPrice = currentPrice

				// Asegurarse de que tx.Total tenga el valor correcto (precio * cantidad)
				if tx.Total <= 0 {
					tx.Total = tx.Amount * tx.PurchasePrice
				}

				// Valor actual: precio actual * cantidad
				details.CurrentValue = tx.Amount * currentPrice

				// Ganancia/pérdida: valor actual - total
				details.GainLoss = details.CurrentValue - tx.Total

				// Porcentaje de ganancia/pérdida
				if tx.Total > 0 {
					details.GainLossPercent = (details.GainLoss / tx.Total) * 100
				}
			} else if tx.Type == models.TransactionTypeSell {
				// Para ventas:
				// Precio: precio de venta
				// Precio actual: obtenido de la API
				details.CurrentPrice = currentPrice

				// Calcular el costo base usando el precio promedio
				avgPrice, err := r.getAveragePurchasePrice(tx.UserID, tx.Ticker, tx.Date)
				if err != nil || avgPrice <= 0 {
					// Si hay un error o el precio promedio es 0, usar el precio de compra de la transacción
					avgPrice = tx.PurchasePrice
				}

				// costBasis := avgPrice * tx.Amount

				// Asegurarse de que el total sea correcto (lo que se recibió por la venta)
				if tx.USDTReceived > 0 {
					tx.Total = tx.USDTReceived
				} else if tx.Total <= 0 {
					tx.Total = tx.Amount * tx.PurchasePrice
				}

				// El valor actual es lo que valdría si aún tuviéramos la criptomoneda
				details.CurrentValue = tx.Amount * currentPrice

				// Para ventas, la ganancia/pérdida debe ser lo que se recibió por la venta (tx.Total) menos lo que valdría ahora (details.CurrentValue)
				details.GainLoss = tx.Total - details.CurrentValue

				// Calcular el porcentaje de ganancia/pérdida
				if details.CurrentValue > 0 {
					details.GainLossPercent = (details.GainLoss / details.CurrentValue) * 100
				}
			}
		} else {
			// Si hay un error, usar el precio de compra
			details.CurrentPrice = tx.PurchasePrice

			// Para ventas, aún podemos calcular la ganancia/pérdida
			if tx.Type == models.TransactionTypeSell {
				// Intentar obtener el precio promedio de compra
				avgPrice, err := r.getAveragePurchasePrice(tx.UserID, tx.Ticker, tx.Date)
				if err != nil || avgPrice <= 0 {
					// Si hay un error o el precio promedio es 0, usar el precio de compra de la transacción
					avgPrice = tx.PurchasePrice
				}

				// costBasis := avgPrice * tx.Amount

				// Asegurarse de que el total sea correcto (lo que se recibió por la venta)
				if tx.USDTReceived > 0 {
					tx.Total = tx.USDTReceived
				} else if tx.Total <= 0 {
					tx.Total = tx.Amount * tx.PurchasePrice
				}

				// El valor actual es lo que valdría si aún tuviéramos la criptomoneda
				details.CurrentValue = tx.Amount * tx.PurchasePrice

				// La ganancia/pérdida es lo que se recibió menos lo que valdría ahora
				details.GainLoss = tx.Total - details.CurrentValue

				// Calcular el porcentaje de ganancia/pérdida
				if details.CurrentValue > 0 {
					details.GainLossPercent = (details.GainLoss / details.CurrentValue) * 100
				}
			} else {
				// Para compras, el valor actual es la cantidad multiplicada por el precio actual (que en este caso es el precio de compra)
				details.CurrentValue = tx.Amount * tx.PurchasePrice
				details.GainLoss = 0
				details.GainLossPercent = 0
			}
		}

		transactions = append(transactions, details)
	}

	if err = rows.Err(); err != nil {
		return nil, err
	}

	return transactions, nil
}

func (r *CryptoRepository) getAveragePurchasePrice(userID string, ticker string, date time.Time) (float64, error) {
	query := `
		SELECT AVG(purchase_price) 
		FROM crypto_transactions 
		WHERE user_id = ? AND ticker = ? AND date < ?
	`

	var avgPrice float64
	err := r.db.QueryRow(query, userID, ticker, date).Scan(&avgPrice)
	if err != nil {
		if err == sql.ErrNoRows {
			return 0, nil
		}
		return 0, err
	}

	return avgPrice, nil
}

func (r *CryptoRepository) GetInvestmentHistory(userID string) (models.InvestmentHistory, error) {
	// Obtener todas las transacciones del usuario ordenadas por fecha
	query := `
		SELECT id, ticker, crypto_name, amount, purchase_price, total, type, date
		FROM crypto_transactions
		WHERE user_id = ?
		ORDER BY date ASC`

	rows, err := r.db.Query(query, userID)
	if err != nil {
		return models.InvestmentHistory{}, err
	}
	defer rows.Close()

	// Estructura para almacenar transacciones por día
	type DailyTransactions struct {
		Buys  float64
		Sells float64
	}

	// Mapa para acumular transacciones por día
	dailyTxs := make(map[string]*DailyTransactions)

	// Variables para seguimiento
	var firstDate time.Time
	var allDates []string

	// Procesar cada transacción cronológicamente para agruparlas por día
	for rows.Next() {
		var id, ticker, cryptoName, txType string
		var amount, purchasePrice, total float64
		var date time.Time

		err := rows.Scan(&id, &ticker, &cryptoName, &amount, &purchasePrice, &total, &txType, &date)
		if err != nil {
			return models.InvestmentHistory{}, err
		}

		// Formato de fecha como string (YYYY-MM-DD)
		dateStr := date.Format("2006-01-02")

		// Agregar la fecha al slice si no existe
		dateExists := false
		for _, d := range allDates {
			if d == dateStr {
				dateExists = true
				break
			}
		}
		if !dateExists {
			allDates = append(allDates, dateStr)
		}

		// Inicializar el registro diario si no existe
		if _, exists := dailyTxs[dateStr]; !exists {
			dailyTxs[dateStr] = &DailyTransactions{
				Buys:  0,
				Sells: 0,
			}
		}

		// Actualizar compras o ventas según el tipo de transacción
		if txType == models.TransactionTypeBuy {
			dailyTxs[dateStr].Buys += total
		} else if txType == models.TransactionTypeSell {
			dailyTxs[dateStr].Sells += total
		}

		// Actualizar la fecha inicial
		if firstDate.IsZero() {
			firstDate = date
		}
	}

	// Ordenar las fechas cronológicamente
	sort.Strings(allDates)

	// Crear el historial de valores diarios
	history := make([]models.DailyValue, 0, len(allDates))
	var runningTotal float64 = 0

	// Procesar cada día en orden cronológico
	for i, dateStr := range allDates {
		// Obtener las transacciones del día
		txs := dailyTxs[dateStr]

		// Calcular el valor neto del día (compras - ventas)
		dailyNetValue := txs.Buys - txs.Sells

		// Actualizar el total acumulado
		runningTotal += dailyNetValue

		// Crear el valor diario
		dailyValue := models.DailyValue{
			Date:       dateStr,
			TotalValue: runningTotal,
		}

		// Calcular el porcentaje de cambio
		if i > 0 && history[i-1].TotalValue > 0 {
			dailyValue.ChangePercentage = ((dailyValue.TotalValue - history[i-1].TotalValue) / history[i-1].TotalValue) * 100
		} else {
			dailyValue.ChangePercentage = 0
		}

		// Agregar al historial
		history = append(history, dailyValue)
	}

	// Calcular el porcentaje de tendencia general (desde el inicio hasta el final)
	var trendPercentage float64 = 0
	if len(history) > 1 && history[0].TotalValue > 0 {
		firstValue := history[0].TotalValue
		lastValue := history[len(history)-1].TotalValue
		trendPercentage = ((lastValue - firstValue) / firstValue) * 100
	}

	// Crear el objeto de historial de inversión
	investmentHistory := models.InvestmentHistory{
		StartDate:       firstDate,
		History:         history,
		TrendPercentage: trendPercentage,
	}

	return investmentHistory, nil
}

func (r *CryptoRepository) SaveInvestmentSnapshot(userID string, totalValue, totalInvested, profit, profitPercentage float64) error {
	// Verificar que los valores sean válidos
	if totalValue <= 0 || totalInvested <= 0 {
		log.Printf("No se guardó el snapshot porque los valores no son válidos: totalValue=%f, totalInvested=%f", totalValue, totalInvested)
		return nil // No guardamos snapshots con valores inválidos
	}

	// Obtener la fecha actual
	currentTime := time.Now()

	// Verificar si ya existe un snapshot para el día actual
	currentDateStr := currentTime.Format("2006-01-02")

	// Consultar si existe un snapshot para la fecha actual
	query := `
		SELECT id FROM investment_snapshots 
		WHERE user_id = ? AND date(date) = date(?)
		ORDER BY date DESC LIMIT 1
	`

	var existingID string
	err := r.db.QueryRow(query, userID, currentTime).Scan(&existingID)

	// Si existe un snapshot para hoy, actualizarlo en lugar de crear uno nuevo
	if err == nil {
		log.Printf("Ya existe un snapshot para hoy (%s) con ID %s, actualizándolo", currentDateStr, existingID)

		updateQuery := `
			UPDATE investment_snapshots 
			SET total_value = ?, total_invested = ?, profit = ?, profit_percentage = ?, date = ?
			WHERE id = ?
		`

		_, err := r.db.Exec(
			updateQuery,
			totalValue,
			totalInvested,
			profit,
			profitPercentage,
			currentTime,
			existingID,
		)

		if err != nil {
			log.Printf("Error al actualizar snapshot existente: %v", err)
			return err
		}

		log.Printf("Snapshot actualizado exitosamente para la fecha %s", currentDateStr)
		return nil
	}

	// Si no existe un snapshot para hoy o hubo un error, crear uno nuevo
	// Generar un ID único para el snapshot
	id := fmt.Sprintf("snapshot_%d", time.Now().UnixNano())

	// Registrar los valores que se van a guardar
	log.Printf("Guardando nuevo snapshot para usuario %s en fecha %s: totalValue=%f, totalInvested=%f, profit=%f, profitPercentage=%f",
		userID, currentDateStr, totalValue, totalInvested, profit, profitPercentage)

	// Insertar el snapshot en la base de datos
	insertQuery := `
		INSERT INTO investment_snapshots (
			id, user_id, date, total_value, total_invested, profit, profit_percentage
		) VALUES (?, ?, ?, ?, ?, ?, ?)
	`

	_, err = r.db.Exec(
		insertQuery,
		id,
		userID,
		currentTime,
		totalValue,
		totalInvested,
		profit,
		profitPercentage,
	)

	if err != nil {
		log.Printf("Error al guardar nuevo snapshot: %v", err)
	} else {
		log.Printf("Nuevo snapshot guardado exitosamente con ID: %s para la fecha %s", id, currentDateStr)
	}

	return err
}

func (r *CryptoRepository) GetInvestmentSnapshots(userID string) ([]models.InvestmentSnapshot, error) {
	// Consultar los snapshots ordenados por fecha
	query := `
		SELECT id, user_id, date, total_value, total_invested, profit, profit_percentage
		FROM investment_snapshots
		WHERE user_id = ?
		ORDER BY date ASC
	`

	rows, err := r.db.Query(query, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	// Procesar los resultados
	snapshots := []models.InvestmentSnapshot{}
	for rows.Next() {
		var snapshot models.InvestmentSnapshot
		err := rows.Scan(
			&snapshot.ID,
			&snapshot.UserID,
			&snapshot.Date,
			&snapshot.TotalValue,
			&snapshot.TotalInvested,
			&snapshot.Profit,
			&snapshot.ProfitPercentage,
		)
		if err != nil {
			return nil, err
		}

		snapshots = append(snapshots, snapshot)
	}

	return snapshots, nil
}

func (r *CryptoRepository) GetInvestmentHistoryFromSnapshots(userID string) (models.InvestmentHistory, error) {
	// Obtener todos los snapshots del usuario
	snapshots, err := r.GetInvestmentSnapshots(userID)
	if err != nil {
		return models.InvestmentHistory{}, err
	}

	// Si no hay snapshots, devolver un historial vacío
	if len(snapshots) == 0 {
		return models.InvestmentHistory{
			StartDate:       time.Now(),
			History:         []models.DailyValue{},
			TrendPercentage: 0,
		}, nil
	}

	// Agrupar snapshots por día (usando solo el más reciente de cada día)
	dailySnapshots := make(map[string]models.InvestmentSnapshot)
	var allDates []string

	for _, snapshot := range snapshots {
		dateStr := snapshot.Date.Format("2006-01-02")

		// Si ya existe un snapshot para este día, solo actualizar si este es más reciente
		if existing, exists := dailySnapshots[dateStr]; exists {
			if snapshot.Date.After(existing.Date) {
				dailySnapshots[dateStr] = snapshot
			}
		} else {
			dailySnapshots[dateStr] = snapshot
			allDates = append(allDates, dateStr)
		}
	}

	// Ordenar las fechas
	sort.Strings(allDates)

	// Crear el historial de valores diarios
	history := make([]models.DailyValue, 0, len(allDates))

	// Procesar cada día en orden cronológico
	for i, dateStr := range allDates {
		snapshot := dailySnapshots[dateStr]

		// Crear el valor diario
		dailyValue := models.DailyValue{
			Date:       dateStr,
			TotalValue: snapshot.TotalValue,
		}

		// Calcular el porcentaje de cambio
		if i > 0 && history[i-1].TotalValue > 0 {
			dailyValue.ChangePercentage = ((dailyValue.TotalValue - history[i-1].TotalValue) / history[i-1].TotalValue) * 100
		} else {
			dailyValue.ChangePercentage = 0
		}

		// Agregar al historial
		history = append(history, dailyValue)
	}

	// Calcular el porcentaje de tendencia general (desde el inicio hasta el final)
	var trendPercentage float64 = 0
	if len(history) > 1 && history[0].TotalValue > 0 {
		firstValue := history[0].TotalValue
		lastValue := history[len(history)-1].TotalValue
		trendPercentage = ((lastValue - firstValue) / firstValue) * 100
	}

	// Crear el objeto de historial de inversión
	investmentHistory := models.InvestmentHistory{
		StartDate:       snapshots[0].Date,
		History:         history,
		TrendPercentage: trendPercentage,
	}

	return investmentHistory, nil
}
