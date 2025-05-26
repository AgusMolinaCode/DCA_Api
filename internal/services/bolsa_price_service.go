package services

import (
	"log"
	"sync"
	"time"

	"github.com/AgusMolinaCode/DCA_Api.git/internal/models"
)

// BolsaPriceService es un servicio para mantener actualizados los precios de las criptomonedas en las bolsas
type BolsaPriceService struct {
	priceCache map[string]cachedCryptoPrice
	mutex      sync.RWMutex
}

type cachedCryptoPrice struct {
	Price     float64
	Timestamp time.Time
}

// Singleton para el servicio
var (
	bolsaPriceService     *BolsaPriceService
	bolsaPriceServiceOnce sync.Once
)

// GetBolsaPriceService devuelve la instancia del servicio
func GetBolsaPriceService() *BolsaPriceService {
	bolsaPriceServiceOnce.Do(func() {
		bolsaPriceService = &BolsaPriceService{
			priceCache: make(map[string]cachedCryptoPrice),
		}
	})

	return bolsaPriceService
}

// GetCurrentPrice obtiene el precio actual de una criptomoneda
// Si el precio está en caché y es reciente (menos de 1 minuto), lo devuelve
// De lo contrario, obtiene el precio actual de la API
func (s *BolsaPriceService) GetCurrentPrice(ticker string) (float64, error) {
	// Siempre obtenemos el precio actual de la API para asegurar que esté actualizado
	// No usamos caché para garantizar que siempre tengamos el precio más reciente

	// Obtener el precio actual de la API
	cryptoData, err := GetCryptoPriceFromCoinGecko(ticker)
	if err != nil {
		log.Printf("Error al obtener precio actual para %s: %v", ticker, err)
		return 0, err
	}

	// Actualizar el caché
	s.mutex.Lock()
	s.priceCache[ticker] = cachedCryptoPrice{
		Price:     cryptoData.Price,
		Timestamp: time.Now(),
	}
	s.mutex.Unlock()

	log.Printf("Precio actualizado para %s: %.2f", ticker, cryptoData.Price)
	return cryptoData.Price, nil
}

// UpdateAssetPrices actualiza los precios de los activos en una bolsa
func (s *BolsaPriceService) UpdateAssetPrices(assets []models.AssetInBolsa) []models.AssetInBolsa {
	for i := range assets {
		// Obtener directamente el precio de la API de CoinGecko para asegurar que esté actualizado
		cryptoData, err := GetCryptoPriceFromCoinGecko(assets[i].Ticker)
		if err != nil {
			// Si no podemos obtener el precio actual, usamos el precio de compra
			log.Printf("Error al obtener precio para %s, usando precio de compra: %.2f", assets[i].Ticker, assets[i].PurchasePrice)
			assets[i].CurrentPrice = assets[i].PurchasePrice
		} else {
			// Siempre usar el precio actual de la API
			assets[i].CurrentPrice = cryptoData.Price
			log.Printf("Precio actualizado para %s: %.2f (precio anterior: %.2f)", assets[i].Ticker, cryptoData.Price, assets[i].CurrentPrice)
		}

		// Recalcular valores derivados
		assets[i].CurrentValue = assets[i].Amount * assets[i].CurrentPrice
		assets[i].GainLoss = assets[i].CurrentValue - assets[i].Total

		if assets[i].Total > 0 {
			assets[i].GainLossPercent = (assets[i].GainLoss / assets[i].Total) * 100
		}
	}

	return assets
}

// UpdateBolsaPrices actualiza los precios de todos los activos en una bolsa
func (s *BolsaPriceService) UpdateBolsaPrices(bolsa *models.Bolsa) *models.Bolsa {
	if bolsa == nil || len(bolsa.Assets) == 0 {
		return bolsa
	}

	// Actualizar los precios de los activos
	bolsa.Assets = s.UpdateAssetPrices(bolsa.Assets)

	// Recalcular el valor actual total de la bolsa
	bolsa.CurrentValue = 0
	for _, asset := range bolsa.Assets {
		bolsa.CurrentValue += asset.CurrentValue
	}

	// Actualizar información de progreso si hay un objetivo establecido
	if bolsa.Goal > 0 {
		// Calcular el porcentaje real de progreso
		rawPercent := (bolsa.CurrentValue / bolsa.Goal) * 100

		// Crear objeto de progreso si no existe
		if bolsa.Progress == nil {
			bolsa.Progress = &models.ProgressInfo{}
		}

		bolsa.Progress.RawPercent = rawPercent

		// Limitar el porcentaje mostrado a 100% si se superó el objetivo
		if rawPercent > 100 {
			bolsa.Progress.Percent = 100
			bolsa.Progress.Status = "superado"
			bolsa.Progress.ExcessAmount = bolsa.CurrentValue - bolsa.Goal
			bolsa.Progress.ExcessPercent = rawPercent - 100
		} else if rawPercent == 100 {
			bolsa.Progress.Percent = 100
			bolsa.Progress.Status = "completado"
		} else {
			bolsa.Progress.Percent = rawPercent
			bolsa.Progress.Status = "pendiente"
		}
	}

	return bolsa
}
