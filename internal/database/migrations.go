package database

import (
	"log"
)

// RunMigrations ejecuta las migraciones necesarias para actualizar el esquema de la base de datos
func RunMigrations() error {
	log.Println("Ejecutando migraciones de la base de datos...")

	// Migración para añadir campos max_value y min_value a la tabla investment_snapshots
	addMaxMinValueColumnsSQL := `
	ALTER TABLE investment_snapshots ADD COLUMN max_value REAL DEFAULT 0;
	ALTER TABLE investment_snapshots ADD COLUMN min_value REAL DEFAULT 0;
	`

	_, err := DB.Exec(addMaxMinValueColumnsSQL)
	if err != nil {
		log.Printf("Error al añadir columnas max_value y min_value: %v", err)
		// No retornamos error porque SQLite puede dar error si la columna ya existe
		// y queremos que la migración continúe
	} else {
		log.Println("Columnas max_value y min_value añadidas correctamente")
	}

	return nil
}
