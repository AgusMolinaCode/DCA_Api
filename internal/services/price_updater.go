package services

import (
	"database/sql"
	"fmt"
	"log"
	"sort"
	"sync"
	"time"

	"github.com/AgusMolinaCode/DCA_Api.git/internal/database"
	"github.com/AgusMolinaCode/DCA_Api.git/internal/models"
)

// RepositoryInterface define las operaciones que necesitamos del repositorio
type CryptoRepositoryInterface interface {
	SaveInvestmentSnapshot(userID string, totalValue, totalInvested, profit, profitPercentage float64) error
	GetInvestmentHistory(userID string, limit int) ([]models.InvestmentSnapshot, error)
	GetInvestmentHistorySince(userID string, since time.Time) ([]models.InvestmentSnapshot, error)
}

type HoldingsRepositoryInterface interface {
	GetHoldings(userID string) (*models.Holdings, error)
}

// userBalance almacena el balance de un usuario
type userBalance struct {
	totalValue    float64
	totalInvested float64
	profit        float64
	profitPct     float64
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
	userBalances  sync.Map // Almacena userBalance por userID
}

// NewPriceUpdater crea un nuevo servicio de actualización de precios
// El parámetro interval ya no se usa, se mantiene por compatibilidad
func NewPriceUpdater(interval time.Duration) *PriceUpdater {
	// Ignoramos el intervalo que nos pasan y usamos 1 minuto fijo
	return &PriceUpdater{
		interval:      time.Minute, // Siempre 1 minuto
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
		return nil
	}

	// Generar un ID único para el snapshot
	snapshotID := fmt.Sprintf("snapshot_%d", time.Now().UnixNano())

	// Obtener la fecha actual y formatearla para el día actual
	currentTime := time.Now()
	currentDay := currentTime.Truncate(24 * time.Hour) // Truncar a día completo
	dayStr := currentDay.Format("2006-01-02")

	// Registrar la operación para depuración
	log.Printf("=== INICIO SaveInvestmentSnapshot para usuario %s a las %s ===", userID, currentTime.Format("2006-01-02 15:04:05"))
	log.Printf("Intentando guardar snapshot para el día %s con valor: %.2f", dayStr, totalValue)

	// Verificar si ya existe un snapshot para este día
	existingQuery := `
		SELECT id, total_value, date FROM investment_snapshots
		WHERE user_id = ? AND 
		      date >= ? AND 
		      date < datetime(?, '+1 day')
		ORDER BY total_value DESC
		LIMIT 1
	`

	var existingID string
	var existingValue float64
	var existingTime time.Time

	err := a.db.QueryRow(existingQuery, userID, currentDay, currentDay).Scan(
		&existingID, &existingValue, &existingTime,
	)

	if err == nil {
		// Ya existe un snapshot para este día
		existingTimeStr := existingTime.Format("2006-01-02 15:04:05")
		log.Printf("Ya existe un snapshot para este día (ID: %s, Hora: %s) con valor: %.2f", 
			existingID, existingTimeStr, existingValue)
		
		if totalValue <= existingValue {
			// El valor actual no es mayor, no hacemos nada
			log.Printf("No se actualiza porque el valor actual (%.2f) no es mayor que el existente (%.2f)", 
				totalValue, existingValue)
			return nil
		}

		// El valor actual es mayor, actualizamos el snapshot existente
		log.Printf("Actualizando snapshot existente (ID: %s) porque el valor actual (%.2f) es mayor que el existente (%.2f)", 
			existingID, totalValue, existingValue)
		
		updateQuery := `
			UPDATE investment_snapshots
			SET total_value = ?, total_invested = ?, profit = ?, profit_percentage = ?, date = ?
			WHERE id = ?
		`

		_, err = a.db.Exec(
			updateQuery,
			totalValue,
			totalInvested,
			profit,
			profitPercentage,
			currentTime, // Mantenemos la hora exacta de la actualización
			existingID,
		)

		if err != nil {
			log.Printf("Error al actualizar snapshot existente: %v", err)
		} else {
			log.Printf("Snapshot actualizado exitosamente para el día %s con nuevo valor %.2f", 
				dayStr, totalValue)
		}

		return err
	} else if err != sql.ErrNoRows {
		// Error al consultar
		log.Printf("Error al verificar snapshot existente: %v", err)
		return err
	}

	// No existe un snapshot para este día, insertamos uno nuevo
	log.Printf("Creando nuevo snapshot para el día %s con valor %.2f", dayStr, totalValue)
	
	insertQuery := `
		INSERT INTO investment_snapshots (id, user_id, date, total_value, total_invested, profit, profit_percentage)
		VALUES (?, ?, ?, ?, ?, ?, ?)
	`

	_, err = a.db.Exec(
		insertQuery,
		snapshotID,
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
		log.Printf("Nuevo snapshot creado exitosamente para el día %s con valor %.2f", dayStr, totalValue)
	}

	return err
}

