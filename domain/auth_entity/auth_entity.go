package authentity

import "time"

// RefreshSession représente une session de refresh token stockée en base.
//
// Chaque refresh token possède un JTI (ID unique). Le JTI est stocké ici,
// ce qui permet :
//   - de vérifier si le refresh token existe
//   - de vérifier s’il a été révoqué
//   - d’implémenter la rotation des refresh tokens
//   - d’empêcher la réutilisation d’un token révoqué (reuse detection)
type RefreshSession struct {
	ID        string    // JTI (unique ID du refresh token)
	UserID    string    // ID de l'utilisateur propriétaire du token
	ExpiresAt time.Time // Date d’expiration
	Revoked   bool      // Si le token est invalidé
	CreatedAt time.Time // Date de création
}
