package services

import (
	"database/sql"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/AgusMolinaCode/DCA_Api.git/internal/database"
	"github.com/AgusMolinaCode/DCA_Api.git/internal/models"
)

// RepositoryInterface define las operaciones que necesitamos del repositorio
type CryptoRepositoryInterface interface {
	SaveInvestmentSnapshot(userID string, totalValue, totalInvested, profit, profitPercentage float64) error
}

type HoldingsRepositoryInterface interface {
	GetHoldings(userID string) (*models.Holdings, error)
}

// PriceUpdater es un servicio que actualiza los precios de las criptomonedas periódicamente
type PriceUpdater struct {
	interval      time.Duration
	cryptoRepo    CryptoRepositoryInterface
	holdingsRepo  HoldingsRepositoryInterface
	isRunning     bool
	stopChan      chan struct{}
	mutex         sync.Mutex
	lastUpdated   time.Time
	cachedResults map[string]interface{}
}

// NewPriceUpdater crea un nuevo servicio de actualización de precios
func NewPriceUpdater(interval time.Duration) *PriceUpdater {
	// Aquí usamos la implementación concreta, pero a través de la interfaz
	return &PriceUpdater{
		interval:      interval,
		cryptoRepo:    createCryptoRepository(),
		holdingsRepo:  createHoldingsRepository(),
		isRunning:     false,
		stopChan:      make(chan struct{}),
		cachedResults: make(map[string]interface{}),
	}
}

// Funciones auxiliares para crear los repositorios
func createCryptoRepository() CryptoRepositoryInterface {
	// Implementación que crea el repositorio concreto
	return &cryptoRepositoryAdapter{db: database.DB}
}

func createHoldingsRepository() HoldingsRepositoryInterface {
	// Implementación que crea el repositorio concreto
	return &holdingsRepositoryAdapter{db: database.DB}
}

// Adaptadores para los repositorios
type cryptoRepositoryAdapter struct {
	db *sql.DB
}

func (a *cryptoRepositoryAdapter) SaveInvestmentSnapshot(userID string, totalValue, totalInvested, profit, profitPercentage float64) error {
	// Verificar que los valores sean válidos
	if totalValue <= 0 || totalInvested <= 0 {
		log.Printf("No se guardó el snapshot porque los valores no son válidos: totalValue=%f, totalInvested=%f", totalValue, totalInvested)
		return nil // No guardamos snapshots con valores inválidos
	}

	// Obtener la fecha actual
	currentTime := time.Now()

	// Verificar si ya existe un snapshot para el día actual
	currentDateStr := currentTime.Format("2006-01-02")

	// Consultar si existe un snapshot para la fecha actual
	query := `
		SELECT id, total_value FROM investment_snapshots 
		WHERE user_id = ? AND date(date) = date(?)
		ORDER BY total_value DESC LIMIT 1
	`

	var existingID string
	var existingValue float64
	err := a.db.QueryRow(query, userID, currentTime).Scan(&existingID, &existingValue)

	// Si existe un snapshot para hoy, verificar si el valor actual es mayor
	if err == nil {
		// Si el valor existente es mayor o igual al nuevo valor, no actualizamos
		if existingValue >= totalValue {
			log.Printf("No se actualizó el snapshot porque el valor existente (%f) es mayor o igual al nuevo valor (%f)", existingValue, totalValue)
			return nil
		}

		log.Printf("Actualizando snapshot para hoy (%s) con ID %s, nuevo valor máximo: %f (anterior: %f)", currentDateStr, existingID, totalValue, existingValue)

		updateQuery := `
			UPDATE investment_snapshots 
			SET total_value = ?, total_invested = ?, profit = ?, profit_percentage = ?, date = ?
			WHERE id = ?
		`

		_, err := a.db.Exec(
			updateQuery,
			totalValue,
			totalInvested,
			profit,
			profitPercentage,
			currentTime,
			existingID,
		)

		if err != nil {
			log.Printf("Error al actualizar snapshot existente: %v", err)
			return err
		}

		log.Printf("Snapshot actualizado exitosamente para la fecha %s con nuevo valor máximo: %f", currentDateStr, totalValue)
		return nil
	}

	// Si no existe un snapshot para hoy o hubo un error, crear uno nuevo
	// Generar un ID único para el snapshot
	id := fmt.Sprintf("snapshot_%d", time.Now().UnixNano())

	// Registrar los valores que se van a guardar
	log.Printf("Guardando nuevo snapshot para usuario %s en fecha %s: totalValue=%f, totalInvested=%f, profit=%f, profitPercentage=%f",
		userID, currentDateStr, totalValue, totalInvested, profit, profitPercentage)

	// Insertar el snapshot en la base de datos
	insertQuery := `
		INSERT INTO investment_snapshots (
			id, user_id, date, total_value, total_invested, profit, profit_percentage
		) VALUES (?, ?, ?, ?, ?, ?, ?)
	`

	_, err = a.db.Exec(
		insertQuery,
		id,
		userID,
		currentTime,
		totalValue,
		totalInvested,
		profit,
		profitPercentage,
	)

	if err != nil {
		log.Printf("Error al guardar nuevo snapshot: %v", err)
	} else {
		log.Printf("Nuevo snapshot guardado exitosamente con ID: %s para la fecha %s con valor inicial: %f", id, currentDateStr, totalValue)
	}

	return err
}

