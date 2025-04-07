package repository

import (
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
