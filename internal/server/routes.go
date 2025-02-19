package routes

import (
	"net/http"

	"github.com/AgusMolinaCode/DCA_Api.git/internal/middleware"
	"github.com/gin-gonic/gin"
)

func RegisterRoutes(router *gin.Engine) {
	router.POST("/signup", Signup)
	router.POST("/login", Login)

	protected := router.Group("/")
	protected.Use(middleware.AuthMiddleware())
	{
		// protected.GET("/events", getEvents)
		// protected.POST("/events", createEvent)
		// ... otras rutas protegidas
	}
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

	// Aquí deberías verificar las credenciales contra tu base de datos
	// Por ahora, simulamos un login exitoso
	userId := "123" // Este ID vendría de tu base de datos

	token, err := middleware.GenerateToken(userId)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Error generando token"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"token": token,
		"user": gin.H{
			"id":    userId,
			"email": login.Email,
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

	// Aquí deberías crear el usuario en tu base de datos
	// Por ahora, simulamos un registro exitoso
	userId := "123"

	token, err := middleware.GenerateToken(userId)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Error generando token"})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"token": token,
		"user": gin.H{
			"id":    userId,
			"email": signup.Email,
			"name":  signup.Name,
		},
	})
}