type holdingsRepositoryAdapter struct {
	db *sql.DB
}

func (a *holdingsRepositoryAdapter) GetHoldings(userID string) (*models.Holdings, error) {
	// Obtener todas las transacciones del usuario
	transactionsQuery := `
		SELECT id, user_id, ticker, type, amount, purchase_price, total, date, usdt_received
		FROM transactions
		WHERE user_id = ?
		ORDER BY date DESC
	`

	rows, err := a.db.Query(transactionsQuery, userID)
	if err != nil {
		log.Printf("Error al obtener transacciones: %v", err)
		return nil, err
	}
	defer rows.Close()

	// Mapas para almacenar los totales por criptomoneda
	type tempHolding struct {
		Ticker       string
		Amount       float64
		Invested     float64
		CurrentPrice float64
		CurrentValue float64
		Color        string
	}

	holdingsMap := make(map[string]*tempHolding)
	totalInvested := 0.0
	totalCurrentValue := 0.0

	// Procesar cada transacción
	for rows.Next() {
		var tx models.CryptoTransaction
		err := rows.Scan(
			&tx.ID, &tx.UserID, &tx.Ticker, &tx.Type, &tx.Amount,
			&tx.PurchasePrice, &tx.Total, &tx.Date, &tx.USDTReceived,
		)
		if err != nil {
			log.Printf("Error al escanear transacción: %v", err)
			continue
		}

		// Actualizar los holdings según el tipo de transacción
		if tx.Type == "buy" {
			// Si es una compra, agregar al holding
			if holding, exists := holdingsMap[tx.Ticker]; exists {
				// Si ya existe el holding, actualizar
				holding.Amount += tx.Amount
				holding.Invested += tx.Total
			} else {
				// Si no existe, crear nuevo holding
				holdingsMap[tx.Ticker] = &tempHolding{
					Ticker:   tx.Ticker,
					Amount:   tx.Amount,
					Invested: tx.Total,
				}
			}
			totalInvested += tx.Total
		} else if tx.Type == "sell" {
			// Si es una venta, reducir el holding
			if holding, exists := holdingsMap[tx.Ticker]; exists {
				// Calcular la proporción vendida y reducir la inversión proporcionalmente
				if holding.Amount > 0 {
					proportion := tx.Amount / holding.Amount
					reducedInvestment := holding.Invested * proportion
					holding.Amount -= tx.Amount
					holding.Invested -= reducedInvestment
					totalInvested -= reducedInvestment
				}
			}
		}
	}

	// Obtener los precios actuales y calcular el valor actual
	var holdings []*models.CryptoWeight
	colors := []string{"#FF6384", "#36A2EB", "#FFCE56", "#4BC0C0", "#9966FF", "#FF9F40"}
	colorIndex := 0

	for ticker, holding := range holdingsMap {
		if holding.Amount <= 0 {
			continue // Ignorar holdings con cantidad cero o negativa
		}

		// Obtener el precio actual
		cryptoData, err := GetCryptoPriceFromCoinGecko(ticker)
		if err != nil {
			// Si hay error, usar el último precio conocido o un valor por defecto
			holding.CurrentPrice = holding.Invested / holding.Amount // Precio promedio de compra
		} else {
			holding.CurrentPrice = cryptoData.Price
		}

		// Calcular el valor actual
		holding.CurrentValue = holding.Amount * holding.CurrentPrice
		totalCurrentValue += holding.CurrentValue

		// Asignar un color
		holding.Color = colors[colorIndex%len(colors)]
		colorIndex++

		// Crear el objeto CryptoWeight
		weight := &models.CryptoWeight{
			Ticker: holding.Ticker,
			Name:   holding.Ticker, // Usar ticker como nombre por defecto
			Value:  holding.CurrentValue,
			Color:  holding.Color,
		}

		holdings = append(holdings, weight)
	}

	// Calcular ganancias/pérdidas
	totalProfit := totalCurrentValue - totalInvested
	profitPercentage := 0.0
	if totalInvested > 0 {
		profitPercentage = (totalProfit / totalInvested) * 100
	}

	// Preparar datos para el gráfico
	var labels []string
	var values []float64
	var chartColors []string

	for _, holding := range holdings {
		labels = append(labels, holding.Ticker)
		values = append(values, holding.Value)
		chartColors = append(chartColors, holding.Color)
	}

	// Convertir slice de punteros a slice de valores
	distribution := make([]models.CryptoWeight, len(holdings))
	for i, h := range holdings {
		distribution[i] = *h
	}

	// Crear el objeto de respuesta
	result := &models.Holdings{
		TotalCurrentValue: totalCurrentValue,
		TotalInvested:     totalInvested,
		TotalProfit:       totalProfit,
		ProfitPercentage:  profitPercentage,
		Distribution:      distribution,
		ChartData: models.PieChartData{
			Labels:   labels,
			Values:   values,
			Colors:   chartColors,
			Currency: "USD",
		},
	}

	return result, nil
}

