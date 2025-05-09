package models

import "time"

type CryptoDashboard struct {
	Ticker        string  `json:"ticker"`
	CryptoName    string  `json:"crypto_name"` // Nuevo campo para el nombre de la criptomoneda
	ImageURL      string  `json:"image_url"`   // Nuevo campo para la URL de la imagen
	TotalInvested float64 `json:"total_invested"`
	Holdings      float64 `json:"holdings"`
	AvgPrice      float64 `json:"avg_price"`
	CurrentPrice  float64 `json:"current_price"`
	CurrentProfit float64 `json:"current_profit"`
	ProfitPercent float64 `json:"profit_percent"`
}

// DailyValue representa el valor total de las inversiones en un día específico
type DailyValue struct {
	Date             string  `json:"date"`
	TotalValue       float64 `json:"total_value"`
	ChangePercentage float64 `json:"change_percentage"`
}

// InvestmentHistory representa el historial de inversiones a lo largo del tiempo
type InvestmentHistory struct {
	StartDate       time.Time    `json:"start_date"`
	History         []DailyValue `json:"history"`
	TrendPercentage float64      `json:"trend_percentage"`
}

// InvestmentSnapshot representa un registro del valor total de las inversiones en un momento específico
type InvestmentSnapshot struct {
	ID               string    `json:"id"`
	UserID           string    `json:"user_id"`
	Date             time.Time `json:"date"`
	TotalValue       float64   `json:"total_value"`
	TotalInvested    float64   `json:"total_invested"`
	Profit           float64   `json:"profit"`
	ProfitPercentage float64   `json:"profit_percentage"`
}
