package models

import "time"

type PortfolioHistory struct {
	ID               string    `json:"id"`
	UserID           string    `json:"user_id"`
	TotalValue       float64   `json:"total_value"`
	TotalInvested    float64   `json:"total_invested"`
	Profit           float64   `json:"profit"`
	ProfitPercentage float64   `json:"profit_percentage"`
	Timestamp        time.Time `json:"timestamp"`
}

type PortfolioChartData struct {
	Labels []string  `json:"labels"` // Fechas en formato string
	Values []float64 `json:"values"` // Valores totales
	High   float64   `json:"high"`   // Valor más alto en el período
	Low    float64   `json:"low"`    // Valor más bajo en el período
}
