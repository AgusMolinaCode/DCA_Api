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

	// Validar que todas las variables de entorno estén presentes
	if smtpHost == "" || smtpPort == "" || smtpUser == "" || smtpPass == "" || fromEmail == "" {
		log.Printf("Configuración de email incompleta. No se puede enviar correo a %s", email)
		return fmt.Errorf("configuración de email incompleta")
	}

	// Configurar autenticación
	auth := smtp.PlainAuth("", smtpUser, smtpPass, smtpHost)

	// Construir mensaje
	to := []string{email}
	subject := "Restablecimiento de contraseña"
	resetLink := fmt.Sprintf("http://localhost:3000/reset-password?token=%s", token)
	body := fmt.Sprintf(`
	<!DOCTYPE html>
	<html lang="es">
	<head>
		<meta charset="UTF-8">
		<meta name="viewport" content="width=device-width, initial-scale=1.0">
		<title>Restablecimiento de Contraseña</title>
		<style>
			body {
				font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, Oxygen, Ubuntu, Cantarell, 'Open Sans', 'Helvetica Neue', sans-serif;
				line-height: 1.6;
				color: #333;
				max-width: 600px;
				margin: 0 auto;
				padding: 20px;
				background-color: #f4f4f4;
			}
			.container {
				background-color: white;
				border-radius: 10px;
				box-shadow: 0 4px 6px rgba(0,0,0,0.1);
				padding: 30px;
				text-align: center;
			}
			.header {
				background-color: #007bff;
				color: white;
				padding: 15px;
				border-radius: 10px 10px 0 0;
				margin: -30px -30px 20px;
			}
			.btn {
				display: inline-block;
				background-color: #28a745;
				color: white;
				padding: 12px 24px;
				text-decoration: none;
				border-radius: 5px;
				margin: 20px 0;
				font-weight: bold;
			}
			.footer {
				margin-top: 20px;
				font-size: 0.8em;
				color: #666;
			}
		</style>
	</head>
	<body>
		<div class="container">
			<div class="header">
				<h1>Restablecimiento de Contraseña</h1>
			</div>
			<p>Hola,</p>
			<p>Hemos recibido una solicitud para restablecer la contraseña de tu cuenta. Haz clic en el botón de abajo para continuar:</p>
			
			<a href="%s" class="btn">Restablecer Contraseña</a>
			
			<p>Si no solicitaste este cambio, puedes ignorar este correo. Tu contraseña permanecerá sin cambios.</p>
			
			<p>El enlace es válido por las próximas 24 horas.</p>
			
			<div class="footer">
				<p>Si tienes problemas, copia y pega el siguiente enlace en tu navegador:</p>
				<p>%s</p>
				<p>© 2024 Tu Aplicación. Todos los derechos reservados.</p>
			</div>
		</div>
	</body>
	</html>
	`, resetLink, resetLink)

	message := fmt.Sprintf("To: %s\r\n"+
		"Subject: %s\r\n"+
		"MIME-Version: 1.0\r\n"+
		"Content-Type: text/html; charset=UTF-8\r\n"+
		"\r\n"+
		"%s\r\n", email, subject, body)

	// Enviar email
	err := smtp.SendMail(smtpHost+":"+smtpPort, auth, fromEmail, to, []byte(message))
	if err != nil {
		log.Printf("Error al enviar email de restablecimiento a %s: %v", email, err)
		return fmt.Errorf("error al enviar email de restablecimiento: %v", err)
	}

	log.Printf("Email de restablecimiento de contraseña enviado a %s", email)
	return nil
}
