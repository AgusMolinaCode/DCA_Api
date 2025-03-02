package models

// Holdings representa el resumen de las tenencias del usuario
type Holdings struct {
	TotalCurrentValue float64        `json:"total_current_value"` // Valor total actual de todas las criptomonedas
	TotalInvested     float64        `json:"total_invested"`      // Total invertido históricamente
	TotalProfit       float64        `json:"total_profit"`        // Ganancia o pérdida total
	ProfitPercentage  float64        `json:"profit_percentage"`   // Porcentaje de ganancia/pérdida
	Distribution      []CryptoWeight `json:"distribution"`        // Para el gráfico de torta
	ChartData         PieChartData   `json:"chart_data"`          // Datos formateados para el gráfico de torta
}

// CryptoWeight representa el peso de una criptomoneda en el portafolio
type CryptoWeight struct {
	Ticker       string         `json:"ticker"`
	Name         string         `json:"name"`
	Value        float64        `json:"value"`  // Valor actual en USD
	Weight       float64        `json:"weight"` // Porcentaje del portafolio (0-100)
	Color        string         `json:"color,omitempty"`
	IsOthers     bool           `json:"is_others,omitempty"`     // Indica si es la categoría "OTROS"
	OthersDetail []CryptoWeight `json:"others_detail,omitempty"` // Nuevo campo para detalles de criptomonedas menores
}

// PieChartData contiene los datos formateados para un gráfico de torta
type PieChartData struct {
	Labels   []string  `json:"labels"`   // Etiquetas (tickers)
	Values   []float64 `json:"values"`   // Valores (porcentajes)
	Colors   []string  `json:"colors"`   // Colores para cada segmento
	Currency string    `json:"currency"` // Moneda (USD)
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
