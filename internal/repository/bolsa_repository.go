package repository

import (
	"database/sql"
	"time"

	"github.com/AgusMolinaCode/DCA_Api.git/internal/models"
	"github.com/AgusMolinaCode/DCA_Api.git/internal/services"
)

// BolsaRepository maneja las operaciones de base de datos para bolsas
type BolsaRepository struct {
	db *sql.DB
}

// NewBolsaRepository crea un nuevo repositorio de bolsas
func NewBolsaRepository(db *sql.DB) *BolsaRepository {
	return &BolsaRepository{
		db: db,
	}
}

// CreateBolsa crea una nueva bolsa
func (r *BolsaRepository) CreateBolsa(bolsa models.Bolsa) error {
	// Iniciar transacción SQL
	tx, err := r.db.Begin()
	if err != nil {
		return err
	}
	defer func() {
		if err != nil {
			tx.Rollback()
			return
		}
		err = tx.Commit()
	}()

	// Generar ID único para la bolsa si no tiene uno
	if bolsa.ID == "" {
		bolsa.ID = models.GenerateUUID()
	}

	// Establecer timestamps
	now := time.Now()
	bolsa.CreatedAt = now
	bolsa.UpdatedAt = now

	// Insertar la bolsa en la base de datos
	_, err = tx.Exec(
		`INSERT INTO bolsas (id, user_id, name, description, goal, created_at, updated_at) 
		VALUES (?, ?, ?, ?, ?, ?, ?)`,
		bolsa.ID, bolsa.UserID, bolsa.Name, bolsa.Description, bolsa.Goal, bolsa.CreatedAt, bolsa.UpdatedAt,
	)

	return err
}

// GetBolsaByID obtiene una bolsa por su ID
func (r *BolsaRepository) GetBolsaByID(id string) (*models.Bolsa, error) {
	var bolsa models.Bolsa

	// Obtener la bolsa
	err := r.db.QueryRow(
		`SELECT id, user_id, name, description, goal, created_at, updated_at 
		FROM bolsas WHERE id = ?`, id,
	).Scan(
		&bolsa.ID, &bolsa.UserID, &bolsa.Name, &bolsa.Description, &bolsa.Goal, &bolsa.CreatedAt, &bolsa.UpdatedAt,
	)

	if err != nil {
		return nil, err
	}

	// Obtener los activos de la bolsa
	rows, err := r.db.Query(
		`SELECT id, bolsa_id, crypto_name, ticker, amount, purchase_price, total, image_url, created_at, updated_at 
		FROM assets_in_bolsa WHERE bolsa_id = ?`, id,
	)

	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var assets []models.AssetInBolsa
	for rows.Next() {
		var asset models.AssetInBolsa
		err := rows.Scan(
			&asset.ID, &asset.BolsaID, &asset.CryptoName, &asset.Ticker, &asset.Amount,
			&asset.PurchasePrice, &asset.Total, &asset.ImageURL, &asset.CreatedAt, &asset.UpdatedAt,
		)
		if err != nil {
			return nil, err
		}

		// Obtener precio actual y calcular valores
		cryptoData, err := services.GetCryptoPriceFromCoinGecko(asset.Ticker)
		if err != nil {
			// Si no podemos obtener el precio actual, usamos el precio de compra
			asset.CurrentPrice = asset.PurchasePrice
		} else {
			asset.CurrentPrice = cryptoData.Price
		}

		asset.CurrentValue = asset.Amount * asset.CurrentPrice
		asset.GainLoss = asset.CurrentValue - asset.Total

		if asset.Total > 0 {
			asset.GainLossPercent = (asset.GainLoss / asset.Total) * 100
		}

		assets = append(assets, asset)
		bolsa.CurrentValue += asset.CurrentValue
	}

	bolsa.Assets = assets

	// Obtener las reglas de la bolsa
	rows, err = r.db.Query(
		`SELECT id, bolsa_id, type, ticker, target_value, active, triggered, created_at, updated_at 
		FROM trigger_rules WHERE bolsa_id = ?`, id,
	)

	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var rules []models.TriggerRule
	for rows.Next() {
		var rule models.TriggerRule
		var active, triggered int
		err := rows.Scan(
			&rule.ID, &rule.BolsaID, &rule.Type, &rule.Ticker, &rule.TargetValue,
			&active, &triggered, &rule.CreatedAt, &rule.UpdatedAt,
		)
		if err != nil {
			return nil, err
		}

		// Convertir enteros a booleanos
		rule.Active = active == 1
		rule.Triggered = triggered == 1

		rules = append(rules, rule)
	}

	bolsa.Rules = rules

	return &bolsa, nil
}

