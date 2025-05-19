package middleware

import (
	"fmt"
	"log"
	"net/http"
	"strconv"
	"time"

	"github.com/AgusMolinaCode/DCA_Api.git/internal/database"
	"github.com/AgusMolinaCode/DCA_Api.git/internal/models"
	"github.com/AgusMolinaCode/DCA_Api.git/internal/repository"
	"github.com/gin-gonic/gin"
)

var cryptoRepo *repository.CryptoRepository

func InitCrypto() {
	cryptoRepo = repository.NewCryptoRepository(database.DB)
}

// GetInvestmentHistory obtiene el historial de valores de inversión
func GetInvestmentHistory(c *gin.Context) {
	userID := c.GetString("userId")

	// Obtener los minutos hacia atrás que queremos mostrar (por defecto 60 minutos)
	minutesStr := c.DefaultQuery("minutes", "60")
	minutes, err := strconv.Atoi(minutesStr)
	if err != nil || minutes <= 0 {
		minutes = 60 // Valor predeterminado: 60 minutos
	}

	// Calcular la fecha desde la que queremos los datos
	since := time.Now().Add(-time.Duration(minutes) * time.Minute)

	// Paso 1: Guardar o actualizar el snapshot actual
	// Obtener el valor actual de las inversiones
	holdingsRepo := repository.NewHoldingsRepository(database.DB)
	holdings, err := holdingsRepo.GetHoldings(userID)
	if err == nil && holdings.TotalCurrentValue > 0 {
		// Generar un ID único para el snapshot
		snapshotID := fmt.Sprintf("snapshot_%d", time.Now().UnixNano())

		// Truncar a intervalos de 5 minutos
		currentTime := time.Now()
		intervalSeconds := 5 * 60
		currentInterval := currentTime.Truncate(time.Duration(intervalSeconds) * time.Second)

		// Consultar si ya existe un snapshot para este intervalo
		queryExisting := `
			SELECT id, max_value, min_value 
			FROM investment_snapshots 
			WHERE user_id = ? AND 
			      date >= ? AND 
			      date < ?
			LIMIT 1
		`

		nextInterval := currentInterval.Add(time.Duration(intervalSeconds) * time.Second)

		var existingID string
		var maxValue, minValue float64

		errScan := database.DB.QueryRow(queryExisting, userID, currentInterval, nextInterval).Scan(
			&existingID, &maxValue, &minValue,
		)

		if errScan == nil && existingID != "" {
			// Ya existe un snapshot para este intervalo
			log.Printf("Ya existe un snapshot para este intervalo (ID: %s)", existingID)

			// Actualizar valores máximo y mínimo
			newMaxValue := maxValue
			newMinValue := minValue

			// Si el valor actual es mayor que el máximo, actualizar el máximo
			if holdings.TotalCurrentValue > maxValue {
				newMaxValue = holdings.TotalCurrentValue
				log.Printf("Nuevo valor máximo: %.2f (anterior: %.2f)", holdings.TotalCurrentValue, maxValue)
			}

			// Si el valor actual es menor que el mínimo, actualizar el mínimo
			if holdings.TotalCurrentValue < minValue {
				newMinValue = holdings.TotalCurrentValue
				log.Printf("Nuevo valor mínimo: %.2f (anterior: %.2f)", holdings.TotalCurrentValue, minValue)
			}

			// Actualizar el snapshot existente
			updateQuery := `
				UPDATE investment_snapshots 
				SET total_value = ?, total_invested = ?, profit = ?, profit_percentage = ?, max_value = ?, min_value = ? 
				WHERE id = ?
			`

			_, errUpdate := database.DB.Exec(
				updateQuery,
				holdings.TotalCurrentValue,
				holdings.TotalInvested,
				holdings.TotalProfit,
				holdings.ProfitPercentage,
				newMaxValue,
				newMinValue,
				existingID,
			)

			if errUpdate != nil {
				log.Printf("Error al actualizar snapshot: %v", errUpdate)
			}
		} else {
			// No existe un snapshot para este intervalo, crear uno nuevo
			insertQuery := `
				INSERT INTO investment_snapshots (id, user_id, date, total_value, total_invested, profit, profit_percentage, max_value, min_value)
				VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
			`

			_, errInsert := database.DB.Exec(
				insertQuery,
				snapshotID,
				userID,
				currentInterval,
				holdings.TotalCurrentValue,
				holdings.TotalInvested,
				holdings.TotalProfit,
				holdings.ProfitPercentage,
				holdings.TotalCurrentValue, // max_value inicial = valor actual
				holdings.TotalCurrentValue, // min_value inicial = valor actual
			)

			if errInsert != nil {
				log.Printf("Error al crear nuevo snapshot: %v", errInsert)
			}
		}
	}

	// Paso 2: Obtener todos los snapshots para mostrar
	querySnapshots := `
		SELECT id, user_id, date, total_value, total_invested, profit, profit_percentage, max_value, min_value
		FROM investment_snapshots
		WHERE user_id = ? AND date >= ?
		ORDER BY date ASC
	`

	rows, err := database.DB.Query(querySnapshots, userID, since)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Error al obtener el historial de inversiones: %v", err)})
		return
	}
	defer rows.Close()

	var snapshots []models.InvestmentSnapshot
	var labels []string
	var values []map[string]interface{}
	var maxValues []map[string]interface{}
	var minValues []map[string]interface{}

	for rows.Next() {
		var snapshot models.InvestmentSnapshot
		errScan := rows.Scan(
			&snapshot.ID,
			&snapshot.UserID,
			&snapshot.Date,
			&snapshot.TotalValue,
			&snapshot.TotalInvested,
			&snapshot.Profit,
			&snapshot.ProfitPercentage,
			&snapshot.MaxValue,
			&snapshot.MinValue,
		)
		if errScan != nil {
			log.Printf("Error al escanear snapshot: %v", errScan)
			continue
		}

		snapshots = append(snapshots, snapshot)
		// Formatear la fecha para el gráfico (formato dd/mm HH:MM)
		dateFormatted := snapshot.Date.Format("02/01 15:04")
		labels = append(labels, dateFormatted)
		
		// Crear objetos que contengan tanto la fecha como el valor
		values = append(values, map[string]interface{}{
			"fecha": dateFormatted,
			"valor": snapshot.TotalValue,
		})
		
		maxValues = append(maxValues, map[string]interface{}{
			"fecha": dateFormatted,
			"valor": snapshot.MaxValue,
		})
		
		minValues = append(minValues, map[string]interface{}{
			"fecha": dateFormatted,
			"valor": snapshot.MinValue,
		})
	}

	// Paso 3: Devolver la respuesta
	historyData := map[string]interface{}{
		"snapshots": snapshots,
		"labels":    labels,
		"values":    values,
		"max_values": maxValues,
		"min_values": minValues,
	}

	c.JSON(http.StatusOK, gin.H{"investment_history": historyData})
}

// Las funciones GetLiveBalance y DeleteInvestmentSnapshot se han movido a snapshot_handlers.go
