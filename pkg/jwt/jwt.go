package jwt

import (
	"os"
	"strconv"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

func GenerateToken(employeeID string, role string) (string, error) {
	secret := os.Getenv("JWT_SECRET")
	expHours, _ := strconv.Atoi(os.Getenv("JWT_EXPIRATION_HOURS"))
	if expHours == 0 {
		expHours = 24
	}

	claims := jwt.MapClaims{
		"employee_id": employeeID,
		"role":        role,
		"exp":         time.Now().Add(time.Hour * time.Duration(expHours)).Unix(),
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(secret))
}
