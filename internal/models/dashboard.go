package models

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
