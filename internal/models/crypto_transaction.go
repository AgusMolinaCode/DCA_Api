package models

import "time"

type CryptoTransaction struct {
	ID            string    `json:"id"`
	UserID        string    `json:"user_id"`
	CryptoName    string    `json:"crypto_name" binding:"required"`
	Ticker        string    `json:"ticker" binding:"required"`
	Amount        float64   `json:"amount" binding:"required,gt=0"`
	PurchasePrice float64   `json:"purchase_price" binding:"required,gt=0"`
	Total         float64   `json:"total"`
	Date          time.Time `json:"date"`
	Note          string    `json:"note,omitempty"`
	CreatedAt     time.Time `json:"created_at"`
} 