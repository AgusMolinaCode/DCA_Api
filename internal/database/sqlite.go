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
	return err
}
