package middleware

import (
	"fmt"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/AgusMolinaCode/DCA_Api.git/internal/models"
	"github.com/AgusMolinaCode/DCA_Api.git/internal/repository"
	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"
)

// Estructura para almacenar usuarios
type User struct {
	ID       string
	Email    string
	Password string
	Name     string
}

// Almacén de usuarios en memoria
var (
	users = make(map[string]User) // email -> user
	mutex sync.RWMutex
)

var userRepo *repository.UserRepository

// Estructura para almacenar tokens revocados
var (
	revokedTokens = make(map[string]time.Time)
	tokenMutex    sync.RWMutex
)

func InitAuth() {
	userRepo = repository.NewUserRepository()
}

// Función para revocar un token
func revokeToken(token string) {
	tokenMutex.Lock()
	defer tokenMutex.Unlock()
	revokedTokens[token] = time.Now()
}

// Función para limpiar tokens expirados
func cleanupRevokedTokens() {
	tokenMutex.Lock()
	defer tokenMutex.Unlock()
	now := time.Now()
	for token, revokedAt := range revokedTokens {
		// Eliminar tokens revocados hace más de 24 horas
		if now.Sub(revokedAt) > 24*time.Hour {
			delete(revokedTokens, token)
		}
	}
}

// Modificar el middleware de autenticación para verificar tokens revocados
func AuthMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Token no proporcionado"})
			c.Abort()
			return
		}

		tokenString := strings.Replace(authHeader, "Bearer ", "", 1)

		// Verificar si el token está revocado
		tokenMutex.RLock()
		_, isRevoked := revokedTokens[tokenString]
		tokenMutex.RUnlock()

		if isRevoked {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Token inválido o revocado"})
			c.Abort()
			return
		}

		token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
			return []byte(os.Getenv("JWT_SECRET")), nil
		})

		if err != nil || !token.Valid {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Token inválido"})
			c.Abort()
			return
		}

		claims := token.Claims.(jwt.MapClaims)
		c.Set("userId", claims["userId"])
		c.Next()
	}
}

func GenerateToken(userId string) (string, error) {
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"userId": userId,
		"exp":    time.Now().Add(time.Hour * 1).Unix(),
	})

	return token.SignedString([]byte(os.Getenv("JWT_SECRET")))
}

func GenerateResetToken(email string) (string, error) {
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"email": email,
		"exp":   time.Now().Add(time.Hour * 24).Unix(),
	})

	return token.SignedString([]byte(os.Getenv("JWT_SECRET")))
}

func Login(c *gin.Context) {
	var login struct {
		Email    string `json:"email" binding:"required,email"`
		Password string `json:"password" binding:"required"`
	}

	if err := c.ShouldBindJSON(&login); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Verificar si el usuario existe
	user, err := userRepo.GetUserByEmail(login.Email)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Usuario no encontrado"})
		return
	}

	// Verificar la contraseña
	if err := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(login.Password)); err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Contraseña incorrecta"})
		return
	}

	token, err := GenerateToken(user.ID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Error al generar el token"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "Inicio de sesión exitoso",
		"token":   token,
		"user": gin.H{
			"id":    user.ID,
			"email": user.Email,
			"name":  user.Name,
		},
	})
}

func Signup(c *gin.Context) {
	var signup struct {
		Email    string `json:"email" binding:"required,email"`
		Password string `json:"password" binding:"required,min=6"`
		Name     string `json:"name" binding:"required"`
	}

	if err := c.ShouldBindJSON(&signup); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Verificar si el email ya está registrado
	mutex.RLock()
	_, exists := users[signup.Email]
	mutex.RUnlock()
	if exists {
		c.JSON(http.StatusConflict, gin.H{"error": "El email ya está registrado"})
		return
	}

	// Hash de la contraseña
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(signup.Password), bcrypt.DefaultCost)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Error al procesar la contraseña"})
		return
	}

	// Crear nuevo usuario
	user := &models.User{
		ID:       generateUserId(),
		Email:    signup.Email,
		Password: string(hashedPassword),
		Name:     signup.Name,
	}

	// Guardar usuario en la base de datos
	if err := userRepo.CreateUser(user); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Error al crear usuario"})
		return
	}

	token, err := GenerateToken(user.ID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Error al generar el token"})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"message": "Registro exitoso",
		"token":   token,
		"user": gin.H{
			"id":    user.ID,
			"email": user.Email,
			"name":  user.Name,
		},
	})
}

// Función auxiliar para generar IDs de usuario
func generateUserId() string {
	return fmt.Sprintf("%d", time.Now().UnixNano())
}

// Nueva función de logout
func Logout(c *gin.Context) {
	authHeader := c.GetHeader("Authorization")
	if authHeader == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Token no proporcionado"})
		return
	}

	tokenString := strings.Replace(authHeader, "Bearer ", "", 1)

	// Revocar el token
	revokeToken(tokenString)

	// Limpiar tokens expirados
	go cleanupRevokedTokens()

	// Establecer encabezados para CORS
	c.Header("Access-Control-Allow-Origin", "http://localhost:3000")
	c.Header("Access-Control-Allow-Credentials", "true")

	c.JSON(http.StatusOK, gin.H{
		"message": "Sesión cerrada exitosamente",
	})
}
