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

	// Actualizar los precios actuales de todos los activos en todas las bolsas
	for i := range bolsas {
		updateCryptoPrices(&bolsas[i])
	}

	c.JSON(http.StatusOK, gin.H{"bolsas": bolsas})
}

// updateCryptoPrices actualiza los precios actuales de las criptomonedas en una bolsa
func updateCryptoPrices(bolsa *models.Bolsa) {
	if bolsa == nil || len(bolsa.Assets) == 0 {
		return
	}

	// Recopilar todos los tickers únicos de la bolsa
	tickers := make([]string, 0)
	tickerMap := make(map[string]bool)
	for _, asset := range bolsa.Assets {
		if !tickerMap[asset.Ticker] {
			tickers = append(tickers, asset.Ticker)
			tickerMap[asset.Ticker] = true
		}
	}

	// Obtener los precios actuales de todas las criptomonedas en una sola llamada a la API
	prices, err := services.GetMultipleCryptoPrices(tickers)
	if err != nil {
		log.Printf("Error al obtener precios actuales: %v", err)
		// Si hay un error, continuamos con los precios existentes
	} else {
		// Actualizar el precio actual de cada activo
		for i := range bolsa.Assets {
			if currentPrice, exists := prices[bolsa.Assets[i].Ticker]; exists {
				// Actualizar el precio actual con el valor de la API
				bolsa.Assets[i].CurrentPrice = currentPrice
				log.Printf("Precio actualizado para %s: %.2f (precio anterior: %.2f)", 
					bolsa.Assets[i].Ticker, currentPrice, bolsa.Assets[i].PurchasePrice)
			} else {
				// Si no encontramos el precio, mantenemos el precio de compra
				log.Printf("No se encontró precio para %s, manteniendo precio de compra: %.2f", 
					bolsa.Assets[i].Ticker, bolsa.Assets[i].PurchasePrice)
			}
		}
	}

	// Recalcular valores derivados para todos los activos
	for i := range bolsa.Assets {
		bolsa.Assets[i].CurrentValue = bolsa.Assets[i].Amount * bolsa.Assets[i].CurrentPrice
		bolsa.Assets[i].GainLoss = bolsa.Assets[i].CurrentValue - bolsa.Assets[i].Total

		if bolsa.Assets[i].Total > 0 {
			bolsa.Assets[i].GainLossPercent = (bolsa.Assets[i].GainLoss / bolsa.Assets[i].Total) * 100
		}
	}

	// Recalcular el valor actual total de la bolsa
	bolsa.CurrentValue = 0
	for _, asset := range bolsa.Assets {
		bolsa.CurrentValue += asset.CurrentValue
	}
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

	// Actualizar los precios actuales de todos los activos en la bolsa
	updateCryptoPrices(bolsa)

	// Imprimir los precios actualizados para depuración
	log.Printf("Precios actualizados para la bolsa %s:", bolsa.ID)
	for _, asset := range bolsa.Assets {
		log.Printf("  - %s: Precio de compra: %.2f, Precio actual: %.2f", asset.Ticker, asset.PurchasePrice, asset.CurrentPrice)
	}

	// Calcular información de progreso si hay un objetivo establecido
	if bolsa.Goal > 0 {
		// Calcular el porcentaje real de progreso
		rawPercent := (bolsa.CurrentValue / bolsa.Goal) * 100

		// Crear objeto de progreso
		progress := &models.ProgressInfo{
			RawPercent: rawPercent,
		}

		// Limitar el porcentaje mostrado a 100% si se superó el objetivo
		if rawPercent > 100 {
			progress.Percent = 100
			progress.Status = "superado"
			progress.ExcessAmount = bolsa.CurrentValue - bolsa.Goal
			progress.ExcessPercent = rawPercent - 100
		} else if rawPercent == 100 {
			progress.Percent = 100
			progress.Status = "completado"
		} else {
			progress.Percent = rawPercent
			progress.Status = "pendiente"
		}

		// Asignar el progreso a la bolsa
		bolsa.Progress = progress
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
	userID := c.GetString("userId")
	if userID == "" {
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
	if bolsa.UserID != userID {
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

		// Recopilar todos los tickers para obtener los precios en una sola llamada
		tickers := []string{asset.Ticker}
		
		// Obtener precio actual y calcular valores derivados
		prices, err := services.GetMultipleCryptoPrices(tickers)
		if err != nil {
			// Si no se puede obtener el precio actual, usar el precio de compra
			log.Printf("Error al obtener precio para %s: %v", asset.Ticker, err)
			asset.CurrentPrice = asset.PurchasePrice
		} else if currentPrice, exists := prices[asset.Ticker]; exists {
			// Actualizar el precio actual con el valor de la API
			asset.CurrentPrice = currentPrice
			log.Printf("Precio actualizado para %s: %.2f (precio de compra: %.2f)", asset.Ticker, currentPrice, asset.PurchasePrice)
		} else {
			// Si no encontramos el precio, mantenemos el precio de compra
			log.Printf("No se encontró precio para %s, manteniendo precio de compra: %.2f", asset.Ticker, asset.PurchasePrice)
			asset.CurrentPrice = asset.PurchasePrice
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

// UpdateBolsa actualiza una bolsa existente y sus activos
func UpdateBolsa(c *gin.Context) {
	// Obtener el ID de la bolsa de los parámetros de la URL
	bolsaID := c.Param("id")
	if bolsaID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "ID de bolsa no proporcionado"})
		return
	}

	// Obtener el ID del usuario del contexto
	userID := c.GetString("userId")
	if userID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Usuario no autenticado"})
		return
	}

	// Obtener la bolsa actual para verificar que pertenece al usuario
	bolsaRepo := repository.NewBolsaRepository(database.DB)
	existingBolsa, err := bolsaRepo.GetBolsaByID(bolsaID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Bolsa no encontrada"})
		return
	}

	// Verificar que la bolsa pertenece al usuario
	if existingBolsa.UserID != userID {
		c.JSON(http.StatusForbidden, gin.H{"error": "No tienes permiso para acceder a esta bolsa"})
		return
	}

	// Parsear los datos de actualización del cuerpo de la solicitud
	var request struct {
		Name        string                `json:"name,omitempty"`
		Description string                `json:"description,omitempty"`
		Goal        float64               `json:"goal,omitempty"`
		Assets      []models.AssetInBolsa `json:"assets,omitempty"`
	}

	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Actualizar los campos básicos de la bolsa si se proporcionaron
	updated := false
	if request.Name != "" {
		existingBolsa.Name = request.Name
		updated = true
	}

	if request.Description != "" {
		existingBolsa.Description = request.Description
		updated = true
	}

	if request.Goal > 0 {
		existingBolsa.Goal = request.Goal
		updated = true
	}

	if updated {
		existingBolsa.UpdatedAt = time.Now()

		// Guardar los cambios en la base de datos
		err = bolsaRepo.UpdateBolsa(*existingBolsa)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Error al actualizar la bolsa"})
			return
		}
	}

	// Actualizar los activos si se proporcionaron
	updatedAssets := []models.AssetInBolsa{}
	if len(request.Assets) > 0 {
		for _, updatedAsset := range request.Assets {
			// Buscar el activo existente por ID
			found := false
			for _, existingAsset := range existingBolsa.Assets {
				if updatedAsset.ID == existingAsset.ID {
					// Actualizar solo los campos proporcionados
					if updatedAsset.Amount > 0 {
						existingAsset.Amount = updatedAsset.Amount
					}

					if updatedAsset.PurchasePrice > 0 {
						existingAsset.PurchasePrice = updatedAsset.PurchasePrice
					}

					// Recalcular valores derivados
					existingAsset.Total = existingAsset.Amount * existingAsset.PurchasePrice

					// Obtener precio actual y calcular valores derivados
					cryptoData, err := services.GetCryptoPrice(existingAsset.Ticker)
					if err != nil {
						// Si no se puede obtener el precio actual, usar el precio de compra
						log.Printf("Error al obtener precio para %s: %v", existingAsset.Ticker, err)
						existingAsset.CurrentPrice = existingAsset.PurchasePrice
					} else {
						existingAsset.CurrentPrice = cryptoData.Raw[existingAsset.Ticker]["USD"].PRICE
					}

					existingAsset.CurrentValue = existingAsset.Amount * existingAsset.CurrentPrice
					existingAsset.GainLoss = existingAsset.CurrentValue - existingAsset.Total

					if existingAsset.Total > 0 {
						existingAsset.GainLossPercent = (existingAsset.GainLoss / existingAsset.Total) * 100
					}

					existingAsset.UpdatedAt = time.Now()

					// Actualizar el activo en la base de datos
					err = bolsaRepo.UpdateAsset(existingAsset)
					if err != nil {
						c.JSON(http.StatusInternalServerError, gin.H{"error": "Error al actualizar activo: " + existingAsset.ID})
						return
					}

					updatedAssets = append(updatedAssets, existingAsset)
					found = true
					break
				}
			}

			if !found {
				// Si no se encontró el activo, devolver un error
				c.JSON(http.StatusBadRequest, gin.H{"error": "Activo no encontrado: " + updatedAsset.ID})
				return
			}
		}
	}

	// Obtener la bolsa actualizada con todos sus datos
	updatedBolsa, err := bolsaRepo.GetBolsaByID(bolsaID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Error al obtener la bolsa actualizada."})
		return
	}

	// Calcular información de progreso actualizada
	if updatedBolsa.Goal > 0 {
		// Calcular el porcentaje real de progreso
		rawPercent := (updatedBolsa.CurrentValue / updatedBolsa.Goal) * 100

		// Crear objeto de progreso
		progress := &models.ProgressInfo{
			RawPercent: rawPercent,
		}

		// Limitar el porcentaje mostrado a 100% si se superó el objetivo
		if rawPercent > 100 {
			progress.Percent = 100
			progress.Status = "superado"
			progress.ExcessAmount = updatedBolsa.CurrentValue - updatedBolsa.Goal
			progress.ExcessPercent = rawPercent - 100
		} else if rawPercent == 100 {
			progress.Percent = 100
			progress.Status = "completado"
		} else {
			progress.Percent = rawPercent
			progress.Status = "pendiente"
		}

		// Asignar el progreso a la bolsa
		updatedBolsa.Progress = progress
	}

	c.JSON(http.StatusOK, gin.H{
		"message":        "Bolsa actualizada correctamente",
		"bolsa":          updatedBolsa,
		"updated_assets": updatedAssets,
	})
}

