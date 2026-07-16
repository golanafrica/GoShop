package userdto

import (
	"errors"
	"regexp"
	"strings"

	"github.com/rs/zerolog"
)

var (
	hasUpper   = regexp.MustCompile(`[A-Z]`).MatchString
	hasNumber  = regexp.MustCompile(`[0-9]`).MatchString
	hasSpecial = regexp.MustCompile(`[^a-zA-Z0-9]`).MatchString
)

func isStrongPassword(password string) bool {
	if len(password) < 8 {
		return false
	}
	return hasUpper(password) && hasNumber(password) && hasSpecial(password)
}

type RegisterUserRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
	Name     string `json:"name,omitempty"`
	Role     string `json:"role,omitempty"`
}

type LoginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type UpdateProfileRequest struct {
	Name  *string `json:"name,omitempty"`
	Email *string `json:"email,omitempty"`
}

func (r *RegisterUserRequest) Validate() error {
	if !strings.Contains(r.Email, "@") {
		return errors.New("invalid email")
	}
	if !isStrongPassword(r.Password) {
		return errors.New("password must be at least 8 characters long and contain at least one uppercase letter, one number, and one special character")
	}
	return nil
}

func (r *RegisterUserRequest) ValidateWithLogging(logger zerolog.Logger) error {
	var validationErrors []string

	email := strings.TrimSpace(r.Email)
	if email == "" {
		validationErrors = append(validationErrors, "email is required")
	} else if !strings.Contains(email, "@") {
		validationErrors = append(validationErrors, "invalid email format")
	} else if len(email) > 255 {
		validationErrors = append(validationErrors, "email too long")
	}

	password := strings.TrimSpace(r.Password)
	if password == "" {
		validationErrors = append(validationErrors, "password is required")
	} else if !isStrongPassword(password) {
		validationErrors = append(validationErrors, "password is too weak")
	} else if len(password) > 100 {
		validationErrors = append(validationErrors, "password too long")
	}

	if r.Name != "" {
		name := strings.TrimSpace(r.Name)
		if len(name) < 2 || len(name) > 100 {
			validationErrors = append(validationErrors, "invalid name length")
		}
	}

	if len(validationErrors) > 0 {
		logger.Warn().
			Int("error_count", len(validationErrors)).
			Strs("validation_errors", validationErrors).
			Msg("Register request validation failed")
		return errors.New(validationErrors[0])
	}

	return nil
}

func (r *RegisterUserRequest) Normalize() {
	r.Email = strings.ToLower(strings.TrimSpace(r.Email))
	r.Password = strings.TrimSpace(r.Password)
	if r.Name != "" {
		r.Name = strings.TrimSpace(r.Name)
	}
	// ✅ Normalisation du rôle : on force "merchant" par défaut
	if r.Role == "" {
		r.Role = "merchant"
	} else {
		r.Role = strings.ToLower(strings.TrimSpace(r.Role))
	}
}

func maskEmail(email string) string {
	if email == "" {
		return ""
	}
	parts := strings.Split(email, "@")
	if len(parts) != 2 {
		return "***@***"
	}
	username := parts[0]
	if len(username) <= 2 {
		username = "***"
	} else {
		username = username[:2] + "***"
	}
	return username + "@" + parts[1]
}

func extractEmailDomain(email string) string {
	if email == "" {
		return ""
	}
	parts := strings.Split(email, "@")
	if len(parts) != 2 {
		return ""
	}
	return parts[1]
}