// GetBolsasByUserID obtiene todas las bolsas de un usuario
func (r *BolsaRepository) GetBolsasByUserID(userID string) ([]models.Bolsa, error) {
	// Obtener las bolsas del usuario
	rows, err := r.db.Query(
		`SELECT id, user_id, name, description, goal, created_at, updated_at 
		FROM bolsas WHERE user_id = ?`, userID,
	)

	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var bolsas []models.Bolsa
	for rows.Next() {
		var bolsa models.Bolsa
		err := rows.Scan(
			&bolsa.ID, &bolsa.UserID, &bolsa.Name, &bolsa.Description, &bolsa.Goal, &bolsa.CreatedAt, &bolsa.UpdatedAt,
		)
		if err != nil {
			return nil, err
		}

		// Obtener los activos de la bolsa
		assetsRows, err := r.db.Query(
			`SELECT id, bolsa_id, crypto_name, ticker, amount, purchase_price, total, image_url, created_at, updated_at 
			FROM assets_in_bolsa WHERE bolsa_id = ?`, bolsa.ID,
		)

		if err != nil {
			return nil, err
		}

		var assets []models.AssetInBolsa
		for assetsRows.Next() {
			var asset models.AssetInBolsa
			err := assetsRows.Scan(
				&asset.ID, &asset.BolsaID, &asset.CryptoName, &asset.Ticker, &asset.Amount,
				&asset.PurchasePrice, &asset.Total, &asset.ImageURL, &asset.CreatedAt, &asset.UpdatedAt,
			)
			if err != nil {
				assetsRows.Close()
				return nil, err
			}

			// Obtener precio actual y calcular valores
			cryptoData, err := services.GetCryptoPriceFromCoinGecko(asset.Ticker)
			if err != nil {
				// Si no podemos obtener el precio actual, usamos el precio de compra
				asset.CurrentPrice = asset.PurchasePrice
			} else {
				asset.CurrentPrice = cryptoData.Price
			}

			asset.CurrentValue = asset.Amount * asset.CurrentPrice
			asset.GainLoss = asset.CurrentValue - asset.Total

			if asset.Total > 0 {
				asset.GainLossPercent = (asset.GainLoss / asset.Total) * 100
			}

			assets = append(assets, asset)
			bolsa.CurrentValue += asset.CurrentValue
		}
		assetsRows.Close()

		bolsa.Assets = assets

		// Obtener las reglas de la bolsa
		rulesRows, err := r.db.Query(
			`SELECT id, bolsa_id, type, ticker, target_value, active, triggered, created_at, updated_at 
			FROM trigger_rules WHERE bolsa_id = ?`, bolsa.ID,
		)

		if err != nil {
			return nil, err
		}

		var rules []models.TriggerRule
		for rulesRows.Next() {
			var rule models.TriggerRule
			var active, triggered int
			err := rulesRows.Scan(
				&rule.ID, &rule.BolsaID, &rule.Type, &rule.Ticker, &rule.TargetValue,
				&active, &triggered, &rule.CreatedAt, &rule.UpdatedAt,
			)
			if err != nil {
				rulesRows.Close()
				return nil, err
			}

			// Convertir enteros a booleanos
			rule.Active = active == 1
			rule.Triggered = triggered == 1

			rules = append(rules, rule)
		}
		rulesRows.Close()

		bolsa.Rules = rules

		bolsas = append(bolsas, bolsa)
	}

	return bolsas, nil
}

// AddAssetToBolsa añade un activo a una bolsa
func (r *BolsaRepository) AddAssetToBolsa(asset models.AssetInBolsa) error {
	// Iniciar transacción SQL
	tx, err := r.db.Begin()
	if err != nil {
		return err
	}
	defer func() {
		if err != nil {
			tx.Rollback()
			return
		}
		err = tx.Commit()
	}()

	// Generar ID único para el activo si no tiene uno
	if asset.ID == "" {
		asset.ID = models.GenerateUUID()
	}

	// Establecer timestamps
	now := time.Now()
	asset.CreatedAt = now
	asset.UpdatedAt = now

	// Insertar el activo en la base de datos
	_, err = tx.Exec(
		`INSERT INTO assets_in_bolsa (id, bolsa_id, crypto_name, ticker, amount, purchase_price, total, image_url, created_at, updated_at) 
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		asset.ID, asset.BolsaID, asset.CryptoName, asset.Ticker, asset.Amount,
		asset.PurchasePrice, asset.Total, asset.ImageURL, asset.CreatedAt, asset.UpdatedAt,
	)

	return err
}

// AddRuleToBolsa añade una regla a una bolsa
func (r *BolsaRepository) AddRuleToBolsa(rule models.TriggerRule) error {
	// Iniciar transacción SQL
	tx, err := r.db.Begin()
	if err != nil {
		return err
	}
	defer func() {
		if err != nil {
			tx.Rollback()
			return
		}
		err = tx.Commit()
	}()

	// Generar ID único para la regla si no tiene uno
	if rule.ID == "" {
		rule.ID = models.GenerateUUID()
	}

	// Establecer timestamps
	now := time.Now()
	rule.CreatedAt = now
	rule.UpdatedAt = now

	// Convertir booleanos a enteros para SQLite
	active := 0
	if rule.Active {
		active = 1
	}

	triggered := 0
	if rule.Triggered {
		triggered = 1
	}

	// Insertar la regla en la base de datos
	_, err = tx.Exec(
		`INSERT INTO trigger_rules (id, bolsa_id, type, ticker, target_value, active, triggered, created_at, updated_at) 
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		rule.ID, rule.BolsaID, rule.Type, rule.Ticker, rule.TargetValue,
		active, triggered, rule.CreatedAt, rule.UpdatedAt,
	)

	return err
}

