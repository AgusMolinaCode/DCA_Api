package repository

import (
	"database/sql"
	"fmt"
	"time"
	
	"github.com/AgusMolinaCode/DCA_Api.git/internal/models"
	"github.com/AgusMolinaCode/DCA_Api.git/internal/services"
)

func (r *CryptoRepository) GetPerformance(userID string) (*models.Performance, error) {
	// Obtener el dashboard para calcular el rendimiento
	dashboard, err := r.GetCryptoDashboard(userID)
	if err != nil {
		return nil, err
	}

	// Si no hay criptomonedas, devolver un rendimiento vacío
	if len(dashboard) == 0 {
		return &models.Performance{
			TopGainer: models.PerformanceDetail{
				Ticker:       "",
				ChangePct24h: 0,
				PriceChange:  0,
				ImageURL:     "",
			},
			TopLoser: models.PerformanceDetail{
				Ticker:       "",
				ChangePct24h: 0,
				PriceChange:  0,
				ImageURL:     "",
			},
		}, nil
	}

	// Si solo hay una criptomoneda, ponerla como ganadora o perdedora según su rendimiento
	if len(dashboard) == 1 {
		crypto := dashboard[0]

		// Obtener datos de cambio en 24h
		cryptoData, err := services.GetCryptoPrice(crypto.Ticker)
		if err != nil {
			return nil, err
		}

		changePct24h := cryptoData.Raw[crypto.Ticker]["USD"].CHANGEPCT24HOUR
		priceChange := cryptoData.Raw[crypto.Ticker]["USD"].CHANGE24HOUR

		// Obtener la URL de la imagen
		imageURL, err := services.GetCryptoImageURL(crypto.Ticker)
		if err != nil {
			// Si hay un error, usar una URL vacía
			imageURL = ""
		}

		// Si el cambio es positivo, ponerla como ganadora
		if changePct24h >= 0 {
			return &models.Performance{
				TopGainer: models.PerformanceDetail{
					Ticker:       crypto.Ticker,
					ChangePct24h: changePct24h,
					PriceChange:  priceChange,
					ImageURL:     imageURL,
				},
				TopLoser: models.PerformanceDetail{
					Ticker:       "",
					ChangePct24h: 0,
					PriceChange:  0,
					ImageURL:     "",
				},
			}, nil
		} else {
			// Si el cambio es negativo, ponerla como perdedora
			return &models.Performance{
				TopGainer: models.PerformanceDetail{
					Ticker:       "",
					ChangePct24h: 0,
					PriceChange:  0,
					ImageURL:     "",
				},
				TopLoser: models.PerformanceDetail{
					Ticker:       crypto.Ticker,
					ChangePct24h: changePct24h,
					PriceChange:  priceChange,
					ImageURL:     imageURL,
				},
			}, nil
		}
	}

	// Para múltiples criptomonedas, encontrar la mejor y la peor
	var topGainer, topLoser models.PerformanceDetail
	topGainer.ChangePct24h = -999999
	topLoser.ChangePct24h = 999999

	for _, crypto := range dashboard {
		// Ignorar USDT para el cálculo de rendimiento
		if crypto.Ticker == "USDT" {
			continue
		}

		// Obtener datos de cambio en 24h
		cryptoData, err := services.GetCryptoPrice(crypto.Ticker)
		if err != nil {
			continue
		}

		changePct24h := cryptoData.Raw[crypto.Ticker]["USD"].CHANGEPCT24HOUR
		priceChange := cryptoData.Raw[crypto.Ticker]["USD"].CHANGE24HOUR

		// Obtener la URL de la imagen
		imageURL, err := services.GetCryptoImageURL(crypto.Ticker)
		if err != nil {
			// Si hay un error, usar una URL vacía
			imageURL = ""
		}

		// Actualizar el mejor rendimiento
		if changePct24h > topGainer.ChangePct24h {
			topGainer.Ticker = crypto.Ticker
			topGainer.ChangePct24h = changePct24h
			topGainer.PriceChange = priceChange
			topGainer.ImageURL = imageURL
		}

		// Actualizar el peor rendimiento
		if changePct24h < topLoser.ChangePct24h {
			topLoser.Ticker = crypto.Ticker
			topLoser.ChangePct24h = changePct24h
			topLoser.PriceChange = priceChange
			topLoser.ImageURL = imageURL
		}
	}

	// Si no se encontraron datos válidos
	if topGainer.Ticker == "" {
		topGainer.ChangePct24h = 0
		topGainer.PriceChange = 0
		topGainer.ImageURL = ""
	}

	if topLoser.Ticker == "" {
		topLoser.ChangePct24h = 0
		topLoser.PriceChange = 0
		topLoser.ImageURL = ""
	}

	return &models.Performance{
		TopGainer: topGainer,
		TopLoser:  topLoser,
	}, nil
}