// Start inicia el servicio de actualización de precios
func (p *PriceUpdater) Start() {
	p.mutex.Lock()
	defer p.mutex.Unlock()

	if p.isRunning {
		return
	}

	p.isRunning = true
	p.stopChan = make(chan struct{})

	go func() {
		ticker := time.NewTicker(p.interval)
		defer ticker.Stop()

		// Actualizar inmediatamente al iniciar
		p.updateAllUserBalances()

		for {
			select {
			case <-ticker.C:
				p.updateAllUserBalances()
			case <-p.stopChan:
				return
			}
		}
	}()

	log.Printf("Servicio de actualización de precios iniciado con intervalo de %v", p.interval)
}

// Stop detiene el servicio de actualización de precios
func (p *PriceUpdater) Stop() {
	p.mutex.Lock()
	defer p.mutex.Unlock()

	if !p.isRunning {
		return
	}

	p.isRunning = false
	close(p.stopChan)
	log.Printf("Servicio de actualización de precios detenido")
}

// updateAllUserBalances actualiza los balances de todos los usuarios
func (p *PriceUpdater) updateAllUserBalances() {
	// Obtener todos los usuarios
	users, err := p.getAllUsers()
	if err != nil {
		log.Printf("Error al obtener usuarios: %v", err)
		return
	}

	// Para cada usuario, actualizar su balance
	for _, userID := range users {
		p.updateUserBalance(userID)
	}

	p.lastUpdated = time.Now()
	log.Printf("Actualización de precios completada para %d usuarios", len(users))
}

// updateUserBalance actualiza el balance de un usuario específico
func (p *PriceUpdater) updateUserBalance(userID string) {
	// Obtener las tenencias actualizadas del usuario
	holdings, err := p.holdingsRepo.GetHoldings(userID)
	if err != nil {
		log.Printf("Error al obtener tenencias para usuario %s: %v", userID, err)
		return
	}

	// Guardar los resultados en caché
	p.mutex.Lock()
	p.cachedResults[userID] = holdings
	p.mutex.Unlock()

	// Guardar un snapshot si el valor es mayor que el máximo del día
	err = p.cryptoRepo.SaveInvestmentSnapshot(
		userID,
		holdings.TotalCurrentValue,
		holdings.TotalInvested,
		holdings.TotalProfit,
		holdings.ProfitPercentage,
	)

	if err != nil {
		log.Printf("Error al guardar snapshot para usuario %s: %v", userID, err)
	}
}

// GetCachedBalance obtiene el balance en caché para un usuario
func (p *PriceUpdater) GetCachedBalance(userID string) (interface{}, bool) {
	p.mutex.Lock()
	defer p.mutex.Unlock()

	result, exists := p.cachedResults[userID]
	return result, exists
}

// GetLastUpdated obtiene la última vez que se actualizaron los precios
func (p *PriceUpdater) GetLastUpdated() time.Time {
	p.mutex.Lock()
	defer p.mutex.Unlock()

	return p.lastUpdated
}

// getAllUsers obtiene todos los IDs de usuarios en el sistema
func (p *PriceUpdater) getAllUsers() ([]string, error) {
	query := `SELECT id FROM users`
	rows, err := database.DB.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var users []string
	for rows.Next() {
		var userID sql.NullString
		if err := rows.Scan(&userID); err != nil {
			return nil, err
		}
		if userID.Valid {
			users = append(users, userID.String)
		}
	}

	return users, nil
}
