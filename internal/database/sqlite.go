package database

import (
	"database/sql"
	"fmt"
	"os"

	_ "github.com/lib/pq"
)

var DB *sql.DB

func InitDB() error {
	// Configurar conexión a PostgreSQL
	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		// Configuración por defecto para desarrollo local
		host := getEnvOrDefault("DB_HOST", "localhost")
		port := getEnvOrDefault("DB_PORT", "5432")
		user := getEnvOrDefault("DB_USER", "postgres")
		password := getEnvOrDefault("DB_PASSWORD", "")
		dbname := getEnvOrDefault("DB_NAME", "dca_api")
		sslmode := getEnvOrDefault("DB_SSLMODE", "disable")
		
		dbURL = fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=%s sslmode=%s",
			host, port, user, password, dbname, sslmode)
	}

	var err error
	DB, err = sql.Open("postgres", dbURL)
	if err != nil {
		return err
	}

	// Verificar la conexión
	if err = DB.Ping(); err != nil {
		return err
	}

	// Crear tabla de usuarios si no existe
	createTableSQL := `
	CREATE TABLE IF NOT EXISTS users (
		id TEXT PRIMARY KEY,
		email TEXT UNIQUE NOT NULL,
		password TEXT NOT NULL,
		name TEXT NOT NULL,
		created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
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
		date TIMESTAMP NOT NULL,
		note TEXT,
		created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
		type TEXT DEFAULT 'compra',
		usdt_received REAL DEFAULT 0,
		image_url TEXT,
		FOREIGN KEY(user_id) REFERENCES users(id)
	);`

	_, err = DB.Exec(createCryptoTableSQL)
	if err != nil {
		return err
	}

	// Crear tabla de bolsas
	createBolsasTableSQL := `
	CREATE TABLE IF NOT EXISTS bolsas (
		id TEXT PRIMARY KEY,
		user_id TEXT NOT NULL,
		name TEXT NOT NULL,
		description TEXT,
		goal REAL,
		created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
		updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
		FOREIGN KEY(user_id) REFERENCES users(id)
	);`

	_, err = DB.Exec(createBolsasTableSQL)
	if err != nil {
		return err
	}

	// Crear tabla de activos en bolsas
	createAssetsInBolsaTableSQL := `
	CREATE TABLE IF NOT EXISTS assets_in_bolsa (
		id TEXT PRIMARY KEY,
		bolsa_id TEXT NOT NULL,
		crypto_name TEXT NOT NULL,
		ticker TEXT NOT NULL,
		amount REAL NOT NULL,
		purchase_price REAL NOT NULL,
		total REAL NOT NULL,
		image_url TEXT,
		created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
		updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
		FOREIGN KEY(bolsa_id) REFERENCES bolsas(id) ON DELETE CASCADE
	);`

	_, err = DB.Exec(createAssetsInBolsaTableSQL)
	if err != nil {
		return err
	}

	// Crear tabla de reglas de trigger
	createTriggerRulesTableSQL := `
	CREATE TABLE IF NOT EXISTS trigger_rules (
		id TEXT PRIMARY KEY,
		bolsa_id TEXT NOT NULL,
		type TEXT NOT NULL,
		ticker TEXT,
		target_value REAL NOT NULL,
		active INTEGER DEFAULT 1,
		triggered INTEGER DEFAULT 0,
		created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
		updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
		FOREIGN KEY(bolsa_id) REFERENCES bolsas(id) ON DELETE CASCADE
	);`

	_, err = DB.Exec(createTriggerRulesTableSQL)
	if err != nil {
		return err
	}

	// Crear tabla de etiquetas para bolsas
	createBolsaTagsTableSQL := `
	CREATE TABLE IF NOT EXISTS bolsa_tags (
		id TEXT PRIMARY KEY,
		bolsa_id TEXT NOT NULL,
		tag TEXT NOT NULL,
		created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
		UNIQUE(bolsa_id, tag),
		FOREIGN KEY(bolsa_id) REFERENCES bolsas(id) ON DELETE CASCADE
	);`

	_, err = DB.Exec(createBolsaTagsTableSQL)
	if err != nil {
		return err
	}

	// Crear tabla para almacenar el historial de inversiones
	createInvestmentHistoryTableSQL := `
	CREATE TABLE IF NOT EXISTS investment_snapshots (
		id TEXT PRIMARY KEY,
		user_id TEXT NOT NULL,
		date TIMESTAMP NOT NULL,
		total_value REAL NOT NULL,
		total_invested REAL NOT NULL,
		profit REAL NOT NULL,
		profit_percentage REAL NOT NULL,
		created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
		FOREIGN KEY(user_id) REFERENCES users(id) ON DELETE CASCADE
	);`

	_, err = DB.Exec(createInvestmentHistoryTableSQL)
	if err != nil {
		return err
	}

	// Crear índice para búsqueda rápida por usuario y fecha
	createInvestmentHistoryIndexSQL := `
	CREATE INDEX IF NOT EXISTS idx_investment_snapshots_user_date 
	ON investment_snapshots(user_id, date);`

	_, err = DB.Exec(createInvestmentHistoryIndexSQL)
	if err != nil {
		return err
	}

	// Ejecutar migraciones para actualizar el esquema
	err = RunMigrations()
	return err
}

func getEnvOrDefault(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}
