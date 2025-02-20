package repository

import (
	"database/sql"
	"errors"

	"github.com/AgusMolinaCode/DCA_Api.git/internal/database"
	"github.com/AgusMolinaCode/DCA_Api.git/internal/models"
	"golang.org/x/crypto/bcrypt"
)

type UserRepository struct {
	db *sql.DB
}

func NewUserRepository() *UserRepository {
	return &UserRepository{
		db: database.DB,
	}
}

func (r *UserRepository) CreateUser(user *models.User) error {
	query := `
		INSERT INTO users (id, email, password, name)
		VALUES (?, ?, ?, ?)`

	_, err := r.db.Exec(query, user.ID, user.Email, user.Password, user.Name)
	return err
}

func (r *UserRepository) GetAllUsers() ([]models.User, error) {
	users := []models.User{}
	query := `SELECT id, email, name, created_at FROM users`

	rows, err := r.db.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var user models.User
		err := rows.Scan(
			&user.ID,
			&user.Email,
			&user.Name,
			&user.CreatedAt,
		)
		if err != nil {
			return nil, err
		}
		users = append(users, user)
	}

	return users, nil
}

func (r *UserRepository) GetUserById(id string) (*models.User, error) {
	user := &models.User{}
	query := `SELECT id, email, name, created_at FROM users WHERE id = ?`

	err := r.db.QueryRow(query, id).Scan(
		&user.ID,
		&user.Email,
		&user.Name,
		&user.CreatedAt,
	)

	if err == sql.ErrNoRows {
		return nil, errors.New("usuario no encontrado")
	}

	return user, err
}

func (r *UserRepository) GetUserByEmail(email string) (*models.User, error) {
	user := &models.User{}
	query := `SELECT id, email, password, name, created_at FROM users WHERE email = ?`

	err := r.db.QueryRow(query, email).Scan(
		&user.ID,
		&user.Email,
		&user.Password,
		&user.Name,
		&user.CreatedAt,
	)

	if err == sql.ErrNoRows {
		return nil, errors.New("usuario no encontrado")
	}

	return user, err
}

func (r *UserRepository) UpdateUser(user *models.User) error {
	query := `
		UPDATE users 
		SET email = ?, name = ?
		WHERE id = ?`

	_, err := r.db.Exec(query, user.Email, user.Name, user.ID)
	return err
}

func (r *UserRepository) DeleteUser(id string) error {
	query := `DELETE FROM users WHERE id = ?`

	_, err := r.db.Exec(query, id)
	return err
}

func (r *UserRepository) UpdatePassword(email, password string) error {
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return err
	}

	query := `UPDATE users SET password = ? WHERE email = ?`

	_, err = r.db.Exec(query, string(hashedPassword), email)
	return err
}