// UpdateRule actualiza una regla existente
func (r *BolsaRepository) UpdateRule(rule models.TriggerRule) error {
	// Iniciar transacción SQL
	tx, err := r.db.Begin()
	if err != nil {
		return err
	}
	defer func() {
		if err != nil {
			tx.Rollback()
			return
		}
		err = tx.Commit()
	}()

	// Establecer timestamp de actualización
	rule.UpdatedAt = time.Now()

	// Convertir booleanos a enteros para SQLite
	active := 0
	if rule.Active {
		active = 1
	}

	triggered := 0
	if rule.Triggered {
		triggered = 1
	}

	// Actualizar la regla en la base de datos
	_, err = tx.Exec(
		`UPDATE trigger_rules SET 
			type = ?, 
			ticker = ?, 
			target_value = ?, 
			active = ?, 
			triggered = ?, 
			updated_at = ? 
		WHERE id = ?`,
		rule.Type, rule.Ticker, rule.TargetValue, active, triggered, rule.UpdatedAt, rule.ID,
	)

	return err
}

// UpdateBolsa actualiza los datos de una bolsa existente
func (r *BolsaRepository) UpdateBolsa(bolsa models.Bolsa) error {
	// Iniciar transacción SQL
	tx, err := r.db.Begin()
	if err != nil {
		return err
	}
	defer func() {
		if err != nil {
			tx.Rollback()
			return
		}
		err = tx.Commit()
	}()

	// Actualizar la bolsa en la base de datos
	_, err = tx.Exec(
		`UPDATE bolsas SET 
			name = ?, 
			description = ?, 
			goal = ?, 
			updated_at = ? 
		WHERE id = ?`,
		bolsa.Name, bolsa.Description, bolsa.Goal, time.Now(), bolsa.ID,
	)

	return err
}

// UpdateAsset actualiza un activo existente en una bolsa
func (r *BolsaRepository) UpdateAsset(asset models.AssetInBolsa) error {
	// Iniciar transacción SQL
	tx, err := r.db.Begin()
	if err != nil {
		return err
	}
	defer func() {
		if err != nil {
			tx.Rollback()
			return
		}
		err = tx.Commit()
	}()

	// Actualizar el activo en la base de datos
	_, err = tx.Exec(
		`UPDATE assets_in_bolsa SET 
			crypto_name = ?, 
			ticker = ?, 
			amount = ?, 
			purchase_price = ?, 
			total = ?, 
			image_url = ?, 
			updated_at = ? 
		WHERE id = ?`,
		asset.CryptoName, asset.Ticker, asset.Amount, asset.PurchasePrice,
		asset.Total, asset.ImageURL, time.Now(), asset.ID,
	)

	return err
}

// AddTagToBolsa añade una etiqueta a una bolsa
func (r *BolsaRepository) AddTagToBolsa(bolsaID string, tag string) error {
	// Iniciar transacción SQL
	tx, err := r.db.Begin()
	if err != nil {
		return err
	}
	defer func() {
		if err != nil {
			tx.Rollback()
			return
		}
		err = tx.Commit()
	}()

	// Generar ID único para la etiqueta
	tagID := models.GenerateUUID()

	// Insertar la etiqueta en la base de datos
	_, err = tx.Exec(
		"INSERT INTO bolsa_tags (id, bolsa_id, tag) VALUES (?, ?, ?)",
		tagID, bolsaID, tag,
	)

	return err
}

// RemoveTagFromBolsa elimina una etiqueta de una bolsa
func (r *BolsaRepository) RemoveTagFromBolsa(bolsaID string, tag string) error {
	// Iniciar transacción SQL
	tx, err := r.db.Begin()
	if err != nil {
		return err
	}
	defer func() {
		if err != nil {
			tx.Rollback()
			return
		}
		err = tx.Commit()
	}()

	// Eliminar la etiqueta de la base de datos
	_, err = tx.Exec(
		"DELETE FROM bolsa_tags WHERE bolsa_id = ? AND tag = ?",
		bolsaID, tag,
	)

	return err
}

