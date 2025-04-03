package repository

import (
	"database/sql"
	"errors"
	"fmt"
	"log"
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
		return fmt.Errorf("no se encontraron transacciones con el ticker %s", ticker)
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
			costBasis := avgPrice * tx.Amount

			// Asegurarse de que el total sea correcto (lo que se recibió por la venta)
			if tx.USDTReceived > 0 {
				tx.Total = tx.USDTReceived
			} else if tx.Total <= 0 {
				tx.Total = tx.Amount * tx.PurchasePrice
			}

			// El valor actual es lo que valdría si aún tuviéramos la criptomoneda
			details.CurrentValue = tx.Amount * tx.PurchasePrice

			// La ganancia/pérdida es lo que se recibió menos lo que costó
			details.GainLoss = tx.Total - costBasis

			// Calcular el porcentaje de ganancia/pérdida
			if costBasis > 0 {
				details.GainLossPercent = (details.GainLoss / costBasis) * 100
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
