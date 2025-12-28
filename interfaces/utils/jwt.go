package utils

import (
	"errors"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

//go:generate mockgen -destination=../../mocks/utils/mock_jwt_validator.go -package=utils . JWTValidator

// AJOUTER CETTE INTERFACE (ligne 7-12)
type JWTValidator interface {
	ValidateToken(tokenString string) (jwt.MapClaims, error)
}

var jwtSecret []byte // remplir depuis config.Init()

func InitJWT(secret string) {
	jwtSecret = []byte(secret)
}

// Generate access token (short lived)
func GenerateAccessToken(userID string) (string, error) {
	claims := jwt.MapClaims{
		"sub":  userID,
		"iat":  time.Now().Unix(),
		"exp":  time.Now().Add(15 * time.Minute).Unix(),
		"type": "access",
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(jwtSecret)
}

// Generate refresh token (longer lived) with jti
func GenerateRefreshToken(userID, jti string) (string, error) {
	claims := jwt.MapClaims{
		"sub":  userID,
		"iat":  time.Now().Unix(),
		"exp":  time.Now().Add(7 * 24 * time.Hour).Unix(),
		"jti":  jti,
		"type": "refresh",
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(jwtSecret)
}

// Validate token and return claims
func ValidateToken(tokenString string) (jwt.MapClaims, error) {
	token, err := jwt.Parse(tokenString, func(t *jwt.Token) (interface{}, error) {
		if t.Method != jwt.SigningMethodHS256 {
			return nil, errors.New("unexpected signing method")
		}
		return jwtSecret, nil
	})
	if err != nil || !token.Valid {
		return nil, errors.New("invalid token")
	}
	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		return nil, errors.New("invalid claims")
	}
	return claims, nil
}

// ValidateJWT valide un token JWT et retourne le userID (claim `sub`)
func ValidateJWT(tokenString string) (string, error) {
	claims, err := ValidateToken(tokenString)
	if err != nil {
		return "", err
	}

	sub, ok := claims["sub"].(string)
	if !ok || sub == "" {
		return "", errors.New("invalid subject in token")
	}

	return sub, nil
}

// ValidateTokenMap valide un token et retourne les claims sous forme de map[string]interface{},
// utile pour les usecases (RefreshUsecase par exemple).
func ValidateTokenMap(tokenString string) (map[string]interface{}, error) {
	claims, err := ValidateToken(tokenString)
	if err != nil {
		return nil, err
	}
	return map[string]interface{}(claims), nil
}
