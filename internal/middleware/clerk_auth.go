package middleware

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/AgusMolinaCode/DCA_Api.git/internal/models"
	"github.com/AgusMolinaCode/DCA_Api.git/internal/repository"
	"github.com/clerk/clerk-sdk-go/v2"
	"github.com/clerk/clerk-sdk-go/v2/jwt"
	"github.com/clerk/clerk-sdk-go/v2/user"
	"github.com/gin-gonic/gin"
)

var userClient *user.Client

// InitClerk initializes the Clerk client using the recommended pattern
func InitClerk() {
	secretKey := os.Getenv("CLERK_SECRET_KEY")
	if secretKey == "" {
		panic("CLERK_SECRET_KEY environment variable is required")
	}
	
	// Set global Clerk key (recommended approach)
	clerk.SetKey(secretKey)
	
	// Also initialize user client for API operations
	config := &clerk.ClientConfig{}
	config.Key = &secretKey
	userClient = user.NewClient(config)
}

// ClerkAuthMiddleware validates Clerk JWT tokens using the proper SDK approach
func ClerkAuthMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Token no proporcionado"})
			c.Abort()
			return
		}

		tokenString := strings.Replace(authHeader, "Bearer ", "", 1)

		// Verify the JWT token with Clerk using proper SDK method
		claims, err := jwt.Verify(c.Request.Context(), &jwt.VerifyParams{
			Token: tokenString,
		})
		
		if err != nil {
			log.Printf("JWT verification failed: %v", err)
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Token inválido"})
			c.Abort()
			return
		}

		// Extract user ID from claims (Subject contains the user ID)
		userID := claims.Subject
		if userID == "" {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Token inválido: no se pudo extraer el ID del usuario"})
			c.Abort()
			return
		}

		// Store both user ID and full claims in context
		c.Set("userId", userID)
		c.Set("clerkClaims", claims)
		
		log.Printf("User authenticated: %s", userID)
		c.Next()
	}
}

// GetUserFromClerk retrieves user information from Clerk
func GetUserFromClerk(c *gin.Context) {
	userID := c.GetString("userId")
	if userID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "ID de usuario no encontrado"})
		return
	}

	user, err := userClient.Get(c.Request.Context(), userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Error al obtener información del usuario"})
		return
	}

	var email string
	if len(user.EmailAddresses) > 0 {
		email = user.EmailAddresses[0].EmailAddress
	}

	c.JSON(http.StatusOK, gin.H{
		"user": gin.H{
			"id":          user.ID,
			"email":       email,
			"first_name":  user.FirstName,
			"last_name":   user.LastName,
			"created_at":  user.CreatedAt,
			"updated_at":  user.UpdatedAt,
		},
	})
}

