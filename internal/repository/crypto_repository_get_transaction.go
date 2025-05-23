package repository

import (
	"github.com/AgusMolinaCode/DCA_Api.git/internal/models"
)

// GetTransaction obtiene una transacciu00f3n por su ID
func (r *CryptoRepository) GetTransaction(transactionID string) (*models.CryptoTransaction, error) {
	query := `
		SELECT id, user_id, crypto_name, ticker, amount, purchase_price, total, date, note
		FROM crypto_transactions
		WHERE id = ?
	`

	var transaction models.CryptoTransaction
	err := r.db.QueryRow(query, transactionID).Scan(
		&transaction.ID,
		&transaction.UserID,
		&transaction.CryptoName,
		&transaction.Ticker,
		&transaction.Amount,
		&transaction.PurchasePrice,
		&transaction.Total,
		&transaction.Date,
		&transaction.Note,
	)

	if err != nil {
		return nil, err
	}

	return &transaction, nil
}
