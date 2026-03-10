package mail

import (
	"bytes"
	"crypto/tls"
	"html/template"
	"os"
	"strconv"

	"gopkg.in/gomail.v2"
)

const otpTemplateHTML = `
<!DOCTYPE html>
<html>
<head><style>body { font-family: sans-serif; }</style></head>
<body>
    <h2>Kode OTP Login HRIS</h2>
    <p>Berikut adalah kode OTP Anda. Berlaku selama 5 menit:</p>
    <h1 style="letter-spacing: 5px; color: #2c3e50;">{{.OTPCode}}</h1>
    <p>Jangan bagikan kode ini ke siapapun.</p>
</body>
</html>
`

func SendOTP(toEmail string, otpCode string) error {
	port, _ := strconv.Atoi(os.Getenv("SMTP_PORT"))
	d := gomail.NewDialer(os.Getenv("SMTP_HOST"), port, os.Getenv("SMTP_USER"), os.Getenv("SMTP_PASS"))
	d.TLSConfig = &tls.Config{InsecureSkipVerify: true}

	// Parsing template HTML
	t, err := template.New("otp").Parse(otpTemplateHTML)
	if err != nil {
		return err
	}
	var body bytes.Buffer
	t.Execute(&body, struct{ OTPCode string }{OTPCode: otpCode})

	m := gomail.NewMessage()
	m.SetHeader("From", os.Getenv("SMTP_FROM_EMAIL"))
	m.SetHeader("To", toEmail)
	m.SetHeader("Subject", "Kode OTP Login HRIS Anda")
	m.SetBody("text/html", body.String())

	return d.DialAndSend(m)
}
