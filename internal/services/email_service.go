package services

import (
	"fmt"
	"log"
	"net/smtp"
	"os"
)

func SendPasswordResetEmail(email, token string) error {
	// Obtener configuración de email desde variables de entorno
	smtpHost := os.Getenv("SMTP_HOST")
	smtpPort := os.Getenv("SMTP_PORT")
	smtpUser := os.Getenv("SMTP_USER")
	smtpPass := os.Getenv("SMTP_PASS")
	fromEmail := os.Getenv("FROM_EMAIL")

	// Si no hay configuración de email, solo registramos el token y simulamos éxito
	if smtpHost == "" || smtpPort == "" || smtpUser == "" || smtpPass == "" {
		log.Printf("Configuración de email no encontrada. Token para %s: %s", email, token)
		return nil
	}

	// Configurar autenticación
	auth := smtp.PlainAuth("", smtpUser, smtpPass, smtpHost)

	// Construir mensaje
	to := []string{email}
	subject := "Restablecimiento de contraseña"
	body := fmt.Sprintf(`
	<html>
	<body>
		<h2>Restablecimiento de contraseña</h2>
		<p>Has solicitado restablecer tu contraseña. Utiliza el siguiente token:</p>
		<p><strong>%s</strong></p>
		<p>Si no has solicitado este cambio, puedes ignorar este correo.</p>
	</body>
	</html>
	`, token)

	message := fmt.Sprintf("To: %s\r\n"+
		"Subject: %s\r\n"+
		"MIME-Version: 1.0\r\n"+
		"Content-Type: text/html; charset=UTF-8\r\n"+
		"\r\n"+
		"%s\r\n", email, subject, body)

	// Enviar email
	err := smtp.SendMail(smtpHost+":"+smtpPort, auth, fromEmail, to, []byte(message))
	if err != nil {
		log.Printf("Error al enviar email: %v", err)
		return err
	}

	return nil
}
