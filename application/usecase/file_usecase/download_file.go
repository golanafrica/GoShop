package fileusecase

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"Goshop/domain/entity"
	"Goshop/infrastructure/storage"

	"github.com/rs/zerolog"
)

// DownloadFileUsecase gère le téléchargement sécurisé de fichiers
type DownloadFileUsecase struct {
	fileStorage *storage.FileStorage
	secretKey   string
}

// NewDownloadFileUsecase crée une nouvelle instance
func NewDownloadFileUsecase(fileStorage *storage.FileStorage, secretKey string) *DownloadFileUsecase {
	return &DownloadFileUsecase{
		fileStorage: fileStorage,
		secretKey:   secretKey,
	}
}

// DownloadFileRequest représente la requête de téléchargement
type DownloadFileRequest struct {
	FilePath  string
	Signature string
	ExpiresAt int64  // Unix timestamp
	UserID    string // User authentifié
}

// DownloadFileResponse représente la réponse
type DownloadFileResponse struct {
	File     io.ReadSeeker
	FileName string
	MimeType string
	Size     int64
}

// Execute télécharge un fichier avec validation de signature
func (uc *DownloadFileUsecase) Execute(
	ctx context.Context,
	req *DownloadFileRequest,
) (*DownloadFileResponse, error) {
	logger := zerolog.Ctx(ctx)

	logger.Info().
		Str("file_path", req.FilePath).
		Str("user_id", req.UserID).
		Msg("📥 Demande de téléchargement de fichier")

	// 1. Vérifier l'expiration
	expiresAt := time.Unix(req.ExpiresAt, 0)
	if time.Now().After(expiresAt) {
		logger.Warn().
			Str("file_path", req.FilePath).
			Msg("❌ URL expirée")
		return nil, fmt.Errorf("presigned URL has expired")
	}

	// 2. Vérifier la signature HMAC
	presigned := &entity.PresignedURL{
		FilePath:  req.FilePath,
		ExpiresAt: expiresAt,
	}

	if !presigned.VerifySignature(req.Signature, uc.secretKey) {
		logger.Warn().
			Str("file_path", req.FilePath).
			Str("user_id", req.UserID).
			Msg("❌ Signature invalide (tentative de falsification)")
		return nil, fmt.Errorf("invalid signature")
	}

	// 3. Vérifier que le fichier existe
	fullPath := uc.fileStorage.GetFullPath(req.FilePath)
	if !uc.fileStorage.Exists(req.FilePath) {
		logger.Error().
			Str("file_path", req.FilePath).
			Msg("❌ Fichier introuvable")
		return nil, fmt.Errorf("file not found")
	}

	// 4. Ouvrir le fichier
	file, err := os.Open(fullPath)
	if err != nil {
		logger.Error().Err(err).Msg("❌ Erreur ouverture fichier")
		return nil, fmt.Errorf("failed to open file: %w", err)
	}

	// 5. Obtenir les métadonnées
	stat, err := file.Stat()
	if err != nil {
		file.Close()
		logger.Error().Err(err).Msg("❌ Erreur stat fichier")
		return nil, fmt.Errorf("failed to stat file: %w", err)
	}

	// 6. Déterminer le MIME type
	ext := strings.ToLower(filepath.Ext(req.FilePath))
	mimeType := getMimeType(ext)

	logger.Info().
		Str("file_path", req.FilePath).
		Str("mime_type", mimeType).
		Int64("size", stat.Size()).
		Msg("✅ Fichier prêt à être téléchargé")

	return &DownloadFileResponse{
		File:     file,
		FileName: filepath.Base(req.FilePath),
		MimeType: mimeType,
		Size:     stat.Size(),
	}, nil
}

// getMimeType retourne le MIME type basé sur l'extension
func getMimeType(ext string) string {
	mimeTypes := map[string]string{
		".jpg":  "image/jpeg",
		".jpeg": "image/jpeg",
		".png":  "image/png",
		".pdf":  "application/pdf",
		".gif":  "image/gif",
		".webp": "image/webp",
	}

	if mime, ok := mimeTypes[ext]; ok {
		return mime
	}
	return "application/octet-stream"
}
