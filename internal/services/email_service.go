package services

import (
	"bytes"
	"html/template"
	"net/smtp"
	"os"
)

const resetPasswordTemplate = `
<!DOCTYPE html>
<html>
<head>
    <style>
        .container {
            padding: 20px;
            font-family: Arial, sans-serif;
        }
        .button {
            background-color: #4CAF50;
            border: none;
            color: white;
            padding: 15px 32px;
            text-align: center;
            text-decoration: none;
            display: inline-block;
            font-size: 16px;
            margin: 4px 2px;
            cursor: pointer;
            border-radius: 4px;
        }
    </style>
</head>
<body>
    <div class="container">
        <h2>Recuperación de Contraseña</h2>
        <p>Hola,</p>
        <p>Has solicitado restablecer tu contraseña. Haz clic en el siguiente enlace para continuar:</p>
        <p>
            <a href="{{.ResetURL}}" class="button">Restablecer Contraseña</a>
        </p>
        <p>O copia y pega el siguiente token:</p>
        <p><strong>{{.Token}}</strong></p>
        <p>Este enlace expirará en 24 horas.</p>
        <p>Si no solicitaste restablecer tu contraseña, puedes ignorar este mensaje.</p>
        <br>
        <p>Saludos,<br>El equipo de soporte</p>
    </div>
</body>
</html>
`

type EmailData struct {
	ResetURL string
	Token    string
}

func SendPasswordResetEmail(to, token string) error {
	from := os.Getenv("EMAIL_FROM")
	pass := os.Getenv("EMAIL_PASSWORD")

	data := EmailData{
		ResetURL: "http://localhost:8080/reset-password?token=" + token,
		Token:    token,
	}

	// Crear template
	t, err := template.New("resetPassword").Parse(resetPasswordTemplate)
	if err != nil {
		return err
	}

	// Ejecutar template
	var body bytes.Buffer
	if err := t.Execute(&body, data); err != nil {
		return err
	}

	// Crear mensaje
	mime := "MIME-version: 1.0;\nContent-Type: text/html; charset=\"UTF-8\";\n\n"
	subject := "Subject: Recuperación de Contraseña\n"
	msg := []byte(subject + mime + body.String())

	// Enviar email
	auth := smtp.PlainAuth("", from, pass, "smtp.gmail.com")
	err = smtp.SendMail("smtp.gmail.com:587", auth, from, []string{to}, msg)

	return err
}
