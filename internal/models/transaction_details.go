package models

type TransactionDetails struct {
	Transaction    CryptoTransaction `json:"transaction"`
	CurrentPrice   float64          `json:"current_price"`
	CurrentValue   float64          `json:"current_value"`    // Amount * CurrentPrice
	GainLoss      float64          `json:"gain_loss"`        // CurrentValue - Total
	GainLossPercent float64        `json:"gain_loss_percent"` // (GainLoss / Total) * 100
} 