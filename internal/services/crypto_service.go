package services

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"strings"

	"github.com/AgusMolinaCode/DCA_Api.git/internal/models"
)

func GetCryptoPrice(ticker string) (*models.Welcome, error) {
	apiKey := os.Getenv("CRYPTO_API_KEY")
	url := fmt.Sprintf("https://min-api.cryptocompare.com/data/pricemultifull?fsyms=%s&tsyms=USD&api_key=%s",
		ticker, apiKey)

	resp, err := http.Get(url)
	if err != nil {
		log.Printf("Error haciendo la petición HTTP para %s: %v", ticker, err)
		return nil, fmt.Errorf("error en la petición HTTP: %v", err)
	}
	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		log.Printf("Error leyendo el cuerpo de la respuesta para %s: %v", ticker, err)
		return nil, fmt.Errorf("error leyendo respuesta: %v", err)
	}

	var result models.Welcome
	if err := json.Unmarshal(body, &result); err != nil {
		log.Printf("Error decodificando JSON para %s: %v", ticker, err)
		return nil, fmt.Errorf("error decodificando JSON: %v", err)
	}

	if _, exists := result.Raw[ticker]; !exists {
		log.Printf("No se encontraron datos para el ticker %s", ticker)
		return nil, fmt.Errorf("no se encontraron datos para %s", ticker)
	}

	return &result, nil
}

// GetMultipleCryptoPrices obtiene los precios actuales de múltiples criptomonedas en una sola llamada a la API
func GetMultipleCryptoPrices(tickers []string) (map[string]float64, error) {
	if len(tickers) == 0 {
		return nil, fmt.Errorf("no se proporcionaron tickers")
	}

	// Unir los tickers en una cadena separada por comas
	tickersStr := strings.Join(tickers, ",")

	// Construir la URL de la API
	apiKey := os.Getenv("CRYPTO_API_KEY")
	url := fmt.Sprintf("https://min-api.cryptocompare.com/data/pricemulti?fsyms=%s&tsyms=USD&api_key=%s",
		tickersStr, apiKey)

	// Realizar la petición HTTP
	resp, err := http.Get(url)
	if err != nil {
		log.Printf("Error haciendo la petición HTTP para múltiples tickers: %v", err)
		return nil, fmt.Errorf("error en la petición HTTP: %v", err)
	}
	defer resp.Body.Close()

	// Leer el cuerpo de la respuesta
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		log.Printf("Error leyendo el cuerpo de la respuesta para múltiples tickers: %v", err)
		return nil, fmt.Errorf("error leyendo respuesta: %v", err)
	}

	// Decodificar la respuesta JSON
	var result map[string]map[string]float64
	if err := json.Unmarshal(body, &result); err != nil {
		log.Printf("Error decodificando JSON para múltiples tickers: %v", err)
		return nil, fmt.Errorf("error decodificando JSON: %v", err)
	}

	// Extraer los precios en USD
	prices := make(map[string]float64)
	for ticker, data := range result {
		if usdPrice, exists := data["USD"]; exists {
			prices[ticker] = usdPrice
		}
	}

	// Verificar que obtuvimos al menos un precio
	if len(prices) == 0 {
		return nil, fmt.Errorf("no se encontraron precios para los tickers proporcionados")
	}

	// Registrar los precios obtenidos para depuración
	log.Printf("Precios obtenidos para %d criptomonedas:", len(prices))
	for ticker, price := range prices {
		log.Printf("  - %s: %.2f USD", ticker, price)
	}

	return prices, nil
}

func GetCryptoImageURL(ticker string) (string, error) {
	// Intentar obtener todos los datos de la criptomoneda, que incluyen la URL de la imagen
	cryptoData, err := GetCryptoPrice(ticker)
	if err != nil {
		return "", err
	}

	// Verificar si existe la información del ticker
	if _, exists := cryptoData.Raw[ticker]; !exists {
		return "", fmt.Errorf("no se encontraron datos para %s", ticker)
	}

	// Obtener la URL de la imagen
	imageURL := cryptoData.Raw[ticker]["USD"].IMAGEURL

	// Si la URL está vacía, construir una URL por defecto usando el servicio de CryptoCompare
	if imageURL == "" {
		imageURL = fmt.Sprintf("https://www.cryptocompare.com/media/37746251/%s.png", strings.ToLower(ticker))
	} else {
		// Asegurarse de que la URL sea completa
		if !strings.HasPrefix(imageURL, "http") {
			imageURL = "https://www.cryptocompare.com" + imageURL
		}
	}

	return imageURL, nil
}
