package middleware

import (
	"net/http"
	"os"

	"github.com/gin-gonic/gin"
)

func AdminAuth() gin.HandlerFunc {
	return func(c *gin.Context) {
		adminKey := c.GetHeader("Admin-Key")
		if adminKey != os.Getenv("ADMIN_SECRET_KEY") {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Acceso no autorizado"})
			c.Abort()
			return
		}
		c.Next()
	}
}