// CompleteBolsaAndTransfer completa una bolsa y transfiere el exceso a otra bolsa
func CompleteBolsaAndTransfer(c *gin.Context) {
	// Obtener el ID de la bolsa de los parámetros de la URL
	bolsaID := c.Param("id")
	if bolsaID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "ID de bolsa no proporcionado"})
		return
	}

	// Obtener el ID del usuario del contexto
	userID := c.GetString("userId")
	if userID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Usuario no autenticado"})
		return
	}

	// Parsear los datos de la solicitud
	var request struct {
		TargetBolsaID string `json:"target_bolsa_id" binding:"required"`
	}

	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Verificar que la bolsa destino exista y pertenezca al usuario
	bolsaRepo := repository.NewBolsaRepository(database.DB)
	targetBolsa, err := bolsaRepo.GetBolsaByID(request.TargetBolsaID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Bolsa destino no encontrada"})
		return
	}

	if targetBolsa.UserID != userID {
		c.JSON(http.StatusForbidden, gin.H{"error": "No tienes permiso para acceder a la bolsa destino"})
		return
	}

	// Obtener la bolsa origen
	sourceBolsa, err := bolsaRepo.GetBolsaByID(bolsaID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Bolsa origen no encontrada"})
		return
	}

	// Verificar que la bolsa origen pertenezca al usuario
	if sourceBolsa.UserID != userID {
		c.JSON(http.StatusForbidden, gin.H{"error": "No tienes permiso para acceder a la bolsa origen"})
		return
	}

	// Verificar que la bolsa origen tenga un objetivo y que lo haya superado
	if sourceBolsa.Goal <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "La bolsa origen no tiene un objetivo definido"})
		return
	}

	if sourceBolsa.CurrentValue <= sourceBolsa.Goal {
		c.JSON(http.StatusBadRequest, gin.H{"error": "La bolsa origen no ha superado su objetivo"})
		return
	}

	// Calcular el exceso a transferir
	excessAmount := sourceBolsa.CurrentValue - sourceBolsa.Goal
	if excessAmount <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "No hay exceso para transferir"})
		return
	}

	// Calcular el porcentaje de exceso para cada activo
	excessRatio := excessAmount / sourceBolsa.CurrentValue

	// Preparar los activos a transferir
	transferredAssets := []models.AssetInBolsa{}

	// Para cada activo en la bolsa origen
	for _, asset := range sourceBolsa.Assets {
		// Calcular la cantidad a transferir
		transferAmount := asset.Amount * excessRatio

		// Si la cantidad a transferir es significativa
		if transferAmount > 0 {
			// Crear un nuevo activo para la bolsa destino
			newAsset := models.AssetInBolsa{
				ID:              models.GenerateUUID(),
				BolsaID:         targetBolsa.ID,
				CryptoName:      asset.CryptoName,
				Ticker:          asset.Ticker,
				Amount:          transferAmount,
				PurchasePrice:   asset.CurrentPrice, // Usar el precio actual como precio de compra
				Total:           transferAmount * asset.CurrentPrice,
				CurrentPrice:    asset.CurrentPrice,
				CurrentValue:    transferAmount * asset.CurrentPrice,
				GainLoss:        0, // No hay ganancia/pérdida inicial
				GainLossPercent: 0,
				ImageURL:        asset.ImageURL,
				CreatedAt:       time.Now(),
				UpdatedAt:       time.Now(),
			}

			// Agregar el activo a la lista de activos transferidos
			transferredAssets = append(transferredAssets, newAsset)

			// Actualizar la cantidad del activo en la bolsa origen
			asset.Amount -= transferAmount
			asset.Total = asset.Amount * asset.PurchasePrice
			asset.CurrentValue = asset.Amount * asset.CurrentPrice
			asset.GainLoss = asset.CurrentValue - asset.Total
			if asset.Total > 0 {
				asset.GainLossPercent = (asset.GainLoss / asset.Total) * 100
			}
			asset.UpdatedAt = time.Now()

			// Actualizar el activo en la base de datos
			err = bolsaRepo.UpdateAsset(asset)
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Error al actualizar activo en bolsa origen: " + asset.ID})
				return
			}
		}
	}

	// Agregar los activos transferidos a la bolsa destino
	for _, asset := range transferredAssets {
		err = bolsaRepo.AddAssetToBolsa(asset)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Error al agregar activo a bolsa destino: " + asset.ID})
			return
		}
	}

	// Obtener las bolsas actualizadas
	updatedSourceBolsa, err := bolsaRepo.GetBolsaByID(bolsaID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Error al obtener bolsa origen actualizada"})
		return
	}

	updatedTargetBolsa, err := bolsaRepo.GetBolsaByID(request.TargetBolsaID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Error al obtener bolsa destino actualizada"})
		return
	}

	// Calcular información de progreso para ambas bolsas
	// Bolsa origen
	if updatedSourceBolsa.Goal > 0 {
		rawPercent := (updatedSourceBolsa.CurrentValue / updatedSourceBolsa.Goal) * 100
		progress := &models.ProgressInfo{
			RawPercent: rawPercent,
		}

		if rawPercent > 100 {
			progress.Percent = 100
			progress.Status = "superado"
			progress.ExcessAmount = updatedSourceBolsa.CurrentValue - updatedSourceBolsa.Goal
			progress.ExcessPercent = rawPercent - 100
		} else if rawPercent == 100 {
			progress.Percent = 100
			progress.Status = "completado"
		} else {
			progress.Percent = rawPercent
			progress.Status = "pendiente"
		}

		updatedSourceBolsa.Progress = progress
	}

	// Bolsa destino
	if updatedTargetBolsa.Goal > 0 {
		rawPercent := (updatedTargetBolsa.CurrentValue / updatedTargetBolsa.Goal) * 100
		progress := &models.ProgressInfo{
			RawPercent: rawPercent,
		}

		if rawPercent > 100 {
			progress.Percent = 100
			progress.Status = "superado"
			progress.ExcessAmount = updatedTargetBolsa.CurrentValue - updatedTargetBolsa.Goal
			progress.ExcessPercent = rawPercent - 100
		} else if rawPercent == 100 {
			progress.Percent = 100
			progress.Status = "completado"
		} else {
			progress.Percent = rawPercent
			progress.Status = "pendiente"
		}

		updatedTargetBolsa.Progress = progress
	}

	// Preparar la respuesta
	response := gin.H{
		"message":            "Transferencia completada exitosamente",
		"source_bolsa":       updatedSourceBolsa,
		"target_bolsa":       updatedTargetBolsa,
		"transferred_assets": transferredAssets,
		"transferred_amount": excessAmount,
	}

	c.JSON(http.StatusOK, response)
}

