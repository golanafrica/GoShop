package userdto

import (
	"errors"
	"strings"

	"github.com/rs/zerolog"
)

type RegisterUserRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
	Name     string `json:"name,omitempty"`
}

type LoginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type UpdateProfileRequest struct {
	Name  *string `json:"name,omitempty"`
	Email *string `json:"email,omitempty"`
}

// Validate valide la requête d'inscription avec messages basiques
func (r *RegisterUserRequest) Validate() error {
	if !strings.Contains(r.Email, "@") {
		return errors.New("invalid email")
	}
	if len(r.Password) < 6 {
		return errors.New("password must be at least 6 chars")
	}
	return nil
}

// ValidateWithLogging valide avec logging structuré
func (r *RegisterUserRequest) ValidateWithLogging(logger zerolog.Logger) error {
	var validationErrors []string

	// Validation de l'email
	email := strings.TrimSpace(r.Email)
	if email == "" {
		validationErrors = append(validationErrors, "email is required")
		logger.Debug().
			Str("field", "email").
			Msg("Validation failed: email is required")
	} else if !strings.Contains(email, "@") {
		validationErrors = append(validationErrors, "invalid email format")
		logger.Debug().
			Str("field", "email").
			Str("value", maskEmail(email)).
			Msg("Validation failed: invalid email format")
	} else if len(email) > 255 {
		validationErrors = append(validationErrors, "email too long")
		logger.Debug().
			Str("field", "email").
			Str("value", maskEmail(email)).
			Int("length", len(email)).
			Msg("Validation failed: email too long")
	}

	// Validation du mot de passe
	password := strings.TrimSpace(r.Password)
	if password == "" {
		validationErrors = append(validationErrors, "password is required")
		logger.Debug().
			Str("field", "password").
			Msg("Validation failed: password is required")
	} else if len(password) < 6 {
		validationErrors = append(validationErrors, "password must be at least 6 characters")
		logger.Debug().
			Str("field", "password").
			Int("length", len(password)).
			Msg("Validation failed: password too short")
	} else if len(password) > 100 {
		validationErrors = append(validationErrors, "password too long")
		logger.Debug().
			Str("field", "password").
			Int("length", len(password)).
			Msg("Validation failed: password too long")
	}

	// Validation du nom (optionnel)
	if r.Name != "" {
		name := strings.TrimSpace(r.Name)
		if len(name) < 2 {
			validationErrors = append(validationErrors, "name must be at least 2 characters")
			logger.Debug().
				Str("field", "name").
				Str("value", name).
				Int("length", len(name)).
				Msg("Validation failed: name too short")
		} else if len(name) > 100 {
			validationErrors = append(validationErrors, "name too long")
			logger.Debug().
				Str("field", "name").
				Str("value", name).
				Int("length", len(name)).
				Msg("Validation failed: name too long")
		}
	}

	if len(validationErrors) > 0 {
		logger.Warn().
			Int("error_count", len(validationErrors)).
			Strs("validation_errors", validationErrors).
			Interface("request_data", map[string]interface{}{
				"email":           maskEmail(r.Email),
				"has_password":    r.Password != "",
				"password_length": len(r.Password),
				"has_name":        r.Name != "",
				"name_length":     len(r.Name),
			}).
			Msg("Register request validation failed")

		return errors.New(validationErrors[0])
	}

	logger.Debug().
		Interface("validated_data", map[string]interface{}{
			"email_length":    len(r.Email),
			"password_length": len(r.Password),
			"has_name":        r.Name != "",
			"email_domain":    extractEmailDomain(r.Email),
		}).
		Msg("Register request validation passed")

	return nil
}

// Validate valide la requête de connexion
func (r *LoginRequest) Validate() error {
	if !strings.Contains(r.Email, "@") {
		return errors.New("invalid email")
	}
	if len(r.Password) < 1 {
		return errors.New("password is required")
	}
	return nil
}

