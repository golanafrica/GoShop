package repository

import (
	"context"

	"Goshop/domain/entity"
)

//go:generate mockgen -destination=../../mocks/repository/mock_upload_token_repository.go -package=repository . UploadTokenRepository

// UploadTokenRepository définit le contrat pour la gestion des tokens d'upload
type UploadTokenRepository interface {
	// Create crée un nouveau token
	Create(ctx context.Context, token *entity.UploadToken) error

	// FindByID récupère un token par son ID
	FindByID(ctx context.Context, tokenID string) (*entity.UploadToken, error)

	// MarkUsed marque un token comme utilisé
	MarkUsed(ctx context.Context, tokenID string) error

	// DeleteExpired supprime les tokens expirés
	DeleteExpired(ctx context.Context) (int64, error)

	// DeleteByUser supprime tous les tokens d'un utilisateur (logout)
	DeleteByUser(ctx context.Context, userID string) error
}
