package services

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"

	"github.com/AgusMolinaCode/DCA_Api.git/internal/models"
)

func GetCryptoPrice(ticker string) (*models.Welcome, error) {
	apiKey := os.Getenv("CRYPTO_API_KEY")
	url := fmt.Sprintf("https://min-api.cryptocompare.com/data/pricemultifull?fsyms=%s&tsyms=USD&api_key=%s",
		ticker, apiKey)

	log.Printf("Consultando API para %s: %s", ticker, url)

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
