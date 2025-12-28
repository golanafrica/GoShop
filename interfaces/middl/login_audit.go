// interfaces/middl/login_audit.go
package middl

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"strings"

	"github.com/rs/zerolog"
)

// LoginAuditMiddleware logue les tentatives de connexion avec request_id
func LoginAuditMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/login" && r.Method == "POST" {
			bodyBytes, err := io.ReadAll(r.Body)
			if err == nil && len(bodyBytes) > 0 {
				r.Body = io.NopCloser(bytes.NewBuffer(bodyBytes))

				var loginData struct {
					Email string `json:"email"`
				}
				if json.Unmarshal(bodyBytes, &loginData) == nil && loginData.Email != "" {
					logger := zerolog.Ctx(r.Context())
					logger.Info().
						Str("user_email", maskEmail(loginData.Email)).
						Int("body_size", len(bodyBytes)).
						Msg("🔐 Tentative de connexion")
				}
			}
		}
		next.ServeHTTP(w, r)
	})
}

func maskEmail(email string) string {
	if len(email) > 3 && strings.Contains(email, "@") {
		return email[:3] + "***@" + email[strings.Index(email, "@")+1:]
	}
	return "***@***"
}