// GetInvestmentHistory obtiene el historial de inversiones de un usuario
func (a *cryptoRepositoryAdapter) GetInvestmentHistory(userID string, limit int) ([]models.InvestmentSnapshot, error) {
	// Si el límite es 0 o negativo, usamos un valor predeterminado
	if limit <= 0 {
		limit = 100 // Valor predeterminado
	}

	// Consulta para obtener todos los snapshots ordenados por fecha
	query := `
		SELECT id, user_id, date, total_value, total_invested, profit, profit_percentage
		FROM investment_snapshots
		WHERE user_id = ?
		ORDER BY date ASC
		LIMIT ?
	`

	rows, err := a.db.Query(query, userID, limit)
	if err != nil {
		log.Printf("Error al obtener historial de inversiones: %v", err)
		return nil, err
	}
	defer rows.Close()

	// Procesar los resultados
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
		)
		if err != nil {
			log.Printf("Error al escanear snapshot: %v", err)
			continue
		}
		snapshots = append(snapshots, snapshot)
	}

	return snapshots, nil
}

// GetInvestmentHistorySince obtiene los snapshots desde una fecha específica
func (a *cryptoRepositoryAdapter) GetInvestmentHistorySince(userID string, since time.Time) ([]models.InvestmentSnapshot, error) {
	query := `
		SELECT id, user_id, date, total_value, total_invested, profit, profit_percentage
		FROM investment_snapshots
		WHERE user_id = ? AND date >= ?
		ORDER BY date ASC
	`

	rows, err := a.db.Query(query, userID, since)
	if err != nil {
		log.Printf("Error al obtener historial de inversiones: %v", err)
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
		)
		if err != nil {
			log.Printf("Error al escanear snapshot: %v", err)
			continue
		}
		snapshots = append(snapshots, snapshot)
	}

	return snapshots, nil
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
// Guarda un snapshot exactamente al inicio de cada día
func (p *PriceUpdater) Start() {
	p.mutex.Lock()
	defer p.mutex.Unlock()

	if p.isRunning {
		log.Println("El servicio de actualización de precios ya está en ejecución")
		return
	}

	p.isRunning = true
	p.stopChan = make(chan struct{})

	go func() {
		// Configurar el logger para incluir la hora exacta
		log.SetFlags(log.LstdFlags | log.Lmicroseconds)
		log.Println("=== INICIANDO SERVICIO DE ACTUALIZACIÓN DE PRECIOS ===")

		// Calcular cuánto tiempo falta para el próximo día a las 00:00
		now := time.Now()
		tomorrow := now.Truncate(24*time.Hour).Add(24 * time.Hour) // Siguiente día a las 00:00
		initialDelay := tomorrow.Sub(now)

		log.Printf("Próximo snapshot diario programado en %v horas (a las %s)", 
			initialDelay.Hours(), tomorrow.Format("2006-01-02 15:04:05"))

		// Esperar hasta el próximo día a las 00:00 para comenzar
		select {
		case <-time.After(initialDelay):
			log.Printf("Iniciando ciclo de actualizaciones diarias a las %s", 
				time.Now().Format("2006-01-02 15:04:05.000"))
		case <-p.stopChan:
			log.Println("Servicio detenido antes de iniciar")
			p.isRunning = false
			return
		}

		// Crear un ticker que se dispare exactamente cada día
		ticker := time.NewTicker(24 * time.Hour)
		defer ticker.Stop()

		// Ticker más frecuente para actualizar los valores máximos (cada minuto)
		updateTicker := time.NewTicker(time.Minute)
		defer updateTicker.Stop()

		// Mapa para almacenar los valores máximos del minuto actual por usuario
		currentMaxValues := make(map[string]float64) // [userID] = maxValue
		currentInvested := make(map[string]float64)  // [userID] = totalInvested
		currentProfit := make(map[string]float64)    // [userID] = profit
		currentProfitPct := make(map[string]float64) // [userID] = profitPercentage

		// Función para guardar los snapshots de todos los usuarios
		saveSnapshots := func() {
			startTime := time.Now()
			snapshotTime := startTime.Truncate(24 * time.Hour) // Truncar al día
			dayStr := snapshotTime.Format("2006-01-02")

			log.Printf("\n=== INICIANDO GUARDADO DE SNAPSHOTS DIARIOS PARA %s ===", dayStr)

			// Obtener todos los usuarios
			userIDs, err := p.getAllUsers()
			if err != nil {
				log.Printf("Error al obtener usuarios: %v", err)
				return
			}

			log.Printf("Procesando %d usuarios para el día %s", len(userIDs), dayStr)

			// Contador para estadísticas
			snapshotsSaved := 0
			snapshotsSkipped := 0

			// Para cada usuario, guardar un snapshot con el valor actual
			for _, userID := range userIDs {
				// Obtener el balance actual del usuario
				totalValue, totalInvested, profit, profitPercentage, err := p.getUserBalance(userID)
				if err != nil {
					log.Printf("Error al obtener balance para usuario %s: %v", userID, err)
					snapshotsSkipped++
					continue
				}

				// Usar SaveInvestmentSnapshot para guardar el snapshot
				err = p.cryptoRepo.SaveInvestmentSnapshot(
					userID,
					totalValue,
					totalInvested,
					profit,
					profitPercentage,
				)

				if err != nil {
					log.Printf("Error al guardar snapshot para usuario %s: %v", userID, err)
					snapshotsSkipped++
				} else {
					log.Printf("Snapshot guardado para usuario %s con valor: %.2f", userID, totalValue)
					snapshotsSaved++
				}

				// Actualizar los valores máximos para el próximo minuto
				currentMaxValues[userID] = totalValue
				currentInvested[userID] = totalInvested
				currentProfit[userID] = profit
				currentProfitPct[userID] = profitPercentage
			}

			// Registrar resumen de la operación
			duration := time.Since(startTime)
			log.Printf("=== RESUMEN SNAPSHOTS DIARIOS PARA %s ===", dayStr)
			log.Printf("Usuarios procesados: %d", len(userIDs))
			log.Printf("Snapshots guardados: %d", snapshotsSaved)
			log.Printf("Snapshots omitidos: %d", snapshotsSkipped)
			log.Printf("Tiempo total de procesamiento: %v\n", duration.Round(time.Millisecond))

			// Reiniciar los valores máximos para el nuevo día
			for userID := range currentMaxValues {
				currentMaxValues[userID] = 0
			}
			for userID := range currentInvested {
				currentInvested[userID] = 0
			}
			for userID := range currentProfit {
				currentProfit[userID] = 0
			}
			for userID := range currentProfitPct {
				currentProfitPct[userID] = 0
			}
		}

		// Función para actualizar los valores máximos
		updateMaxValues := func() {
			startTime := time.Now()
			log.Printf("\n=== INICIANDO ACTUALIZACIÓN DE VALORES MÁXIMOS A LAS %s ===", 
				startTime.Format("15:04:05.000"))

			// Obtener todos los usuarios
			userIDs, err := p.getAllUsers()
			if err != nil {
				log.Printf("Error al obtener usuarios: %v", err)
				return
			}

			log.Printf("Actualizando valores para %d usuarios", len(userIDs))

			// Contadores para estadísticas
			valuesUpdated := 0
			valuesSkipped := 0

			// Para cada usuario, obtener el balance actual y actualizar los máximos
			for _, userID := range userIDs {
				// Obtener el balance actual del usuario
				totalValue, totalInvested, profit, profitPercentage, err := p.getUserBalance(userID)
				if err != nil {
					log.Printf("Error al obtener balance para usuario %s: %v", userID, err)
					valuesSkipped++
					continue
				}

				// Actualizar los valores máximos si es necesario
				currentValue, exists := currentMaxValues[userID]
				if !exists || totalValue > currentValue {
					currentMaxValues[userID] = totalValue
					currentInvested[userID] = totalInvested
					currentProfit[userID] = profit
					currentProfitPct[userID] = profitPercentage
					
					log.Printf("Actualizado máximo para usuario %s: %.2f (anterior: %.2f)", 
						userID, totalValue, currentValue)
					valuesUpdated++
				} else {
					valuesSkipped++
				}
			}

			// Registrar resumen de la operación
			duration := time.Since(startTime)
			log.Printf("=== RESUMEN ACTUALIZACIÓN DE VALORES ===")
			log.Printf("Usuarios procesados: %d", len(userIDs))
			log.Printf("Valores actualizados: %d", valuesUpdated)
			log.Printf("Valores sin cambios: %d", valuesSkipped)
			log.Printf("Tiempo total de procesamiento: %v\n", duration.Round(time.Millisecond))
		}

		// Guardar snapshots inmediatamente al inicio
		updateMaxValues()
		saveSnapshots()

		for {
			select {
			case <-ticker.C:
				// Cada minuto exacto, guardar los snapshots con los valores máximos
				saveSnapshots()
			
			case <-updateTicker.C:
				// Cada 5 segundos, actualizar los valores máximos
				updateMaxValues()
			
			case <-p.stopChan:
				return
			}
		}
	}()

	log.Printf("Servicio de actualización de precios iniciado (guardando un snapshot por minuto)")
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

// getUserBalance obtiene el balance actual de un usuario
func (p *PriceUpdater) getUserBalance(userID string) (totalValue, totalInvested, profit, profitPercentage float64, err error) {
	// Obtener las tenencias del usuario
	holdings, err := p.holdingsRepo.GetHoldings(userID)
	if err != nil {
		log.Printf("Error al obtener tenencias para usuario %s: %v", userID, err)
		return 0, 0, 0, 0, err
	}

	// Actualizar el balance en el mapa
	balance := userBalance{
		totalValue:    holdings.TotalCurrentValue,
		totalInvested: holdings.TotalInvested,
		profit:        holdings.TotalProfit,
		profitPct:     holdings.ProfitPercentage,
	}
	p.userBalances.Store(userID, balance)

	return balance.totalValue, balance.totalInvested, balance.profit, balance.profitPct, nil
}

// updateUserBalance actualiza el balance de un usuario específico
func (p *PriceUpdater) updateUserBalance(userID string) {
	// Obtener las tenencias del usuario
	holdings, err := p.holdingsRepo.GetHoldings(userID)
	if err != nil {
		log.Printf("Error al obtener tenencias para usuario %s: %v", userID, err)
		return
	}

	// Actualizar el balance en el mapa
	p.userBalances.Store(userID, userBalance{
		totalValue:    holdings.TotalCurrentValue,
		totalInvested: holdings.TotalInvested,
		profit:        holdings.TotalProfit,
		profitPct:     holdings.ProfitPercentage,
	})
}

// GetCachedBalance obtiene el balance en caché para un usuario
func (p *PriceUpdater) GetCachedBalance(userID string) (interface{}, bool) {
	if balance, ok := p.userBalances.Load(userID); ok {
		return balance, true
	}
	return nil, false
}

// GetLastUpdated devuelve la última vez que se actualizaron los precios
func (p *PriceUpdater) GetLastUpdated() time.Time {
	p.mutex.Lock()
	defer p.mutex.Unlock()
	return p.lastUpdated
}

// GetInvestmentHistory obtiene el historial de inversiones de un usuario
func (p *PriceUpdater) GetInvestmentHistory(userID string, limit int) ([]models.InvestmentSnapshot, error) {
	return p.cryptoRepo.GetInvestmentHistory(userID, limit)
}

// GetInvestmentHistorySince obtiene el historial de inversiones de un usuario desde una fecha específica
func (p *PriceUpdater) GetInvestmentHistorySince(userID string, since time.Time) ([]models.InvestmentSnapshot, error) {
	return p.cryptoRepo.GetInvestmentHistorySince(userID, since)
}

// GetFormattedInvestmentHistory obtiene el historial de inversiones formateado para gráficos
func (p *PriceUpdater) GetFormattedInvestmentHistory(userID string, limit int) (map[string]interface{}, error) {
	snapshots, err := p.cryptoRepo.GetInvestmentHistory(userID, limit)
	if err != nil {
		return nil, err
	}

	// Preparar datos para el gráfico
	var labels []string
	var values []float64

	for _, snapshot := range snapshots {
		// Formatear la fecha para el gráfico (solo hora:minuto)
		labels = append(labels, snapshot.Date.Format("15:04"))
		values = append(values, snapshot.TotalValue)
	}

	// Crear el objeto de respuesta
	result := map[string]interface{}{
		"snapshots": snapshots,
		"labels":    labels,
		"values":    values,
	}

	return result, nil
}

// GetFormattedInvestmentHistorySince obtiene el historial de inversiones desde una fecha específica, agrupado por día
func (p *PriceUpdater) GetFormattedInvestmentHistorySince(userID string, since time.Time) (map[string]interface{}, error) {
	// Actualizar manualmente el balance del usuario antes de obtener el historial
	totalValue, totalInvested, profit, profitPercentage, err := p.getUserBalance(userID)
	if err != nil {
		log.Printf("Error al obtener balance actual para usuario %s: %v", userID, err)
	} else {
		// Forzar la creación de un snapshot con el valor actual
		// Esto asegura que siempre tengamos el valor más reciente para el día actual
		err = p.cryptoRepo.SaveInvestmentSnapshot(
			userID,
			totalValue,
			totalInvested,
			profit,
			profitPercentage,
		)
		if err != nil {
			log.Printf("Error al guardar snapshot actual: %v", err)
		}
	}

	// Obtener los snapshots desde la fecha especificada
	snapshots, err := p.GetInvestmentHistorySince(userID, since)
	if err != nil {
		return nil, err
	}

	// Si no hay snapshots, devolver un objeto vacío
	if len(snapshots) == 0 {
		return map[string]interface{}{
			"snapshots": []models.InvestmentSnapshot{},
			"labels":    []string{},
			"values":    []float64{},
		}, nil
	}

	// Ordenar los snapshots por fecha
	sort.Slice(snapshots, func(i, j int) bool {
		return snapshots[i].Date.Before(snapshots[j].Date)
	})

	// Crear un mapa para agrupar por día (YYYY-MM-DD)
	dayMap := make(map[string]models.InvestmentSnapshot)

	// Obtener la fecha actual truncada a día
	currentDay := time.Now().Truncate(24 * time.Hour)
	currentDayKey := currentDay.Format("2006-01-02")

	// Procesar cada snapshot
	for _, snapshot := range snapshots {
		// Formatear la fecha a "2006-01-02" (año-mes-día)
		// para agrupar por día exacto
		dayKey := snapshot.Date.Format("2006-01-02")
		
		// Si ya existe un snapshot para este día, solo actualizamos si el valor total es mayor
		// o si es el día actual (siempre queremos el más reciente para el día actual)
		if existing, exists := dayMap[dayKey]; exists {
			if snapshot.TotalValue > existing.TotalValue || dayKey == currentDayKey {
				// Mantener la fecha pero truncar a día completo (00:00:00)
				snapshot.Date = time.Date(
					snapshot.Date.Year(), snapshot.Date.Month(), snapshot.Date.Day(),
					0, 0, 0, 0, // Hora, minuto, segundo, nanosegundo en 0
					time.UTC,
				)
				dayMap[dayKey] = snapshot
				log.Printf("Actualizado snapshot para día %s: valor %.2f", dayKey, snapshot.TotalValue)
			}
		} else {
			// Asegurarse de que la fecha tenga hora, minutos, segundos y milisegundos en 0 para agrupar por día
			snapshot.Date = time.Date(
				snapshot.Date.Year(), snapshot.Date.Month(), snapshot.Date.Day(),
				0, 0, 0, 0, // Hora, minuto, segundo, nanosegundo en 0
				time.UTC,
			)
			dayMap[dayKey] = snapshot
			log.Printf("Nuevo snapshot para día %s: valor %.2f", dayKey, snapshot.TotalValue)
		}
	}

	// Convertir el mapa a slice y ordenar por fecha
	type snapshotWithKey struct {
		key      string
		snapshot models.InvestmentSnapshot
	}

	var snapshotsList []snapshotWithKey
	for key, snapshot := range dayMap {
		snapshotsList = append(snapshotsList, snapshotWithKey{
			key:      key,
			snapshot: snapshot,
		})
	}

	// Ordenar por fecha
	sort.Slice(snapshotsList, func(i, j int) bool {
		return snapshotsList[i].snapshot.Date.Before(snapshotsList[j].snapshot.Date)
	})

	// Crear las listas ordenadas
	var orderedSnapshots []models.InvestmentSnapshot
	var labels []string
	var values []float64

	for _, item := range snapshotsList {
		snapshot := item.snapshot
		orderedSnapshots = append(orderedSnapshots, snapshot)
		// Mostrar la fecha en formato dd/mm en las etiquetas
		labels = append(labels, snapshot.Date.Format("02/01"))
		values = append(values, snapshot.TotalValue)
	}

	// Crear el objeto de respuesta
	result := map[string]interface{}{
		"snapshots": orderedSnapshots,
		"labels":    labels,
		"values":    values,
	}

	return result, nil
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