// ManageBolsaTags gestiona las etiquetas de una bolsa (añadir o eliminar)
func ManageBolsaTags(c *gin.Context) {
	// Obtener el ID de la bolsa de los parámetros de la URL
	bolsaID := c.Param("id")
	if bolsaID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "ID de bolsa no proporcionado"})
		return
	}

	// Obtener el ID del usuario del contexto
	userID := c.GetString("userId")
	if userID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Usuario no autenticado"})
		return
	}

	// Obtener la bolsa actual para verificar que pertenece al usuario
	bolsaRepo := repository.NewBolsaRepository(database.DB)
	existingBolsa, err := bolsaRepo.GetBolsaByID(bolsaID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Bolsa no encontrada"})
		return
	}

	// Verificar que la bolsa pertenece al usuario
	if existingBolsa.UserID != userID {
		c.JSON(http.StatusForbidden, gin.H{"error": "No tienes permiso para acceder a esta bolsa"})
		return
	}

	// Parsear los datos de la solicitud
	var request struct {
		Action string   `json:"action" binding:"required,oneof=add remove"`
		Tags   []string `json:"tags" binding:"required"`
	}

	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Procesar las etiquetas según la acción
	switch request.Action {
	case "add":
		// Añadir etiquetas
		for _, tag := range request.Tags {
			err := bolsaRepo.AddTagToBolsa(bolsaID, tag)
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Error al añadir etiqueta: " + tag})
				return
			}
		}
	case "remove":
		// Eliminar etiquetas
		for _, tag := range request.Tags {
			err := bolsaRepo.RemoveTagFromBolsa(bolsaID, tag)
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Error al eliminar etiqueta: " + tag})
				return
			}
		}
	}

	// Obtener la bolsa actualizada
	updatedBolsa, err := bolsaRepo.GetBolsaByID(bolsaID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Error al obtener la bolsa actualizada"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "Etiquetas actualizadas correctamente",
		"bolsa":   updatedBolsa,
	})
}

