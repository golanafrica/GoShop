package utils

import (
	"errors"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

//go:generate mockgen -destination=../../mocks/utils/mock_jwt_validator.go -package=utils . JWTValidator

// JWTValidator interface pour la validation des tokens
type JWTValidator interface {
	ValidateToken(tokenString string) (jwt.MapClaims, error)
}

var jwtSecret []byte // remplir depuis config.Init()

func InitJWT(secret string) {
	jwtSecret = []byte(secret)
}

// ============================================================
// 🆕 v4.0.0 : GenerateAccessToken avec rôle
// ============================================================

// GenerateAccessToken génère un token d'accès avec user_id ET role
func GenerateAccessToken(userID, role string) (string, error) {
	claims := jwt.MapClaims{
		"sub":  userID,
		"role": role, // 🆕 v4.0.0 : rôle dans le JWT
		"iat":  time.Now().Unix(),
		"exp":  time.Now().Add(15 * time.Minute).Unix(),
		"type": "access",
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(jwtSecret)
}

// ============================================================
// 🆕 v4.0.0 : GenerateRefreshToken avec rôle
// ============================================================

// GenerateRefreshToken génère un refresh token avec user_id, jti ET role
func GenerateRefreshToken(userID, jti, role string) (string, error) {
	claims := jwt.MapClaims{
		"sub":  userID,
		"role": role, // 🆕 v4.0.0 : rôle dans le refresh token aussi
		"iat":  time.Now().Unix(),
		"exp":  time.Now().Add(7 * 24 * time.Hour).Unix(),
		"jti":  jti,
		"type": "refresh",
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(jwtSecret)
}

// ValidateToken valide un token et retourne les claims
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

// ============================================================
// 🆕 v4.0.0 : Extraire le rôle du JWT
// ============================================================

// ValidateTokenAndExtractRole valide un token et retourne (userID, role, error)
func ValidateTokenAndExtractRole(tokenString string) (string, string, error) {
	claims, err := ValidateToken(tokenString)
	if err != nil {
		return "", "", err
	}

	userID, ok := claims["sub"].(string)
	if !ok || userID == "" {
		return "", "", errors.New("invalid subject in token")
	}

	role, ok := claims["role"].(string)
	if !ok || role == "" {
		// Rétrocompatibilité : anciens tokens sans rôle → merchant
		role = "merchant"
	}

	return userID, role, nil
}

// ValidateTokenMap valide un token et retourne les claims sous forme de map
func ValidateTokenMap(tokenString string) (map[string]interface{}, error) {
	claims, err := ValidateToken(tokenString)
	if err != nil {
		return nil, err
	}
	return map[string]interface{}(claims), nil
}
