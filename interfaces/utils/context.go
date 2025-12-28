package utils

import "context"

// On crée deux clés privées non exportées
type contextKey string

const (
	userIDKey   contextKey = "user_id"
	userRoleKey contextKey = "user_role"
)

// GetUserID récupère l'ID utilisateur du contexte
func GetUserID(ctx context.Context) (string, bool) {
	value := ctx.Value(userIDKey)
	if value == nil {
		return "", false
	}

	// Type assertion pour vérifier que c'est bien un string
	userID, ok := value.(string)
	if !ok {
		return "", false
	}

	return userID, true
}

// SetUserID ajoute l'ID utilisateur au contexte
func SetUserID(ctx context.Context, userID string) context.Context {
	return context.WithValue(ctx, userIDKey, userID)
}

// ---------- SETTERS (pour injecter dans le context) ----------

// Injecte l'ID utilisateur
func WithUserID(ctx context.Context, userID string) context.Context {
	return context.WithValue(ctx, userIDKey, userID)
}

// Injecte le rôle utilisateur
func WithUserRole(ctx context.Context, role string) context.Context {
	return context.WithValue(ctx, userRoleKey, role)
}

// Injecte UserID + Role en même temps
func WithUser(ctx context.Context, userID, role string) context.Context {
	ctx = context.WithValue(ctx, userIDKey, userID)
	return context.WithValue(ctx, userRoleKey, role)
}

// ---------- GETTERS (pour récupérer depuis le context) ----------

// Récupère UserID
func UserIDFromContext(ctx context.Context) (string, bool) {
	id, ok := ctx.Value(userIDKey).(string)
	return id, ok
}

// Récupère Role
func UserRoleFromContext(ctx context.Context) (string, bool) {
	role, ok := ctx.Value(userRoleKey).(string)
	return role, ok
}
