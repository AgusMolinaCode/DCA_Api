package middleware

import (
	"log"
	"net/http"
	"time"

	"github.com/AgusMolinaCode/DCA_Api.git/internal/database"
	"github.com/AgusMolinaCode/DCA_Api.git/internal/models"
	"github.com/AgusMolinaCode/DCA_Api.git/internal/repository"
	"github.com/AgusMolinaCode/DCA_Api.git/internal/services"
	"github.com/gin-gonic/gin"
)

var bolsaRepo *repository.BolsaRepository

// InitBolsa inicializa el repositorio de bolsas
func InitBolsa() {
	bolsaRepo = repository.NewBolsaRepository(database.DB)
}

// CreateBolsa crea una nueva bolsa
func CreateBolsa(c *gin.Context) {
	// Obtener el ID del usuario del contexto (establecido por AuthMiddleware)
	userID := c.GetString("userId")
	if userID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "usuario no autenticado"})
		return
	}

	var bolsa models.Bolsa
	if err := c.ShouldBindJSON(&bolsa); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Asignar el ID del usuario
	bolsa.UserID = userID

	// Generar un ID único para la bolsa
	bolsa.ID = models.GenerateUUID()

	// Establecer timestamps
	now := time.Now()
	bolsa.CreatedAt = now
	bolsa.UpdatedAt = now

	// Crear la bolsa
	if err := bolsaRepo.CreateBolsa(bolsa); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "error al crear la bolsa: " + err.Error()})
		return
	}

	c.JSON(http.StatusCreated, gin.H{"message": "bolsa creada exitosamente", "bolsa": bolsa})
}

// GetUserBolsas obtiene todas las bolsas de un usuario
func GetUserBolsas(c *gin.Context) {
	// Obtener el ID del usuario del contexto
	userID := c.GetString("userId")
	if userID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "usuario no autenticado"})
		return
	}

	// Obtener las bolsas del usuario
	bolsas, err := bolsaRepo.GetBolsasByUserID(userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "error al obtener las bolsas: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"bolsas": bolsas})
}

// GetBolsaDetails obtiene los detalles de una bolsa específica
func GetBolsaDetails(c *gin.Context) {
	// Obtener el ID del usuario del contexto
	userID := c.GetString("userId")
	if userID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "usuario no autenticado"})
		return
	}

	// Obtener el ID de la bolsa de los parámetros de la URL
	bolsaID := c.Param("id")
	if bolsaID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "ID de bolsa no proporcionado"})
		return
	}

	// Obtener la bolsa
	bolsa, err := bolsaRepo.GetBolsaByID(bolsaID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "error al obtener la bolsa: " + err.Error()})
		return
	}

	// Verificar que la bolsa pertenece al usuario
	if bolsa.UserID != userID {
		c.JSON(http.StatusForbidden, gin.H{"error": "no tienes permiso para acceder a esta bolsa"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"bolsa": bolsa})
}

// AddAssetsToBolsa añade múltiples activos a una bolsa existente
func AddAssetsToBolsa(c *gin.Context) {
	// Obtener el ID de la bolsa de los parámetros de la URL
	bolsaID := c.Param("id")
	if bolsaID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "ID de bolsa no proporcionado"})
		return
	}

	// Obtener el ID del usuario del contexto
	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Usuario no autenticado"})
		return
	}

	// Obtener la bolsa para verificar que pertenece al usuario
	bolsaRepo := repository.NewBolsaRepository(database.DB)
	bolsa, err := bolsaRepo.GetBolsaByID(bolsaID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Bolsa no encontrada"})
		return
	}

	// Verificar que la bolsa pertenece al usuario
	if bolsa.UserID != userID.(string) {
		c.JSON(http.StatusForbidden, gin.H{"error": "No tienes permiso para acceder a esta bolsa"})
		return
	}

	// Parsear los activos del cuerpo de la solicitud
	var request struct {
		Assets []models.AssetInBolsa `json:"assets"`
	}

	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if len(request.Assets) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "No se proporcionaron activos"})
		return
	}

	// Procesar cada activo
	addedAssets := []models.AssetInBolsa{}
	totalValueAdded := 0.0
	triggeredRules := []models.TriggerRule{}

	for _, asset := range request.Assets {
		// Generar ID único para el activo
		asset.ID = models.GenerateUUID()
		asset.BolsaID = bolsaID

		// Establecer timestamps
		now := time.Now()
		asset.CreatedAt = now
		asset.UpdatedAt = now

		// Calcular el valor total del activo
		asset.Total = asset.Amount * asset.PurchasePrice

		// Obtener precio actual y calcular valores derivados
		cryptoData, err := services.GetCryptoPrice(asset.Ticker)
		if err != nil {
			// Si no se puede obtener el precio actual, usar el precio de compra
			log.Printf("Error al obtener precio para %s: %v", asset.Ticker, err)
			asset.CurrentPrice = asset.PurchasePrice
		} else {
			asset.CurrentPrice = cryptoData.Raw[asset.Ticker]["USD"].PRICE
		}

		asset.CurrentValue = asset.Amount * asset.CurrentPrice
		asset.GainLoss = asset.CurrentValue - asset.Total

		if asset.Total > 0 {
			asset.GainLossPercent = (asset.GainLoss / asset.Total) * 100
		}

		// Añadir el activo a la base de datos
		err = bolsaRepo.AddAssetToBolsa(asset)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Error al añadir activo a la bolsa"})
			return
		}

		addedAssets = append(addedAssets, asset)
		totalValueAdded += asset.Total
	}

	// Obtener la bolsa actualizada con todos los activos
	updatedBolsa, err := bolsaRepo.GetBolsaByID(bolsaID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Error al obtener la bolsa actualizada"})
		return
	}

	// Calcular el progreso hacia el objetivo si existe
	progressPercent := 0.0
	if updatedBolsa.Goal > 0 {
		progressPercent = (updatedBolsa.CurrentValue / updatedBolsa.Goal) * 100
	}

	// Verificar si se han activado reglas de tipo "value_reached"
	for _, rule := range updatedBolsa.Rules {
		if rule.Active && !rule.Triggered && rule.Type == "value_reached" {
			// Para reglas de valor alcanzado, verificar si el valor actual supera el objetivo
			if updatedBolsa.CurrentValue >= rule.TargetValue {
				rule.Triggered = true
				rule.UpdatedAt = time.Now()

				// Actualizar la regla en la base de datos
				err = bolsaRepo.UpdateRule(rule)
				if err != nil {
					log.Printf("Error al actualizar regla: %v", err)
				} else {
					triggeredRules = append(triggeredRules, rule)
				}
			}
		}
	}

	// Preparar la respuesta
	response := gin.H{
		"added_assets":      addedAssets,
		"bolsa":             updatedBolsa,
		"total_value_added": totalValueAdded,
		"current_value":     updatedBolsa.CurrentValue,
	}

	if updatedBolsa.Goal > 0 {
		response["progress_percent"] = progressPercent
	}

	if len(triggeredRules) > 0 {
		response["triggered_rules"] = triggeredRules
	}

	c.JSON(http.StatusOK, response)
}
