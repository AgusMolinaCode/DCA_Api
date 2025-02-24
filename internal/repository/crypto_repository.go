package repository

import (
	"database/sql"
	"fmt"
	"log"
	"sort"

	"github.com/AgusMolinaCode/DCA_Api.git/internal/models"
	"github.com/AgusMolinaCode/DCA_Api.git/internal/services"
)

type CryptoRepository struct {
	db *sql.DB
}

func NewCryptoRepository(db *sql.DB) *CryptoRepository {
	return &CryptoRepository{db: db}
}

func (r *CryptoRepository) CreateTransaction(tx *models.CryptoTransaction) error {
	query := `
		INSERT INTO crypto_transactions (id, user_id, crypto_name, ticker, amount, purchase_price, total, date, note)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`

	_, err := r.db.Exec(query,
		tx.ID,
		tx.UserID,
		tx.CryptoName,
		tx.Ticker,
		tx.Amount,
		tx.PurchasePrice,
		tx.Total,
		tx.Date,
		tx.Note,
	)
	return err
}

func (r *CryptoRepository) GetUserTransactions(userID string) ([]models.CryptoTransaction, error) {
	query := `
		SELECT id, user_id, crypto_name, ticker, amount, purchase_price, total, date, note, created_at 
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
		)
		if err != nil {
			return nil, err
		}
		transactions = append(transactions, tx)
	}
	return transactions, nil
}

func (r *CryptoRepository) GetCryptoDashboard(userID string) ([]models.CryptoDashboard, error) {
	query := `
		SELECT 
			ticker,
			SUM(amount) as holdings,
			SUM(total) as total_invested,
			SUM(total)/SUM(amount) as avg_price
		FROM crypto_transactions 
		WHERE user_id = ?
		GROUP BY ticker`

	rows, err := r.db.Query(query, userID)
	if err != nil {
		log.Printf("Error en la consulta SQL: %v", err)
		return nil, fmt.Errorf("error en la consulta SQL: %v", err)
	}
	defer rows.Close()

	var dashboards []models.CryptoDashboard
	for rows.Next() {
		var d models.CryptoDashboard
		err := rows.Scan(
			&d.Ticker,
			&d.Holdings,
			&d.TotalInvested,
			&d.AvgPrice,
		)
		if err != nil {
			log.Printf("Error escaneando resultados: %v", err)
			return nil, fmt.Errorf("error escaneando resultados: %v", err)
		}

		log.Printf("Procesando ticker: %s", d.Ticker)

		// Obtener precio actual
		cryptoData, err := services.GetCryptoPrice(d.Ticker)
		if err != nil {
			log.Printf("Error obteniendo precio para %s: %v", d.Ticker, err)
			continue // Continuamos con la siguiente criptomoneda en caso de error
		}

		// Acceder dinámicamente a los datos usando el ticker
		if cryptoData.Raw[d.Ticker]["USD"].PRICE > 0 {
			d.CurrentPrice = cryptoData.Raw[d.Ticker]["USD"].PRICE
			d.CurrentProfit = (d.CurrentPrice * d.Holdings) - d.TotalInvested
			d.ProfitPercent = (d.CurrentProfit / d.TotalInvested) * 100
			dashboards = append(dashboards, d)
			log.Printf("Datos procesados exitosamente para %s", d.Ticker)
		} else {
			log.Printf("No se encontraron datos de precio para %s", d.Ticker)
		}
	}

	if len(dashboards) == 0 {
		log.Print("No se encontraron datos para ninguna criptomoneda")
		return nil, fmt.Errorf("no se encontraron datos para ninguna criptomoneda")
	}

	return dashboards, nil
}

func (r *CryptoRepository) GetTransactionDetails(userID string, transactionID string) (*models.TransactionDetails, error) {
	query := `
		SELECT id, user_id, crypto_name, ticker, amount, purchase_price, total, date, note, created_at 
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
		SELECT id, user_id, crypto_name, ticker, amount, purchase_price, total, date, note, created_at 
		FROM crypto_transactions 
		WHERE user_id = ?
		ORDER BY date DESC
		LIMIT ?`

	rows, err := r.db.Query(query, userID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var details []models.TransactionDetails
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
		)
		if err != nil {
			return nil, err
		}

		// Obtener precio actual para cada transacción
		cryptoData, err := services.GetCryptoPrice(tx.Ticker)
		if err != nil {
			log.Printf("Error obteniendo precio para %s: %v", tx.Ticker, err)
			continue
		}

		detail := models.TransactionDetails{
			Transaction: tx,
		}

		if cryptoData.Raw[tx.Ticker]["USD"].PRICE > 0 {
			detail.CurrentPrice = cryptoData.Raw[tx.Ticker]["USD"].PRICE
			detail.CurrentValue = tx.Amount * detail.CurrentPrice
			detail.GainLoss = detail.CurrentValue - tx.Total
			detail.GainLossPercent = (detail.GainLoss / tx.Total) * 100
			details = append(details, detail)
		}
	}

	return details, nil
}

func (r *CryptoRepository) GetPerformance(userID string) (*models.Performance, error) {
	// Obtener todos los tickers únicos del usuario
	query := `
		SELECT DISTINCT ticker
		FROM crypto_transactions 
		WHERE user_id = ?`

	rows, err := r.db.Query(query, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var tickers []string
	for rows.Next() {
		var ticker string
		if err := rows.Scan(&ticker); err != nil {
			return nil, err
		}
		tickers = append(tickers, ticker)
	}

	if len(tickers) == 0 {
		return nil, fmt.Errorf("no se encontraron criptomonedas")
	}

	performance := &models.Performance{
		TopGainer: models.PerformanceDetail{ChangePct24h: -100}, // Inicializar con valor mínimo
		TopLoser:  models.PerformanceDetail{ChangePct24h: 100},  // Inicializar con valor máximo
	}

	for _, ticker := range tickers {
		cryptoData, err := services.GetCryptoPrice(ticker)
		if err != nil {
			log.Printf("Error obteniendo precio para %s: %v", ticker, err)
			continue
		}

		if cryptoData.Raw[ticker]["USD"].PRICE > 0 {
			changePct := cryptoData.Raw[ticker]["USD"].CHANGEPCT24HOUR
			priceChange := cryptoData.Raw[ticker]["USD"].CHANGE24HOUR

			// Actualizar top gainer
			if changePct > performance.TopGainer.ChangePct24h {
				performance.TopGainer = models.PerformanceDetail{
					Ticker:       ticker,
					ChangePct24h: changePct,
					PriceChange:  priceChange,
				}
			}

			// Actualizar top loser
			if changePct < performance.TopLoser.ChangePct24h {
				performance.TopLoser = models.PerformanceDetail{
					Ticker:       ticker,
					ChangePct24h: changePct,
					PriceChange:  priceChange,
				}
			}
		}
	}

	return performance, nil
}

func (r *CryptoRepository) GetHoldings(userID string) (*models.Holdings, error) {
	query := `
		SELECT 
			ticker,
			SUM(amount) as holdings
		FROM crypto_transactions 
		WHERE user_id = ?
		GROUP BY ticker`

	rows, err := r.db.Query(query, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var holdings []models.HoldingDetail
	totalValue := 0.0

	// Primero, calculamos el valor total y recopilamos todos los holdings
	for rows.Next() {
		var ticker string
		var amount float64
		if err := rows.Scan(&ticker, &amount); err != nil {
			return nil, err
		}

		cryptoData, err := services.GetCryptoPrice(ticker)
		if err != nil {
			log.Printf("Error obteniendo precio para %s: %v", ticker, err)
			continue
		}

		if cryptoData.Raw[ticker]["USD"].PRICE > 0 {
			value := amount * cryptoData.Raw[ticker]["USD"].PRICE
			totalValue += value
			holdings = append(holdings, models.HoldingDetail{
				Ticker: ticker,
				Value:  value,
			})
		}
	}

	// Calcular porcentajes y separar en principales y otros
	const MINIMUM_PERCENTAGE = 1.0 // Holdings menores al 1% irán a Other Assets
	var result models.Holdings
	result.TotalValue = totalValue

	for i := range holdings {
		holdings[i].Percentage = (holdings[i].Value / totalValue) * 100

		if holdings[i].Percentage >= MINIMUM_PERCENTAGE {
			result.MainHoldings = append(result.MainHoldings, holdings[i])
		} else {
			result.OtherAssets = append(result.OtherAssets, holdings[i])
			result.OtherPercentage += holdings[i].Percentage
		}
	}

	// Ordenar MainHoldings por porcentaje descendente
	sort.Slice(result.MainHoldings, func(i, j int) bool {
		return result.MainHoldings[i].Percentage > result.MainHoldings[j].Percentage
	})

	// Ordenar OtherAssets por porcentaje descendente
	sort.Slice(result.OtherAssets, func(i, j int) bool {
		return result.OtherAssets[i].Percentage > result.OtherAssets[j].Percentage
	})

	return &result, nil
}
