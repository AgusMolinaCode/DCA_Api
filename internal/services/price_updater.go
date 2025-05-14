package services

import (
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
	db interface{} // Usar el tipo correcto para tu DB
}

func (a *cryptoRepositoryAdapter) SaveInvestmentSnapshot(userID string, totalValue, totalInvested, profit, profitPercentage float64) error {
	// Implementación que llama al repositorio real
	// Aquí podríamos importar el repositorio, pero lo haremos en tiempo de ejecución
	// para evitar el ciclo de importación
	// Por ahora, simplemente registramos la llamada
	log.Printf("Guardando snapshot para usuario %s: valor=%f, invertido=%f, ganancia=%f, porcentaje=%f",
		userID, totalValue, totalInvested, profit, profitPercentage)
	return nil
}

type holdingsRepositoryAdapter struct {
	db interface{} // Usar el tipo correcto para tu DB
}

func (a *holdingsRepositoryAdapter) GetHoldings(userID string) (*models.Holdings, error) {
	// Implementación que llama al repositorio real
	// Por ahora, devolvemos un objeto vacío
	return &models.Holdings{}, nil
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
		var userID string
		if err := rows.Scan(&userID); err != nil {
			return nil, err
		}
		users = append(users, userID)
	}

	return users, nil
}
