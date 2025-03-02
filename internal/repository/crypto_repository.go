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
		INSERT INTO crypto_transactions (id, user_id, crypto_name, ticker, amount, purchase_price, total, date, note, type, usdt_received)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`

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
		tx.Type,
		tx.USDTReceived,
	)

	// Si es una venta, actualizar las tenencias
	if tx.Type == models.TransactionTypeSell {
		err = r.UpdateHoldingsAfterSale(tx)
		if err != nil {
			return fmt.Errorf("error al actualizar holdings después de la venta: %v", err)
		}
	}

	return err
}

func (r *CryptoRepository) UpdateHoldingsAfterSale(tx *models.CryptoTransaction) error {
	// Verificar que el usuario tenga suficiente cantidad para vender
	query := `
		SELECT SUM(amount) as total_amount
		FROM crypto_transactions 
		WHERE user_id = ? AND ticker = ?`

	var totalAmount float64
	err := r.db.QueryRow(query, tx.UserID, tx.Ticker).Scan(&totalAmount)
	if err != nil {
		return err
	}

	// Si la cantidad a vender es mayor que la disponible, retornar error
	if tx.Amount > totalAmount {
		return fmt.Errorf("cantidad insuficiente para vender: disponible %.8f, solicitado %.8f", totalAmount, tx.Amount)
	}

	return nil
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
	query := `
		SELECT 
			ticker,
			SUM(CASE WHEN type = 'compra' THEN amount ELSE -amount END) as holdings,
			SUM(CASE WHEN type = 'compra' THEN total ELSE -total END) as total_invested,
			(SUM(CASE WHEN type = 'compra' THEN total ELSE 0 END) / 
			 SUM(CASE WHEN type = 'compra' THEN amount ELSE 0 END)) as avg_price
		FROM crypto_transactions 
		WHERE user_id = ?
		GROUP BY ticker
		HAVING holdings > 0`

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

		// Caso especial para USDT: total_invested debe ser igual a holdings
		if d.Ticker == "USDT" {
			d.TotalInvested = d.Holdings
			d.AvgPrice = 1.0
			d.CurrentPrice = 1.0
			d.CurrentProfit = 0.0
			d.ProfitPercent = 0.0
			dashboards = append(dashboards, d)
			log.Printf("USDT procesado con valores ajustados: holdings=%f, total_invested=%f", d.Holdings, d.TotalInvested)
			continue
		}

		// Obtener precio actual para otras criptomonedas
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
			&tx.Type,
			&tx.USDTReceived,
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
	// Consulta para obtener detalles de transacciones por ticker
	query := `
		SELECT 
			ticker,
			crypto_name,
			SUM(CASE WHEN type = 'compra' THEN amount ELSE -amount END) as total_amount,
			SUM(CASE WHEN type = 'compra' THEN total ELSE -total END) as total_invested
		FROM crypto_transactions 
		WHERE user_id = ?
		GROUP BY ticker, crypto_name
		HAVING total_amount > 0`

	rows, err := r.db.Query(query, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var totalCurrentValue float64
	var totalInvested float64

	// Slice para almacenar la información de cada criptomoneda
	var cryptoWeights []models.CryptoWeight

	// Procesar cada ticker
	for rows.Next() {
		var ticker, cryptoName string
		var amount, totalInvestedTicker float64
		if err := rows.Scan(&ticker, &cryptoName, &amount, &totalInvestedTicker); err != nil {
			return nil, err
		}

		// Obtener precio actual
		cryptoData, err := services.GetCryptoPrice(ticker)
		if err != nil {
			log.Printf("Error obteniendo precio para %s: %v", ticker, err)
			continue
		}

		if cryptoData.Raw[ticker]["USD"].PRICE > 0 {
			currentPrice := cryptoData.Raw[ticker]["USD"].PRICE
			currentValue := amount * currentPrice

			// Agregar a los totales
			totalCurrentValue += currentValue
			totalInvested += totalInvestedTicker

			// Guardar información para calcular la distribución
			cryptoWeights = append(cryptoWeights, models.CryptoWeight{
				Ticker: ticker,
				Name:   cryptoName,
				Value:  currentValue,
			})
		}
	}

	// Calcular porcentajes y profit total
	totalProfit := totalCurrentValue - totalInvested
	profitPercentage := 0.0
	if totalInvested > 0 {
		profitPercentage = (totalProfit / totalInvested) * 100
	}

	// Calcular el peso de cada criptomoneda en el portafolio
	// y ordenar de mayor a menor
	const othersThreshold = 5.0 // Umbral para agrupar en "OTROS" (porcentaje)
	var distribution []models.CryptoWeight
	var othersValue float64

	// Calcular el peso de cada criptomoneda
	for i := range cryptoWeights {
		weight := (cryptoWeights[i].Value / totalCurrentValue) * 100
		cryptoWeights[i].Weight = weight
	}

	// Ordenar de mayor a menor peso
	sort.Slice(cryptoWeights, func(i, j int) bool {
		return cryptoWeights[i].Weight > cryptoWeights[j].Weight
	})

	// Procesar la distribución final
	for _, cw := range cryptoWeights {
		if cw.Weight >= othersThreshold {
			// Agregar directamente a la distribución
			distribution = append(distribution, cw)
		} else {
			// Acumular en "OTROS"
			othersValue += cw.Value
		}
	}

	// Agregar la categoría "OTROS" si hay valores
	if othersValue > 0 {
		othersWeight := (othersValue / totalCurrentValue) * 100
		distribution = append(distribution, models.CryptoWeight{
			Ticker:   "OTROS",
			Value:    othersValue,
			Weight:   othersWeight,
			IsOthers: true,
		})
	}

	// Generar datos para el gráfico de torta
	pieChartData := models.PieChartData{
		Currency: "USD",
	}

	// Colores predefinidos para las criptomonedas más comunes
	colorMap := map[string]string{
		"BTC":   "#F7931A", // Naranja Bitcoin
		"ETH":   "#627EEA", // Azul Ethereum
		"BNB":   "#F3BA2F", // Amarillo Binance
		"SOL":   "#14F195", // Verde Solana
		"ADA":   "#0033AD", // Azul Cardano
		"XRP":   "#23292F", // Negro Ripple
		"DOT":   "#E6007A", // Rosa Polkadot
		"DOGE":  "#C3A634", // Dorado Dogecoin
		"AVAX":  "#E84142", // Rojo Avalanche
		"MATIC": "#8247E5", // Púrpura Polygon
		"OTROS": "#808080", // Gris para "OTROS"
	}

	// Colores por defecto si no están en el mapa
	defaultColors := []string{
		"#FF6384", "#36A2EB", "#FFCE56", "#4BC0C0", "#9966FF",
		"#FF9F40", "#C9CBCF", "#7CFC00", "#00FFFF", "#FF00FF",
	}

	colorIndex := 0

	// Generar etiquetas, valores y colores para el gráfico
	for _, cw := range distribution {
		pieChartData.Labels = append(pieChartData.Labels, cw.Ticker)
		pieChartData.Values = append(pieChartData.Values, cw.Weight)

		// Asignar color
		color, exists := colorMap[cw.Ticker]
		if !exists {
			color = defaultColors[colorIndex%len(defaultColors)]
			colorIndex++
		}
		pieChartData.Colors = append(pieChartData.Colors, color)

		// Actualizar el color en la distribución también
		for i := range distribution {
			if distribution[i].Ticker == cw.Ticker {
				distribution[i].Color = color
				break
			}
		}
	}

	return &models.Holdings{
		TotalCurrentValue: totalCurrentValue,
		TotalInvested:     totalInvested,
		TotalProfit:       totalProfit,
		ProfitPercentage:  profitPercentage,
		Distribution:      distribution,
		ChartData:         pieChartData,
	}, nil
}

// UpdateTransaction actualiza una transacción existente
func (r *CryptoRepository) UpdateTransaction(tx *models.CryptoTransaction) error {
	query := `
		UPDATE crypto_transactions 
		SET crypto_name = ?, ticker = ?, amount = ?, purchase_price = ?, 
		    total = ?, date = ?, note = ?, type = ?, usdt_received = ?
		WHERE id = ? AND user_id = ?`

	_, err := r.db.Exec(query,
		tx.CryptoName,
		tx.Ticker,
		tx.Amount,
		tx.PurchasePrice,
		tx.Total,
		tx.Date,
		tx.Note,
		tx.Type,
		tx.USDTReceived,
		tx.ID,
		tx.UserID,
	)

	return err
}

// DeleteTransaction elimina una transacción
func (r *CryptoRepository) DeleteTransaction(userID string, transactionID string) error {
	query := `DELETE FROM crypto_transactions WHERE id = ? AND user_id = ?`

	result, err := r.db.Exec(query, transactionID, userID)
	if err != nil {
		return err
	}

	// Verificar si se eliminó alguna fila
	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return err
	}

	if rowsAffected == 0 {
		return fmt.Errorf("no se encontró la transacción o no tienes permiso para eliminarla")
	}

	return nil
}
