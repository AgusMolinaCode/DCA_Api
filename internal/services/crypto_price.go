package services

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"time"
)

// CryptoData contiene la informaciu00f3n de precio de una criptomoneda
type CryptoData struct {
	Price       float64 `json:"price"`
	MarketCap   float64 `json:"market_cap"`
	Volume24h   float64 `json:"volume_24h"`
	Change24h   float64 `json:"change_24h"`
	LastUpdated string  `json:"last_updated"`
}

// Cachu00e9 para almacenar precios y reducir llamadas a la API
var priceCache = make(map[string]cachedPrice)

type cachedPrice struct {
	Data      CryptoData
	Timestamp time.Time
}

// GetCryptoPriceFromCoinGecko obtiene el precio actual de una criptomoneda desde CoinGecko
func GetCryptoPriceFromCoinGecko(ticker string) (CryptoData, error) {
	// Verificar si tenemos el precio en cachu00e9 y si es reciente (menos de 5 minutos)
	if cached, exists := priceCache[ticker]; exists {
		if time.Since(cached.Timestamp) < 5*time.Minute {
			return cached.Data, nil
		}
	}

	// Construir la URL de la API
	url := fmt.Sprintf("https://api.coingecko.com/api/v3/simple/price?ids=%s&vs_currencies=usd&include_market_cap=true&include_24hr_vol=true&include_24hr_change=true&include_last_updated_at=true", ticker)

	// Realizar la solicitud HTTP
	resp, err := http.Get(url)
	if err != nil {
		log.Printf("Error al obtener precio de %s: %v", ticker, err)
		return CryptoData{}, err
	}
	defer resp.Body.Close()

	// Leer el cuerpo de la respuesta
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Printf("Error al leer respuesta para %s: %v", ticker, err)
		return CryptoData{}, err
	}

	// Parsear la respuesta JSON
	var result map[string]map[string]interface{}
	if err := json.Unmarshal(body, &result); err != nil {
		log.Printf("Error al parsear JSON para %s: %v", ticker, err)
		return CryptoData{}, err
	}

	// Extraer los datos
	tokenData, exists := result[ticker]
	if !exists {
		return CryptoData{}, fmt.Errorf("no se encontraron datos para %s", ticker)
	}

	// Crear el objeto CryptoData
	data := CryptoData{
		Price:       getFloat(tokenData, "usd"),
		MarketCap:   getFloat(tokenData, "usd_market_cap"),
		Volume24h:   getFloat(tokenData, "usd_24h_vol"),
		Change24h:   getFloat(tokenData, "usd_24h_change"),
		LastUpdated: time.Unix(int64(getFloat(tokenData, "last_updated_at")), 0).Format(time.RFC3339),
	}

	// Guardar en cachu00e9
	priceCache[ticker] = cachedPrice{
		Data:      data,
		Timestamp: time.Now(),
	}

	return data, nil
}

// getFloat extrae un valor float64 de un mapa
func getFloat(data map[string]interface{}, key string) float64 {
	if val, exists := data[key]; exists {
		switch v := val.(type) {
		case float64:
			return v
		case int:
			return float64(v)
		case int64:
			return float64(v)
		case string:
			var f float64
			fmt.Sscanf(v, "%f", &f)
			return f
		}
	}
	return 0
}