// GetBolsasByTag obtiene todas las bolsas que tienen una etiqueta específica
func GetBolsasByTag(c *gin.Context) {
	// Obtener la etiqueta de los parámetros de la URL
	tag := c.Param("tag")
	if tag == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Etiqueta no proporcionada"})
		return
	}

	// Obtener el ID del usuario del contexto
	userID := c.GetString("userId")
	if userID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Usuario no autenticado"})
		return
	}

	// Obtener las bolsas con la etiqueta especificada
	bolsaRepo := repository.NewBolsaRepository(database.DB)
	bolsas, err := bolsaRepo.GetBolsasByTag(userID, tag)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Error al obtener bolsas por etiqueta"})
		return
	}

	// Calcular información de progreso para cada bolsa
	for i := range bolsas {
		if bolsas[i].Goal > 0 {
			// Calcular el porcentaje real de progreso
			rawPercent := (bolsas[i].CurrentValue / bolsas[i].Goal) * 100

			// Crear objeto de progreso
			progress := &models.ProgressInfo{
				RawPercent: rawPercent,
			}

			// Limitar el porcentaje mostrado a 100% si se superó el objetivo
			if rawPercent > 100 {
				progress.Percent = 100
				progress.Status = "superado"
				progress.ExcessAmount = bolsas[i].CurrentValue - bolsas[i].Goal
				progress.ExcessPercent = rawPercent - 100
			} else if rawPercent == 100 {
				progress.Percent = 100
				progress.Status = "completado"
			} else {
				progress.Percent = rawPercent
				progress.Status = "pendiente"
			}

			// Asignar el progreso a la bolsa
			bolsas[i].Progress = progress
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"tag":    tag,
		"bolsas": bolsas,
	})
}