// GetUserPerformance obtiene el rendimiento de las inversiones del usuario desde una fecha específica
// Esta función es un wrapper para GetPerformance que permite su uso desde los handlers
func GetUserPerformance(db *sql.DB, userID string, startDate time.Time) (*models.Performance, error) {
	// Crear una instancia del repositorio de criptomonedas
	repo := NewCryptoRepository(db)
	
	// Llamar a la función GetPerformance para obtener el rendimiento
	performance, err := repo.GetPerformance(userID)
	if err != nil {
		return nil, fmt.Errorf("error al obtener el rendimiento: %v", err)
	}
	
	// Nota: Actualmente no estamos filtrando por startDate, pero podríamos implementarlo
	// en una versión futura para mostrar el rendimiento desde una fecha específica
	
	return performance, nil
}

// GetUserHoldings obtiene las tenencias actuales del usuario
// Esta función utiliza el dashboard para obtener las tenencias
func GetUserHoldings(db *sql.DB, userID string) ([]models.CryptoDashboard, error) {
	// Crear una instancia del repositorio de criptomonedas
	repo := NewCryptoRepository(db)
	
	// Obtener el dashboard que ya contiene las tenencias
	dashboard, err := repo.GetCryptoDashboard(userID)
	if err != nil {
		return nil, fmt.Errorf("error al obtener las tenencias: %v", err)
	}
	
	// Filtrar solo las criptomonedas con tenencias positivas
	holdings := make([]models.CryptoDashboard, 0)
	for _, crypto := range dashboard {
		if crypto.Holdings > 0 {
			holdings = append(holdings, crypto)
		}
	}
	
	return holdings, nil
}

// GetUserCurrentBalance obtiene el balance actual del usuario
// Esta función calcula el balance sumando el valor actual de todas las tenencias
func GetUserCurrentBalance(db *sql.DB, userID string) (*models.Balance, error) {
	// Crear una instancia del repositorio de criptomonedas
	repo := NewCryptoRepository(db)
	
	// Obtener el dashboard que contiene las tenencias
	dashboard, err := repo.GetCryptoDashboard(userID)
	if err != nil {
		return nil, fmt.Errorf("error al obtener el balance: %v", err)
	}
	
	// Calcular el balance total y el total invertido
	var totalBalance, totalInvested, totalProfit float64
	for _, crypto := range dashboard {
		// Calcular el valor actual de las tenencias
		currentValue := crypto.CurrentPrice * crypto.Holdings
		totalBalance += currentValue
		totalInvested += crypto.TotalInvested
	}
	
	// Calcular la ganancia/pérdida total
	totalProfit = totalBalance - totalInvested
	
	// Calcular el porcentaje de ganancia/pérdida
	var profitPercentage float64
	if totalInvested > 0 {
		profitPercentage = (totalProfit / totalInvested) * 100
	}
	
	// Crear y devolver el objeto de balance
	balance := &models.Balance{
		TotalBalance:      totalBalance,
		TotalInvested:     totalInvested,
		TotalProfit:       totalProfit,
		ProfitPercentage:  profitPercentage,
		LastUpdated:       time.Now(),
	}
	
	return balance, nil
}
