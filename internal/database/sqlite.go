package database

import (
	"database/sql"
	"os"
	"path/filepath"

	_ "github.com/mattn/go-sqlite3"
)

var DB *sql.DB

func InitDB() error {
	// Crear el directorio database si no existe
	if err := os.MkdirAll("database", 0755); err != nil {
		return err
	}

	var err error
	DB, err = sql.Open("sqlite3", filepath.Join("database", "users.db"))
	if err != nil {
		return err
	}

	// Crear tabla de usuarios si no existe
	createTableSQL := `
	CREATE TABLE IF NOT EXISTS users (
		id TEXT PRIMARY KEY,
		email TEXT UNIQUE NOT NULL,
		password TEXT NOT NULL,
		name TEXT NOT NULL,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP
	);`

	_, err = DB.Exec(createTableSQL)
	if err != nil {
		return err
	}

	// Crear tabla de transacciones crypto
	createCryptoTableSQL := `
	CREATE TABLE IF NOT EXISTS crypto_transactions (
		id TEXT PRIMARY KEY,
		user_id TEXT NOT NULL,
		crypto_name TEXT NOT NULL,
		ticker TEXT NOT NULL,
		amount REAL NOT NULL,
		purchase_price REAL NOT NULL,
		total REAL NOT NULL,
		date DATETIME NOT NULL,
		note TEXT,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		type TEXT DEFAULT 'compra',
		usdt_received REAL DEFAULT 0,
		FOREIGN KEY(user_id) REFERENCES users(id)
	);`

	_, err = DB.Exec(createCryptoTableSQL)
	return err
}
