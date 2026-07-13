package service

import "context"

// LoginRateLimiter définit l'interface pour le rate limiting du login
// Permet de prévenir les attaques par force brute
type LoginRateLimiter interface {
	// CheckEmailLimit vérifie si l'email a dépassé la limite de tentatives
	// Retourne une erreur si la limite est dépassée
	CheckEmailLimit(ctx context.Context, email string) error

	// CheckIPLimit vérifie si l'IP a dépassé la limite de tentatives
	// Retourne une erreur si la limite est dépassée
	CheckIPLimit(ctx context.Context, ip string) error

	// RecordFailedAttempt enregistre une tentative échouée
	// Incrémente les compteurs email et IP
	RecordFailedAttempt(ctx context.Context, email, ip string) error

	// ResetOnSuccess réinitialise les compteurs après une connexion réussie
	// Optionnel : peut être utilisé pour ne pas pénaliser les utilisateurs légitimes
	ResetOnSuccess(ctx context.Context, email, ip string) error
}
