package middleware

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

func GetUsers(c *gin.Context) {
	users, err := userRepo.GetAllUsers()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Error al obtener usuarios"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"users": users,
	})
}

func GetUser(c *gin.Context) {
	userId := c.Param("id")

	user, err := userRepo.GetUserById(userId)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Usuario no encontrado"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"user": user,
	})
}

func DeleteUserByAdmin(c *gin.Context) {
	userId := c.Param("id")

	// Verificar que el usuario exista
	_, err := userRepo.GetUserById(userId)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Usuario no encontrado"})
		return
	}

	// Eliminar el usuario
	if err := userRepo.DeleteUser(userId); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Error al eliminar usuario"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "Usuario eliminado exitosamente",
	})
}

func GetUserByEmail(c *gin.Context) {
	email := c.Param("email")

	user, err := userRepo.GetUserByEmail(email)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Usuario no encontrado"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"user": user,
	})
}
