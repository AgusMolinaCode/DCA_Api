# Clerk Authentication Setup

This project has been integrated with Clerk for authentication. Here's how to set it up:

## Prerequisites

1. Create a Clerk account at https://clerk.com
2. Create a new application in your Clerk dashboard

## Configuration

1. In your Clerk dashboard, go to API Keys
2. Copy your publishable key and secret key
3. Update your `.env` file with the following values:

```env
CLERK_PUBLISHABLE_KEY=pk_test_your_publishable_key_here
CLERK_SECRET_KEY=sk_test_your_secret_key_here
```

## API Endpoints

### Authentication

- `GET /user` - Get current user information (requires authentication)
- `POST /clerk/webhook` - Webhook endpoint for Clerk events

### Protected Routes

All routes under the `/` group now require Clerk authentication:

- `/transactions/*` - Transaction management
- `/bolsas/*` - Bolsa management
- `/users/*` - User management
- `/dashboard` - Dashboard data
- `/performance` - Performance metrics
- `/holdings` - User holdings
- etc.

## Frontend Integration

In your frontend application, you'll need to:

1. Install the Clerk frontend SDK for your framework (React, Vue, etc.)
2. Configure it with your `CLERK_PUBLISHABLE_KEY`
3. Include the JWT token in the Authorization header for API requests:

```javascript
// Example for fetch requests
const response = await fetch('http://localhost:8080/user', {
  headers: {
    'Authorization': `Bearer ${await getToken()}`,
    'Content-Type': 'application/json',
  },
});
```

## Migration from Previous Auth System

The legacy authentication endpoints (`/signup`, `/login`, `/logout`) are still available but deprecated. They will be removed in a future version.

## Webhooks

To automatically create users in your database when they sign up through Clerk, configure a webhook in your Clerk dashboard:

### Configuration Steps:

1. Go to **Webhooks** in your Clerk dashboard
2. Click **Add Endpoint**
3. Set the endpoint URL: `https://your-domain.com/clerk/webhook`
4. Select the following events:
   - `user.created` - Automatically creates users in your database
   - `user.updated` - Updates user information when modified
   - `user.deleted` - Removes users from your database when deleted

### Webhook Payload Parameters:

When a user signs up through Clerk, the webhook will automatically extract and save these parameters to your database:

**For user.created event:**
```json
{
  "type": "user.created",
  "data": {
    "id": "user_abc123",
    "email_addresses": [
      {
        "email_address": "user@example.com"
      }
    ],
    "first_name": "John",
    "last_name": "Doe"
  }
}
```

**Database mapping:**
- `data.id` → `users.id` (Primary key)
- `data.email_addresses[0].email_address` → `users.email`
- `data.first_name + data.last_name` → `users.name`
- `password` → Empty string (not needed for Clerk users)
- `created_at` → Current timestamp

### Automatic User Creation:

The webhook handler will automatically:

1. **Extract user data** from the Clerk webhook payload
2. **Create a new user** in your PostgreSQL database
3. **Handle name fallbacks** (uses email username if no name provided)
4. **Log all operations** for debugging
5. **Handle errors gracefully** with proper HTTP responses

### Testing the Webhook:

You can test the webhook by:

1. Creating a new user through your Clerk-enabled frontend
2. Checking your database to confirm the user was created
3. Monitoring your server logs for webhook events

### Local Development:

For local testing, you can use ngrok to expose your local server:

```bash
ngrok http 8080
# Use the provided HTTPS URL in your Clerk webhook configuration
```

## Testing

You can test the authentication by:

1. Creating a user through your Clerk-enabled frontend
2. Making authenticated requests to the API endpoints
3. Checking the `/user` endpoint to verify user information

## Security Notes

- Never expose your `CLERK_SECRET_KEY` in client-side code
- Use HTTPS in production
- Consider implementing rate limiting on your API endpoints
- Regularly rotate your API keys