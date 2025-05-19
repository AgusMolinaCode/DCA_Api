package repository

import (
	"database/sql"
	"fmt"
	"time"

	"github.com/AgusMolinaCode/DCA_Api.git/internal/models"
)

// GetUserInvestmentHistory obtiene el historial de inversiones del usuario desde una fecha específica
// Esta función es un wrapper para GetInvestmentHistorySince que permite su uso desde los handlers
func GetUserInvestmentHistory(db *sql.DB, userID string, startDate time.Time) (models.InvestmentHistory, error) {
	// Crear una instancia del repositorio de criptomonedas
	repo := NewCryptoRepository(db)
	
	// Si la fecha de inicio es la fecha cero, usar GetInvestmentHistoryFromSnapshots
	if startDate.IsZero() {
		return repo.GetInvestmentHistoryFromSnapshots(userID)
	}
	
	// Obtener los snapshots desde la fecha especificada
	snapshots, err := repo.GetInvestmentHistorySince(userID, startDate)
	if err != nil {
		return models.InvestmentHistory{}, fmt.Errorf("error al obtener el historial de inversiones: %v", err)
	}
	
	// Si no hay snapshots, devolver un historial vacío
	if len(snapshots) == 0 {
		return models.InvestmentHistory{
			StartDate:       startDate,
			History:         []models.DailyValue{},
			TrendPercentage: 0,
		}, nil
	}
	
	// Crear historial
	history := models.InvestmentHistory{
		StartDate: startDate,
		History:   make([]models.DailyValue, len(snapshots)),
	}
	
	// Llenar el historial con los datos de los snapshots
	for i, snapshot := range snapshots {
		history.History[i] = models.DailyValue{
			Date:             snapshot.Date.Format("2006-01-02"),
			TotalValue:       snapshot.TotalValue,
			ChangePercentage: snapshot.ProfitPercentage,
		}
	}
	
	// Calcular tendencia general (porcentaje de cambio desde el primer snapshot hasta el último)
	if len(snapshots) > 1 {
		firstValue := snapshots[0].TotalValue
		lastValue := snapshots[len(snapshots)-1].TotalValue
		
		if firstValue > 0 {
			history.TrendPercentage = ((lastValue - firstValue) / firstValue) * 100
		}
	}
	
	return history, nil
}

// GetUserLiveBalance obtiene el balance en tiempo real del usuario
// Esta función calcula el balance actual utilizando el dashboard
func GetUserLiveBalance(db *sql.DB, userID string) (*models.Balance, error) {
	// Crear una instancia del repositorio de criptomonedas
	repo := NewCryptoRepository(db)
	
	// Obtener el dashboard que contiene las tenencias actualizadas
	dashboard, err := repo.GetCryptoDashboard(userID)
	if err != nil {
		return nil, fmt.Errorf("error al obtener el balance en tiempo real: %v", err)
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
		TotalBalance:     totalBalance,
		TotalInvested:    totalInvested,
		TotalProfit:      totalProfit,
		ProfitPercentage: profitPercentage,
		LastUpdated:      time.Now(),
	}
	
	return balance, nil
}
