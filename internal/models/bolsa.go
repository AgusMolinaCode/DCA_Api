package models

import (
	"fmt"
	"time"
)

// Tipos de reglas para triggers
const (
	TriggerTypePriceReached = "price_reached"
	TriggerTypeValueReached = "value_reached"
)

// ProgressInfo contiene información sobre el progreso hacia el objetivo de una bolsa
type ProgressInfo struct {
	Percent       float64 `json:"percent"`                  // Porcentaje de progreso (0-100)
	RawPercent    float64 `json:"raw_percent"`              // Porcentaje real sin limitar a 100%
	Status        string  `json:"status"`                   // "pendiente", "completado", "superado"
	ExcessAmount  float64 `json:"excess_amount,omitempty"`  // Cantidad que excede el objetivo
	ExcessPercent float64 `json:"excess_percent,omitempty"` // Porcentaje que excede el objetivo
}

// Bolsa representa una sub-cartera con un objetivo específico
type Bolsa struct {
	ID           string         `json:"id"`
	UserID       string         `json:"user_id"`
	Name         string         `json:"name" binding:"required"`
	Description  string         `json:"description"`
	Goal         float64        `json:"goal"`
	CurrentValue float64        `json:"current_value"`      // Campo calculado, no almacenado
	Progress     *ProgressInfo  `json:"progress,omitempty"` // Información de progreso hacia el objetivo
	Tags         []string       `json:"tags,omitempty"`
	Assets       []AssetInBolsa `json:"assets,omitempty"`
	Rules        []TriggerRule  `json:"rules,omitempty"`
	CreatedAt    time.Time      `json:"created_at"`
	UpdatedAt    time.Time      `json:"updated_at"`
}

// AssetInBolsa representa un activo dentro de una bolsa
type AssetInBolsa struct {
	ID              string    `json:"id"`
	BolsaID         string    `json:"bolsa_id"`
	CryptoName      string    `json:"crypto_name" binding:"required"`
	Ticker          string    `json:"ticker" binding:"required"`
	Amount          float64   `json:"amount" binding:"required,gt=0"`
	PurchasePrice   float64   `json:"purchase_price"`
	Total           float64   `json:"total"`
	CurrentPrice    float64   `json:"current_price"`     // Campo calculado, no almacenado
	CurrentValue    float64   `json:"current_value"`     // Campo calculado, no almacenado
	GainLoss        float64   `json:"gain_loss"`         // Campo calculado, no almacenado
	GainLossPercent float64   `json:"gain_loss_percent"` // Campo calculado, no almacenado
	ImageURL        string    `json:"image_url,omitempty"`
	CreatedAt       time.Time `json:"created_at"`
	UpdatedAt       time.Time `json:"updated_at"`
}

// TriggerRule representa una regla para una bolsa
type TriggerRule struct {
	ID          string    `json:"id"`
	BolsaID     string    `json:"bolsa_id"`
	Type        string    `json:"type" binding:"required"` // "price_reached" o "value_reached"
	Ticker      string    `json:"ticker,omitempty"`        // Solo para reglas de tipo "price_reached"
	TargetValue float64   `json:"target_value" binding:"required"`
	Active      bool      `json:"active"`
	Triggered   bool      `json:"triggered"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// GenerateUUID - Función auxiliar para generar UUIDs
func GenerateUUID() string {
	// Usamos el timestamp en nanosegundos como ID único
	return fmt.Sprintf("%d", time.Now().UnixNano())
}
