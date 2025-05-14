package middleware

import (
	"github.com/AgusMolinaCode/DCA_Api.git/internal/services"
)

// Variable global para almacenar la instancia del actualizador de precios
var priceUpdaterInstance *services.PriceUpdater

// SetPriceUpdater establece la instancia del actualizador de precios
func SetPriceUpdater(updater *services.PriceUpdater) {
	priceUpdaterInstance = updater
}

// GetPriceUpdater obtiene la instancia del actualizador de precios
func GetPriceUpdater() *services.PriceUpdater {
	return priceUpdaterInstance
}
