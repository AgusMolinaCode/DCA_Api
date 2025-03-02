package repository

import (
	"database/sql"
	"errors"
	"sort"

	"github.com/AgusMolinaCode/DCA_Api.git/internal/models"
)

// HoldingsRepository maneja las operaciones relacionadas con las tenencias de criptomonedas
type HoldingsRepository struct {
	db *sql.DB
}

// NewHoldingsRepository crea un nuevo repositorio de tenencias
func NewHoldingsRepository(db *sql.DB) *HoldingsRepository {
	return &HoldingsRepository{
		db: db,
	}
}

// UpdateHoldingsAfterSale verifica si el usuario tiene suficiente criptomoneda para vender
func (r *HoldingsRepository) UpdateHoldingsAfterSale(tx *sql.Tx, userID, ticker string, amountToSell float64) error {
	// Obtener todas las transacciones del usuario para esta criptomoneda
	query := `
		SELECT type, amount
		FROM crypto_transactions
		WHERE user_id = ? AND ticker = ?
	`
	rows, err := tx.Query(query, userID, ticker)
	if err != nil {
		return err
	}
	defer rows.Close()

	// Calcular el balance actual
	var balance float64
	for rows.Next() {
		var txType string
		var amount float64
		err := rows.Scan(&txType, &amount)
		if err != nil {
			return err
		}

		if txType == models.TransactionTypeBuy {
			balance += amount
		} else if txType == models.TransactionTypeSell {
			balance -= amount
		}
	}

	// Verificar si hay suficiente balance para vender
	if balance < amountToSell {
		return errors.New("saldo insuficiente para realizar la venta")
	}

	return nil
}

// GetHoldings obtiene las tenencias de criptomonedas de un usuario
func (r *HoldingsRepository) GetHoldings(userID string) (models.Holdings, error) {
	// Obtener el dashboard para calcular las tenencias
	cryptoRepo := NewCryptoRepository(r.db)
	dashboard, err := cryptoRepo.GetCryptoDashboard(userID)
	if err != nil {
		return models.Holdings{}, err
	}

	// Si no hay datos en el dashboard, devolver una estructura vacía
	if len(dashboard) == 0 {
		return models.Holdings{
			TotalCurrentValue: 0,
			TotalInvested:     0,
			TotalProfit:       0,
			ProfitPercentage:  0,
			Distribution:      []models.CryptoWeight{},
			ChartData: models.PieChartData{
				Labels:   []string{},
				Values:   []float64{},
				Currency: "USD",
			},
		}, nil
	}

	// Calcular totales
	var totalCurrentValue, totalInvested, totalProfit float64
	var cryptoWeights []models.CryptoWeight

	// Procesar cada criptomoneda en el dashboard
	for _, crypto := range dashboard {
		currentValue := crypto.Holdings * crypto.CurrentPrice
		totalCurrentValue += currentValue
		totalInvested += crypto.TotalInvested
		totalProfit += crypto.CurrentProfit

		// Guardar información para calcular la distribución
		cryptoWeights = append(cryptoWeights, models.CryptoWeight{
			Ticker: crypto.Ticker,
			Name:   crypto.Ticker, // Usar el ticker como nombre
			Value:  currentValue,
		})
	}

	// Calcular porcentaje de ganancia
	var profitPercentage float64
	if totalInvested > 0 {
		profitPercentage = (totalProfit / totalInvested) * 100
	}

	// Calcular la distribución (peso) de cada criptomoneda
	const othersThreshold = 5.0
	var distribution []models.CryptoWeight
	var othersValue float64
	var othersDetails []models.CryptoWeight

	// Calcular el peso de cada criptomoneda
	for i := range cryptoWeights {
		if totalCurrentValue > 0 {
			cryptoWeights[i].Weight = (cryptoWeights[i].Value / totalCurrentValue) * 100
		}
	}

	// Ordenar por peso (de mayor a menor)
	sort.Slice(cryptoWeights, func(i, j int) bool {
		return cryptoWeights[i].Weight > cryptoWeights[j].Weight
	})

	// Procesar la distribución final
	for _, crypto := range cryptoWeights {
		// Si el peso es menor que el umbral, acumular en "OTROS"
		if crypto.Weight < othersThreshold {
			othersValue += crypto.Value
			// Guardar detalles para la categoría "OTROS"
			othersDetails = append(othersDetails, crypto)
		} else {
			// Asignar un color según la posición
			var color string
			switch len(distribution) {
			case 0:
				color = "#FF9500" // Naranja para la primera (generalmente BTC)
			case 1:
				color = "#7D7AFF" // Púrpura para la segunda (generalmente ETH)
			default:
				color = "#30D158" // Verde para las demás
			}

			distribution = append(distribution, models.CryptoWeight{
				Ticker: crypto.Ticker,
				Name:   crypto.Name,
				Value:  crypto.Value,
				Weight: crypto.Weight,
				Color:  color,
			})
		}
	}

	// Si hay criptomonedas en "OTROS", agregar esta categoría
	if othersValue > 0 {
		othersWeight := (othersValue / totalCurrentValue) * 100
		distribution = append(distribution, models.CryptoWeight{
			Ticker:       "OTROS",
			Name:         "OTROS",
			Value:        othersValue,
			Weight:       othersWeight,
			IsOthers:     true,
			Color:        "#FF3B30", // Rojo para "OTROS"
			OthersDetail: othersDetails,
		})
	}

	// Generar datos para el gráfico de torta
	pieChartData := models.PieChartData{
		Currency: "USD",
	}

	// Generar etiquetas y valores para el gráfico
	for _, cw := range distribution {
		pieChartData.Labels = append(pieChartData.Labels, cw.Ticker)
		pieChartData.Values = append(pieChartData.Values, cw.Weight)
		pieChartData.Colors = append(pieChartData.Colors, cw.Color)
	}

	return models.Holdings{
		TotalCurrentValue: totalCurrentValue,
		TotalInvested:     totalInvested,
		TotalProfit:       totalProfit,
		ProfitPercentage:  profitPercentage,
		Distribution:      distribution,
		ChartData:         pieChartData,
	}, nil
}