// ValidateWithLogging valide avec logging structuré
func (r *LoginRequest) ValidateWithLogging(logger zerolog.Logger) error {
	var validationErrors []string

	// Validation de l'email
	email := strings.TrimSpace(r.Email)
	if email == "" {
		validationErrors = append(validationErrors, "email is required")
		logger.Debug().
			Str("field", "email").
			Msg("Validation failed: email is required")
	} else if !strings.Contains(email, "@") {
		validationErrors = append(validationErrors, "invalid email format")
		logger.Debug().
			Str("field", "email").
			Str("value", maskEmail(email)).
			Msg("Validation failed: invalid email format")
	}

	// Validation du mot de passe
	password := strings.TrimSpace(r.Password)
	if password == "" {
		validationErrors = append(validationErrors, "password is required")
		logger.Debug().
			Str("field", "password").
			Msg("Validation failed: password is required")
	}

	if len(validationErrors) > 0 {
		logger.Warn().
			Int("error_count", len(validationErrors)).
			Strs("validation_errors", validationErrors).
			Interface("request_data", map[string]interface{}{
				"email":           maskEmail(r.Email),
				"has_password":    r.Password != "",
				"password_length": len(r.Password),
			}).
			Msg("Login request validation failed")

		return errors.New(validationErrors[0])
	}

	logger.Debug().
		Str("email_domain", extractEmailDomain(r.Email)).
		Bool("has_password", r.Password != "").
		Msg("Login request validation passed")

	return nil
}

// Validate valide la requête de mise à jour de profil
func (r *UpdateProfileRequest) Validate() error {
	if r.Email != nil {
		email := strings.TrimSpace(*r.Email)
		if email == "" {
			return errors.New("email cannot be empty if provided")
		}
		if !strings.Contains(email, "@") {
			return errors.New("invalid email format")
		}
	}

	if r.Name != nil {
		name := strings.TrimSpace(*r.Name)
		if name == "" {
			return errors.New("name cannot be empty if provided")
		}
		if len(name) < 2 {
			return errors.New("name must be at least 2 characters")
		}
	}

	return nil
}

// ValidateWithLogging valide avec logging structuré
func (r *UpdateProfileRequest) ValidateWithLogging(logger zerolog.Logger) error {
	var validationErrors []string
	var updateFields []string

	if r.Email != nil {
		updateFields = append(updateFields, "email")
		email := strings.TrimSpace(*r.Email)
		if email == "" {
			validationErrors = append(validationErrors, "email cannot be empty if provided")
			logger.Debug().
				Str("field", "email").
				Msg("Update validation failed: email cannot be empty")
		} else if !strings.Contains(email, "@") {
			validationErrors = append(validationErrors, "invalid email format")
			logger.Debug().
				Str("field", "email").
				Str("value", maskEmail(email)).
				Msg("Update validation failed: invalid email format")
		}
	}

	if r.Name != nil {
		updateFields = append(updateFields, "name")
		name := strings.TrimSpace(*r.Name)
		if name == "" {
			validationErrors = append(validationErrors, "name cannot be empty if provided")
			logger.Debug().
				Str("field", "name").
				Msg("Update validation failed: name cannot be empty")
		} else if len(name) < 2 {
			validationErrors = append(validationErrors, "name must be at least 2 characters")
			logger.Debug().
				Str("field", "name").
				Str("value", name).
				Int("length", len(name)).
				Msg("Update validation failed: name too short")
		}
	}

	if len(validationErrors) > 0 {
		logger.Warn().
			Int("error_count", len(validationErrors)).
			Strs("validation_errors", validationErrors).
			Strs("update_fields", updateFields).
			Msg("Profile update validation failed")

		return errors.New(validationErrors[0])
	}

	if len(updateFields) > 0 {
		logger.Debug().
			Strs("update_fields", updateFields).
			Int("total_updates", len(updateFields)).
			Msg("Profile update validation passed")
	} else {
		logger.Warn().Msg("No fields provided for profile update")
		return errors.New("no fields provided for update")
	}

	return nil
}

// Normalize normalise les données
func (r *RegisterUserRequest) Normalize() {
	r.Email = strings.ToLower(strings.TrimSpace(r.Email))
	r.Password = strings.TrimSpace(r.Password)
	if r.Name != "" {
		r.Name = strings.TrimSpace(r.Name)
	}
}

func (r *LoginRequest) Normalize() {
	r.Email = strings.ToLower(strings.TrimSpace(r.Email))
	r.Password = strings.TrimSpace(r.Password)
}

func (r *UpdateProfileRequest) Normalize() {
	if r.Email != nil {
		email := strings.ToLower(strings.TrimSpace(*r.Email))
		r.Email = &email
	}
	if r.Name != nil {
		name := strings.TrimSpace(*r.Name)
		r.Name = &name
	}
}

// Helper functions
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

	domain := parts[1]
	return username + "@" + domain
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
