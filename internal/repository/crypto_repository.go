package repository

import (
	"database/sql"

	"github.com/AgusMolinaCode/DCA_Api.git/internal/models"
)

type CryptoRepository struct {
	db *sql.DB
}

func NewCryptoRepository(db *sql.DB) *CryptoRepository {
	return &CryptoRepository{db: db}
}

func (r *CryptoRepository) CreateTransaction(tx *models.CryptoTransaction) error {
	query := `
		INSERT INTO crypto_transactions (id, user_id, crypto_name, ticker, amount, purchase_price, total, date, note)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`

	_, err := r.db.Exec(query,
		tx.ID,
		tx.UserID,
		tx.CryptoName,
		tx.Ticker,
		tx.Amount,
		tx.PurchasePrice,
		tx.Total,
		tx.Date,
		tx.Note,
	)
	return err
}

func (r *CryptoRepository) GetUserTransactions(userID string) ([]models.CryptoTransaction, error) {
	query := `
		SELECT id, user_id, crypto_name, ticker, amount, purchase_price, total, date, note, created_at 
		FROM crypto_transactions 
		WHERE user_id = ?
		ORDER BY date DESC`

	rows, err := r.db.Query(query, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var transactions []models.CryptoTransaction
	for rows.Next() {
		var tx models.CryptoTransaction
		err := rows.Scan(
			&tx.ID,
			&tx.UserID,
			&tx.CryptoName,
			&tx.Ticker,
			&tx.Amount,
			&tx.PurchasePrice,
			&tx.Total,
			&tx.Date,
			&tx.Note,
			&tx.CreatedAt,
		)
		if err != nil {
			return nil, err
		}
		transactions = append(transactions, tx)
	}
	return transactions, nil
}