// WebhookHandler handles Clerk webhooks for user events
func ClerkWebhookHandler(c *gin.Context) {
	// Get the webhook signing secret from environment
	webhookSecret := os.Getenv("CLERK_WEBHOOK_SECRET")
	if webhookSecret == "" {
		log.Printf("CLERK_WEBHOOK_SECRET environment variable is not set")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Webhook secret not configured"})
		return
	}

	// Read the raw body
	body, err := io.ReadAll(c.Request.Body)
	if err != nil {
		log.Printf("Error reading request body: %v", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": "Could not read request body"})
		return
	}

	// Verify the webhook signature
	if !verifyWebhookSignature(body, c.GetHeader("svix-signature"), webhookSecret) {
		log.Printf("Invalid webhook signature")
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid webhook signature"})
		return
	}

	// Parse the webhook payload from the body we already read
	var webhookData map[string]interface{}
	if err := json.Unmarshal(body, &webhookData); err != nil {
		log.Printf("Error parsing JSON payload: %v", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid JSON payload"})
		return
	}

	// Extract the event type
	eventType, ok := webhookData["type"].(string)
	if !ok {
		log.Printf("Missing event type in webhook payload")
		c.JSON(http.StatusBadRequest, gin.H{"error": "Missing event type"})
		return
	}

	log.Printf("Received webhook event: %s", eventType)

	// Handle different event types
	switch eventType {
	case "user.created":
		handleUserCreated(c, webhookData)
	case "user.updated":
		handleUserUpdated(c, webhookData)
	case "user.deleted":
		handleUserDeleted(c, webhookData)
	default:
		// For other events, just return success
		log.Printf("Event type %s not handled", eventType)
		c.JSON(http.StatusOK, gin.H{"message": "Event received but not handled"})
	}
}

// verifyWebhookSignature verifies the Clerk webhook signature
func verifyWebhookSignature(body []byte, signature string, secret string) bool {
	if signature == "" {
		return false
	}

	// Parse the signature header
	parts := strings.Split(signature, ",")
	var timestamp, sig string
	
	for _, part := range parts {
		if strings.HasPrefix(part, "t=") {
			timestamp = strings.TrimPrefix(part, "t=")
		} else if strings.HasPrefix(part, "v1=") {
			sig = strings.TrimPrefix(part, "v1=")
		}
	}

	if timestamp == "" || sig == "" {
		return false
	}

	// Verify timestamp (should be within 5 minutes)
	ts, err := strconv.ParseInt(timestamp, 10, 64)
	if err != nil {
		return false
	}
	
	now := time.Now().Unix()
	if now-ts > 300 { // 5 minutes tolerance
		return false
	}

	// Create the expected signature
	signedPayload := fmt.Sprintf("%s.%s", timestamp, body)
	expectedSig := computeSignature(signedPayload, secret)
	
	// Compare signatures
	return hmac.Equal([]byte(sig), []byte(expectedSig))
}

// computeSignature computes the HMAC signature for the webhook
func computeSignature(payload string, secret string) string {
	// Decode the base64-encoded secret
	decodedSecret, err := base64.StdEncoding.DecodeString(secret)
	if err != nil {
		log.Printf("Error decoding webhook secret: %v", err)
		return ""
	}

	// Create HMAC with SHA256
	h := hmac.New(sha256.New, decodedSecret)
	h.Write([]byte(payload))
	
	// Return hex-encoded signature
	return hex.EncodeToString(h.Sum(nil))
}

// handleUserCreated creates a new user in the database when they sign up through Clerk
func handleUserCreated(c *gin.Context, webhookData map[string]interface{}) {
	// Extract user data from webhook payload
	data, ok := webhookData["data"].(map[string]interface{})
	if !ok {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid webhook data structure"})
		return
	}

	// Extract user ID
	userID, ok := data["id"].(string)
	if !ok {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Missing user ID"})
		return
	}

	// Extract email addresses
	emailAddresses, ok := data["email_addresses"].([]interface{})
	if !ok || len(emailAddresses) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Missing email addresses"})
		return
	}

	// Get primary email
	primaryEmail := ""
	for _, emailAddr := range emailAddresses {
		if emailMap, ok := emailAddr.(map[string]interface{}); ok {
			if emailMap["email_address"] != nil {
				primaryEmail = emailMap["email_address"].(string)
				break
			}
		}
	}

	if primaryEmail == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "No valid email found"})
		return
	}

	// Extract name information
	firstName, _ := data["first_name"].(string)
	lastName, _ := data["last_name"].(string)
	
	// Combine first and last name
	fullName := strings.TrimSpace(firstName + " " + lastName)
	if fullName == "" {
		fullName = strings.Split(primaryEmail, "@")[0] // Use email username as fallback
	}

	// Create user in database
	userRepo := repository.NewUserRepository()
	user := &models.User{
		ID:        userID,
		Email:     primaryEmail,
		Name:      fullName,
		Password:  "", // No password needed for Clerk users
		CreatedAt: time.Now(),
	}

	err := userRepo.CreateUser(user)
	if err != nil {
		log.Printf("Error creating user in database: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create user"})
		return
	}

	log.Printf("User created successfully: ID=%s, Email=%s, Name=%s", userID, primaryEmail, fullName)
	c.JSON(http.StatusOK, gin.H{"message": "User created successfully"})
}

// handleUserUpdated updates user information in the database
func handleUserUpdated(c *gin.Context, webhookData map[string]interface{}) {
	// Extract user data from webhook payload
	data, ok := webhookData["data"].(map[string]interface{})
	if !ok {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid webhook data structure"})
		return
	}

	// Extract user ID
	userID, ok := data["id"].(string)
	if !ok {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Missing user ID"})
		return
	}

	// Extract email addresses
	emailAddresses, ok := data["email_addresses"].([]interface{})
	if !ok || len(emailAddresses) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Missing email addresses"})
		return
	}

	// Get primary email
	primaryEmail := ""
	for _, emailAddr := range emailAddresses {
		if emailMap, ok := emailAddr.(map[string]interface{}); ok {
			if emailMap["email_address"] != nil {
				primaryEmail = emailMap["email_address"].(string)
				break
			}
		}
	}

	if primaryEmail == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "No valid email found"})
		return
	}

	// Extract name information
	firstName, _ := data["first_name"].(string)
	lastName, _ := data["last_name"].(string)
	
	// Combine first and last name
	fullName := strings.TrimSpace(firstName + " " + lastName)
	if fullName == "" {
		fullName = strings.Split(primaryEmail, "@")[0] // Use email username as fallback
	}

	// Update user in database
	userRepo := repository.NewUserRepository()
	user := &models.User{
		ID:    userID,
		Email: primaryEmail,
		Name:  fullName,
	}

	err := userRepo.UpdateUser(user)
	if err != nil {
		log.Printf("Error updating user in database: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update user"})
		return
	}

	log.Printf("User updated successfully: ID=%s, Email=%s, Name=%s", userID, primaryEmail, fullName)
	c.JSON(http.StatusOK, gin.H{"message": "User updated successfully"})
}

// handleUserDeleted removes user from the database
func handleUserDeleted(c *gin.Context, webhookData map[string]interface{}) {
	// Extract user data from webhook payload
	data, ok := webhookData["data"].(map[string]interface{})
	if !ok {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid webhook data structure"})
		return
	}

	// Extract user ID
	userID, ok := data["id"].(string)
	if !ok {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Missing user ID"})
		return
	}

	// Delete user from database
	userRepo := repository.NewUserRepository()
	err := userRepo.DeleteUser(userID)
	if err != nil {
		log.Printf("Error deleting user from database: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete user"})
		return
	}

	log.Printf("User deleted successfully: ID=%s", userID)
	c.JSON(http.StatusOK, gin.H{"message": "User deleted successfully"})
}