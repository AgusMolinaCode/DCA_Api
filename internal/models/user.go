package models

import (
	"time"
)

type User struct {
	ID        string    `json:"id"`
	Email     string    `json:"email"`
	Password  string    `json:"-"` // El "-" evita que se serialice en JSON
	Name      string    `json:"name"`
	CreatedAt time.Time `json:"created_at"`
} 