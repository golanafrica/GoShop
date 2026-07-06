package utils

import (
	"context"

	"github.com/google/uuid"
)

// On crée des clés privées non exportées
type contextKey string

const (
	userIDKey    contextKey = "user_id"
	userRoleKey  contextKey = "user_role"
	emailKey     contextKey = "user_email" // 🆕 v4.4.2
	sessionIDKey contextKey = "session_id" // 🆕 v4.4.2
)

// ============================================================
// 🆕 v4.0.0 : Générateur UUID (pour JTI)
// ============================================================

// GenerateUUID génère un nouvel UUID (utilisé pour JTI dans les refresh tokens)
func GenerateUUID() string {
	return uuid.NewString()
}

// ============================================================
// GETTERS
// ============================================================

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

// UserIDFromContext récupère UserID
func UserIDFromContext(ctx context.Context) (string, bool) {
	id, ok := ctx.Value(userIDKey).(string)
	return id, ok
}

// UserRoleFromContext récupère Role
func UserRoleFromContext(ctx context.Context) (string, bool) {
	role, ok := ctx.Value(userRoleKey).(string)
	return role, ok
}

// 🆕 v4.4.2 : UserEmailFromContext récupère l'email utilisateur
func UserEmailFromContext(ctx context.Context) (string, bool) {
	email, ok := ctx.Value(emailKey).(string)
	return email, ok
}

// 🆕 v4.4.2 : SessionIDFromContext récupère le session_id (jti JWT)
func SessionIDFromContext(ctx context.Context) (string, bool) {
	sessionID, ok := ctx.Value(sessionIDKey).(string)
	return sessionID, ok
}

// ============================================================
// SETTERS
// ============================================================

// SetUserID ajoute l'ID utilisateur au contexte
func SetUserID(ctx context.Context, userID string) context.Context {
	return context.WithValue(ctx, userIDKey, userID)
}

// WithUserID injecte l'ID utilisateur
func WithUserID(ctx context.Context, userID string) context.Context {
	return context.WithValue(ctx, userIDKey, userID)
}

// WithUserRole injecte le rôle utilisateur
func WithUserRole(ctx context.Context, role string) context.Context {
	return context.WithValue(ctx, userRoleKey, role)
}

// WithUser injecte UserID + Role en même temps
func WithUser(ctx context.Context, userID, role string) context.Context {
	ctx = context.WithValue(ctx, userIDKey, userID)
	return context.WithValue(ctx, userRoleKey, role)
}

// 🆕 v4.4.2 : WithUserEmail injecte l'email utilisateur
func WithUserEmail(ctx context.Context, email string) context.Context {
	return context.WithValue(ctx, emailKey, email)
}

// 🆕 v4.4.2 : WithSessionID injecte le session_id (jti JWT)
func WithSessionID(ctx context.Context, sessionID string) context.Context {
	return context.WithValue(ctx, sessionIDKey, sessionID)
}

// 🆕 v4.4.2 : WithFullUser injecte toutes les infos utilisateur
func WithFullUser(ctx context.Context, userID, role, email, sessionID string) context.Context {
	ctx = context.WithValue(ctx, userIDKey, userID)
	ctx = context.WithValue(ctx, userRoleKey, role)
	ctx = context.WithValue(ctx, emailKey, email)
	ctx = context.WithValue(ctx, sessionIDKey, sessionID)
	return ctx
}
