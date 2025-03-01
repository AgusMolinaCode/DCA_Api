package models

type Holdings struct {
	TotalCurrentValue float64 `json:"total_current_value"` // Valor total actual de todas las criptomonedas
	TotalInvested     float64 `json:"total_invested"`      // Total invertido históricamente
	TotalProfit       float64 `json:"total_profit"`        // Ganancia o pérdida total
	ProfitPercentage  float64 `json:"profit_percentage"`   // Porcentaje de ganancia/pérdida
}

type HoldingDetail struct {
	Ticker           string  `json:"ticker"`
	Amount           float64 `json:"amount"`            // Cantidad de criptomoneda
	CurrentPrice     float64 `json:"current_price"`     // Precio actual
	Value            float64 `json:"value"`             // Valor actual (Amount * CurrentPrice)
	AverageBuyPrice  float64 `json:"avg_buy_price"`     // Precio promedio de compra
	TotalInvested    float64 `json:"total_invested"`    // Total invertido en esta moneda
	Profit           float64 `json:"profit"`            // Ganancia/pérdida para esta moneda
	ProfitPercentage float64 `json:"profit_percentage"` // Porcentaje de ganancia/pérdida
	Percentage       float64 `json:"percentage"`        // Porcentaje del portafolio total
}