// GetBolsasByTag obtiene todas las bolsas que tienen una etiqueta específica
func (r *BolsaRepository) GetBolsasByTag(userID string, tag string) ([]models.Bolsa, error) {
	rows, err := r.db.Query(
		`SELECT DISTINCT b.* FROM bolsas b 
		JOIN bolsa_tags t ON b.id = t.bolsa_id 
		WHERE b.user_id = ? AND t.tag = ?`,
		userID, tag,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var bolsas []models.Bolsa

	for rows.Next() {
		var bolsa models.Bolsa
		err := rows.Scan(
			&bolsa.ID,
			&bolsa.UserID,
			&bolsa.Name,
			&bolsa.Description,
			&bolsa.Goal,
			&bolsa.CreatedAt,
			&bolsa.UpdatedAt,
		)
		if err != nil {
			return nil, err
		}

		// Obtener los activos de la bolsa
		assets, err := r.getAssetsForBolsa(bolsa.ID)
		if err != nil {
			return nil, err
		}
		bolsa.Assets = assets

		// Calcular el valor actual de la bolsa
		bolsa.CurrentValue = 0
		for _, asset := range assets {
			bolsa.CurrentValue += asset.CurrentValue
		}

		// Obtener las etiquetas de la bolsa
		tags, err := r.getTagsForBolsa(bolsa.ID)
		if err != nil {
			return nil, err
		}
		bolsa.Tags = tags

		// Obtener las reglas de la bolsa
		rules, err := r.getRulesForBolsa(bolsa.ID)
		if err != nil {
			return nil, err
		}
		bolsa.Rules = rules

		bolsas = append(bolsas, bolsa)
	}

	return bolsas, nil
}

// getTagsForBolsa obtiene todas las etiquetas de una bolsa
func (r *BolsaRepository) getTagsForBolsa(bolsaID string) ([]string, error) {
	rows, err := r.db.Query(
		"SELECT tag FROM bolsa_tags WHERE bolsa_id = ?",
		bolsaID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var tags []string

	for rows.Next() {
		var tag string
		err := rows.Scan(&tag)
		if err != nil {
			return nil, err
		}
		tags = append(tags, tag)
	}

	return tags, nil
}

// getRulesForBolsa obtiene todas las reglas de una bolsa
func (r *BolsaRepository) getRulesForBolsa(bolsaID string) ([]models.TriggerRule, error) {
	rows, err := r.db.Query(
		`SELECT id, bolsa_id, type, ticker, target_value, active, triggered, created_at, updated_at 
		FROM trigger_rules WHERE bolsa_id = ?`, bolsaID,
	)

	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var rules []models.TriggerRule
	for rows.Next() {
		var rule models.TriggerRule
		var active, triggered int
		err := rows.Scan(
			&rule.ID, &rule.BolsaID, &rule.Type, &rule.Ticker, &rule.TargetValue,
			&active, &triggered, &rule.CreatedAt, &rule.UpdatedAt,
		)
		if err != nil {
			return nil, err
		}

		// Convertir enteros a booleanos
		rule.Active = active == 1
		rule.Triggered = triggered == 1

		rules = append(rules, rule)
	}

	return rules, nil
}

// getAssetsForBolsa obtiene todos los activos de una bolsa
func (r *BolsaRepository) getAssetsForBolsa(bolsaID string) ([]models.AssetInBolsa, error) {
	rows, err := r.db.Query(
		`SELECT id, bolsa_id, crypto_name, ticker, amount, purchase_price, total, image_url, created_at, updated_at 
		FROM assets_in_bolsa WHERE bolsa_id = ?`, bolsaID,
	)

	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var assets []models.AssetInBolsa
	for rows.Next() {
		var asset models.AssetInBolsa
		err := rows.Scan(
			&asset.ID, &asset.BolsaID, &asset.CryptoName, &asset.Ticker, &asset.Amount,
			&asset.PurchasePrice, &asset.Total, &asset.ImageURL, &asset.CreatedAt, &asset.UpdatedAt,
		)
		if err != nil {
			return nil, err
		}

		// Obtener precio actual y calcular valores
		cryptoData, err := services.GetCryptoPriceFromCoinGecko(asset.Ticker)
		if err != nil {
			// Si no podemos obtener el precio actual, usamos el precio de compra
			asset.CurrentPrice = asset.PurchasePrice
		} else {
			asset.CurrentPrice = cryptoData.Price
		}

		asset.CurrentValue = asset.Amount * asset.CurrentPrice
		asset.GainLoss = asset.CurrentValue - asset.Total

		if asset.Total > 0 {
			asset.GainLossPercent = (asset.GainLoss / asset.Total) * 100
		}

		assets = append(assets, asset)
	}

	return assets, nil
}
