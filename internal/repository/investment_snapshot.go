package repository

import (
	"database/sql"
	"fmt"
	"log"
	"time"

	"github.com/AgusMolinaCode/DCA_Api.git/internal/models"
)

// SaveInvestmentSnapshotWithMaxMin guarda un snapshot de inversión con valores máximo y mínimo
func (r *CryptoRepository) SaveInvestmentSnapshotWithMaxMin(userID string, totalValue, totalInvested, profit, profitPercentage float64) error {
	// Verificar que los valores sean válidos
	if totalValue <= 0 || totalInvested <= 0 {
		log.Printf("No se guardó el snapshot porque los valores no son válidos: totalValue=%f, totalInvested=%f", totalValue, totalInvested)
		return nil
	}

	// Obtener la fecha actual y truncarla al intervalo diario (24 horas)
	currentTime := time.Now()
	// Truncar al inicio del día (00:00:00)
	currentInterval := time.Date(currentTime.Year(), currentTime.Month(), currentTime.Day(), 0, 0, 0, 0, currentTime.Location())
	// Calcular el siguiente día
	nextInterval := currentInterval.AddDate(0, 0, 1)

	// Formatear para mostrar en logs
	intervalStr := currentInterval.Format("2006-01-02 15:04")
	log.Printf("=== Procesando snapshot para intervalo %s con valor: %.2f ===", intervalStr, totalValue)

	// 1. Verificar si ya existe un snapshot para este intervalo
	existingQuery := `
		SELECT id, max_value, min_value 
		FROM investment_snapshots 
		WHERE user_id = $1 AND 
		      date >= $2 AND 
		      date < $3
		LIMIT 1
	`

	var existingID string
	var maxValue, minValue float64

	err := r.db.QueryRow(existingQuery, userID, currentInterval, nextInterval).Scan(
		&existingID, &maxValue, &minValue,
	)

	// Generar un ID único para el snapshot
	snapshotID := fmt.Sprintf("snapshot_%d", time.Now().UnixNano())

	if err == nil {
		// Ya existe un snapshot para este intervalo
		log.Printf("Encontrado snapshot existente (ID: %s) con max: %.2f, min: %.2f", 
			existingID, maxValue, minValue)

		// Actualizar valores máximo y mínimo
		newMaxValue := maxValue
		newMinValue := minValue

		// Si el valor actual es mayor que el máximo, actualizar el máximo
		if totalValue > maxValue {
			newMaxValue = totalValue
			log.Printf("Nuevo valor máximo: %.2f (anterior: %.2f)", totalValue, maxValue)
		}

		// Si el valor actual es menor que el mínimo, actualizar el mínimo
		if totalValue < minValue {
			newMinValue = totalValue
			log.Printf("Nuevo valor mínimo: %.2f (anterior: %.2f)", totalValue, minValue)
		}

		// Eliminar el snapshot existente
		_, err = r.db.Exec("DELETE FROM investment_snapshots WHERE id = $1", existingID)
		if err != nil {
			log.Printf("Error al eliminar snapshot existente: %v", err)
			return err
		}

		// Insertar un nuevo snapshot con los valores actualizados
		insertQuery := `
			INSERT INTO investment_snapshots (id, user_id, date, total_value, total_invested, profit, profit_percentage, max_value, min_value)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		`

		_, err = r.db.Exec(
			insertQuery,
			snapshotID,
			userID,
			currentInterval, // Usar el inicio del intervalo para consistencia
			totalValue,
			totalInvested,
			profit,
			profitPercentage,
			newMaxValue,
			newMinValue,
		)

		log.Printf("Creado nuevo snapshot (ID: %s) con valor: %.2f, max: %.2f, min: %.2f", 
			snapshotID, totalValue, newMaxValue, newMinValue)
	} else if err == sql.ErrNoRows {
		// No existe un snapshot para este intervalo, crear uno nuevo
		log.Printf("No existe snapshot para el intervalo %s, creando uno nuevo", intervalStr)

		// Para un nuevo snapshot, el valor máximo y mínimo son iguales al valor actual
		insertQuery := `
			INSERT INTO investment_snapshots (id, user_id, date, total_value, total_invested, profit, profit_percentage, max_value, min_value)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		`

		_, err = r.db.Exec(
			insertQuery,
			snapshotID,
			userID,
			currentInterval, // Usar el inicio del intervalo para consistencia
			totalValue,
			totalInvested,
			profit,
			profitPercentage,
			totalValue, // max_value = valor actual
			totalValue, // min_value = valor actual
		)

		log.Printf("Creado primer snapshot (ID: %s) para el intervalo con valor: %.2f", 
			snapshotID, totalValue)
	} else {
		// Error al consultar
		log.Printf("Error al verificar snapshot existente: %v", err)
		return err
	}

	return err
}

// GetInvestmentSnapshotsWithMaxMin obtiene los snapshots de inversión con valores máximo y mínimo
func (r *CryptoRepository) GetInvestmentSnapshotsWithMaxMin(userID string, since time.Time) ([]models.InvestmentSnapshot, error) {
	query := `
		SELECT id, user_id, date, total_value, total_invested, profit, profit_percentage, max_value, min_value
		FROM investment_snapshots
		WHERE user_id = $1 AND date >= $2
		ORDER BY date ASC
	`

	rows, err := r.db.Query(query, userID, since)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var snapshots []models.InvestmentSnapshot

	for rows.Next() {
		var snapshot models.InvestmentSnapshot
		err := rows.Scan(
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
		if err != nil {
			return nil, err
		}

		snapshots = append(snapshots, snapshot)
	}

	if err = rows.Err(); err != nil {
		return nil, err
	}

	return snapshots, nil
}