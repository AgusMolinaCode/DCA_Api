package middleware

import (
	"encoding/json"
	"io"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/AgusMolinaCode/DCA_Api.git/internal/models"
	"github.com/AgusMolinaCode/DCA_Api.git/internal/repository"
	"github.com/clerk/clerk-sdk-go/v2"
	"github.com/clerk/clerk-sdk-go/v2/jwt"
	"github.com/clerk/clerk-sdk-go/v2/user"
	"github.com/gin-gonic/gin"
	svix "github.com/svix/svix-webhooks/go"
)

var userClient *user.Client

// InitClerk initializes the Clerk client using the recommended pattern
func InitClerk() {
	secretKey := os.Getenv("CLERK_SECRET_KEY")
	if secretKey == "" {
		log.Printf("WARNING: CLERK_SECRET_KEY environment variable is not set. Clerk features will be disabled.")
		return
	}
	
	// Set global Clerk key (recommended approach)
	clerk.SetKey(secretKey)
	
	// Also initialize user client for API operations
	config := &clerk.ClientConfig{}
	config.Key = &secretKey
	userClient = user.NewClient(config)
	
	log.Printf("Clerk initialized successfully")
}

// SimpleAPIKeyMiddleware validates using user ID as API key
func SimpleAPIKeyMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Try API key from header first
		apiKey := c.GetHeader("X-API-Key")
		
		// If no API key header, try Authorization header with Bearer
		if apiKey == "" {
			authHeader := c.GetHeader("Authorization")
			if authHeader != "" {
				apiKey = strings.Replace(authHeader, "Bearer ", "", 1)
			}
		}

		if apiKey == "" {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "API Key requerido"})
			c.Abort()
			return
		}

		// Validate that the API key looks like a valid user ID (starts with "user_")
		if !strings.HasPrefix(apiKey, "user_") {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "API Key inválido"})
			c.Abort()
			return
		}

		// Check if user exists in database
		userRepo := repository.NewUserRepository()
		user, err := userRepo.GetUserById(apiKey)
		if err != nil {
			log.Printf("User not found for API key: %s, error: %v", apiKey, err)
			c.JSON(http.StatusUnauthorized, gin.H{"error": "API Key inválido"})
			c.Abort()
			return
		}

		// Store user ID in context
		c.Set("userId", user.ID)
		c.Set("userEmail", user.Email)
		c.Set("userName", user.Name)
		
		log.Printf("User authenticated via API key: %s (%s)", user.ID, user.Email)
		c.Next()
	}
}

// ClerkAuthMiddleware validates Clerk JWT tokens using the proper SDK approach
func ClerkAuthMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Check if Clerk is initialized
		if userClient == nil {
			c.JSON(http.StatusServiceUnavailable, gin.H{"error": "Clerk authentication not available"})
			c.Abort()
			return
		}

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
	// Check if Clerk is initialized
	if userClient == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "Clerk authentication not available"})
		return
	}

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

// ClerkWebhookHandler handles Clerk webhooks for user events using Svix
func ClerkWebhookHandler(c *gin.Context) {
	log.Printf("=== WEBHOOK RECEIVED ===")
	log.Printf("Headers: %+v", c.Request.Header)
	
	// Get the webhook signing secret from environment
	webhookSecret := os.Getenv("CLERK_WEBHOOK_SECRET")
	if webhookSecret == "" {
		log.Printf("ERROR: CLERK_WEBHOOK_SECRET environment variable is not set")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Webhook secret not configured"})
		return
	}
	log.Printf("Webhook secret found: %s", webhookSecret[:10]+"...")

	// Read the raw body
	body, err := io.ReadAll(c.Request.Body)
	if err != nil {
		log.Printf("ERROR: reading request body: %v", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": "Could not read request body"})
		return
	}
	log.Printf("Request body length: %d", len(body))
	log.Printf("Request body: %s", string(body))

	// Initialize Svix webhook with secret
	wh, err := svix.NewWebhook(webhookSecret)
	if err != nil {
		log.Printf("ERROR: creating Svix webhook: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to initialize webhook verification"})
		return
	}

	// Verify the webhook using Svix
	err = wh.Verify(body, c.Request.Header)
	if err != nil {
		log.Printf("ERROR: Svix webhook verification failed: %v", err)
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid webhook signature"})
		return
	}
	log.Printf("Webhook signature verified successfully with Svix")

	// Parse the webhook payload from the body we already read
	var webhookData map[string]interface{}
	if err := json.Unmarshal(body, &webhookData); err != nil {
		log.Printf("ERROR: parsing JSON payload: %v", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid JSON payload"})
		return
	}
	log.Printf("Webhook data parsed successfully: %+v", webhookData)

	// Extract the event type
	eventType, ok := webhookData["type"].(string)
	if !ok {
		log.Printf("ERROR: Missing event type in webhook payload")
		c.JSON(http.StatusBadRequest, gin.H{"error": "Missing event type"})
		return
	}

	log.Printf("Processing webhook event: %s", eventType)

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


// handleUserCreated creates a new user in the database when they sign up through Clerk
func handleUserCreated(c *gin.Context, webhookData map[string]interface{}) {
	log.Printf("=== HANDLING USER CREATED ===")
	log.Printf("Full webhook data: %+v", webhookData)
	
	// Extract user data from webhook payload
	data, ok := webhookData["data"].(map[string]interface{})
	if !ok {
		log.Printf("ERROR: Invalid webhook data structure")
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid webhook data structure"})
		return
	}
	log.Printf("User data extracted: %+v", data)

	// Extract user ID
	userID, ok := data["id"].(string)
	if !ok {
		log.Printf("ERROR: Missing user ID")
		c.JSON(http.StatusBadRequest, gin.H{"error": "Missing user ID"})
		return
	}
	log.Printf("User ID: %s", userID)

	// Extract email addresses
	emailAddresses, ok := data["email_addresses"].([]interface{})
	if !ok || len(emailAddresses) == 0 {
		log.Printf("ERROR: Missing email addresses")
		c.JSON(http.StatusBadRequest, gin.H{"error": "Missing email addresses"})
		return
	}
	log.Printf("Email addresses: %+v", emailAddresses)

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
	log.Printf("Primary email: %s", primaryEmail)

	if primaryEmail == "" {
		log.Printf("ERROR: No valid email found")
		c.JSON(http.StatusBadRequest, gin.H{"error": "No valid email found"})
		return
	}

	// Extract name information
	firstName, _ := data["first_name"].(string)
	lastName, _ := data["last_name"].(string)
	log.Printf("Name info - First: %s, Last: %s", firstName, lastName)
	
	// Combine first and last name
	fullName := strings.TrimSpace(firstName + " " + lastName)
	if fullName == "" {
		fullName = strings.Split(primaryEmail, "@")[0] // Use email username as fallback
	}
	log.Printf("Full name: %s", fullName)

	// Create user in database
	log.Printf("Creating user repository...")
	userRepo := repository.NewUserRepository()
	user := &models.User{
		ID:        userID,
		Email:     primaryEmail,
		Name:      fullName,
		Password:  "", // No password needed for Clerk users
		CreatedAt: time.Now(),
	}
	log.Printf("User object created: %+v", user)

	log.Printf("Attempting to save user to database...")
	err := userRepo.CreateUser(user)
	if err != nil {
		log.Printf("ERROR: creating user in database: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create user"})
		return
	}

	log.Printf("SUCCESS: User created successfully: ID=%s, Email=%s, Name=%s", userID, primaryEmail, fullName)
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