package uploadusecase

import (
	"context"
	"fmt"
	"io"
	"path/filepath"
	"strings"

	"Goshop/domain/entity"
	"Goshop/domain/repository"
	storageinfra "Goshop/infrastructure/storage"
	"Goshop/interfaces/utils"

	"github.com/rs/zerolog"
)

// ============================================================
// UPLOAD FILE USECASE
// ============================================================
//
// 🎯 Objectif :
//   Orchestrer l'upload sécurisé d'un fichier avec validation.
//   Générer un token pour lier l'upload à la soumission KYC.
//
// 🔐 Sécurité :
//   - Validation magic number (contenu réel)
//   - Validation extension
//   - Validation cohérence extension/contenu
//   - Limites de taille par type
//   - Token cryptographique pour scope serveur
//
// ============================================================

// UploadFileUsecase orchestre l'upload de fichiers
type UploadFileUsecase struct {
	storage   *storageinfra.FileStorage
	tokenRepo repository.UploadTokenRepository
}

// NewUploadFileUsecase crée une nouvelle instance
func NewUploadFileUsecase(
	storage *storageinfra.FileStorage,
	tokenRepo repository.UploadTokenRepository,
) *UploadFileUsecase {
	return &UploadFileUsecase{
		storage:   storage,
		tokenRepo: tokenRepo,
	}
}

// UploadFileRequest représente la requête d'upload
type UploadFileRequest struct {
	File         io.ReadSeeker
	Filename     string
	DeclaredMIME string
	DocumentType string // identity_card, passport, business_registry, etc.
	MaxSizeBytes int64  // Taille max globale (optionnel, défaut 10MB)
	UserID       string // User ID pour lier le token (anti-IDOR)
}

// UploadFileResponse représente la réponse
type UploadFileResponse struct {
	FilePath string `json:"file_path"`
	FileName string `json:"file_name"`
	FileSize int64  `json:"file_size"`
	MimeType string `json:"mime_type"`
	FileType string `json:"file_type"`
	Token    string `json:"token"` // Token pour valider la soumission KYC
}

// Execute uploade un fichier avec validation complète
func (uc *UploadFileUsecase) Execute(
	ctx context.Context,
	req *UploadFileRequest,
) (*UploadFileResponse, error) {
	logger := zerolog.Ctx(ctx)

	logger.Info().
		Str("filename", req.Filename).
		Str("document_type", req.DocumentType).
		Str("user_id", req.UserID).
		Msg("📤 Début upload fichier")

	// 1. Déterminer les types autorisés selon le document
	allowedTypes, err := uc.getAllowedTypesForDocument(req.DocumentType)
	if err != nil {
		return nil, err
	}

	// 2. Définir taille max (défaut 10 MB)
	maxSize := req.MaxSizeBytes
	if maxSize <= 0 {
		maxSize = 10 * 1024 * 1024 // 10 MB
	}

	// 3. Valider l'extension
	if err := utils.ValidateFileExtension(req.Filename, allowedTypes); err != nil {
		logger.Warn().
			Str("filename", req.Filename).
			Err(err).
			Msg("❌ Extension invalide")
		return nil, fmt.Errorf("invalid extension: %w", err)
	}

	// 4. Valider le contenu (magic number)
	validation, err := utils.ValidateFileContent(req.File, maxSize, allowedTypes)
	if err != nil {
		logger.Error().Err(err).Msg("❌ Erreur validation contenu")
		return nil, fmt.Errorf("validation error: %w", err)
	}

	if !validation.Valid {
		logger.Warn().
			Str("filename", req.Filename).
			Int64("size", validation.Size).
			Str("error", validation.ErrorMessage).
			Msg("❌ Validation échouée")
		return nil, fmt.Errorf("file validation failed: %s", validation.ErrorMessage)
	}

	// 5. Valider cohérence extension/contenu
	if err := utils.ValidateFileMetadata(req.Filename, req.DeclaredMIME, validation.FileType); err != nil {
		logger.Warn().
			Str("filename", req.Filename).
			Err(err).
			Msg("❌ Incohérence métadonnées")
		return nil, fmt.Errorf("metadata mismatch: %w", err)
	}

	// 6. Extraire l'extension
	ext := strings.ToLower(filepath.Ext(req.Filename))

	// 7. Sauvegarder le fichier
	filePath, err := uc.storage.Save(req.File, req.Filename, ext)
	if err != nil {
		logger.Error().Err(err).Msg("❌ Erreur sauvegarde fichier")
		return nil, fmt.Errorf("save error: %w", err)
	}

	// 8. Créer le token d'upload (DDD : entité métier)
	fullPath := "/uploads/" + filePath
	token, err := entity.NewUploadToken(
		req.UserID,
		fullPath,
		req.Filename,
		validation.Size,
		validation.MimeType,
	)
	if err != nil {
		logger.Error().Err(err).Msg("❌ Erreur création token")
		return nil, fmt.Errorf("failed to create upload token: %w", err)
	}

	// 9. Stocker le token dans Redis (via repository)
	if err := uc.tokenRepo.Create(ctx, token); err != nil {
		logger.Error().Err(err).Msg("❌ Erreur stockage token")
		// Ne pas bloquer l'upload, mais logger l'erreur
		// Le fichier est sauvegardé mais ne pourra pas être utilisé
	}

	logger.Info().
		Str("file_path", filePath).
		Str("token", token.ID).
		Str("file_type", string(validation.FileType)).
		Int64("size", validation.Size).
		Msg("✅ Fichier uploadé avec token")

	// 10. Retourner la réponse avec token
	return &UploadFileResponse{
		FilePath: fullPath,
		FileName: req.Filename,
		FileSize: validation.Size,
		MimeType: validation.MimeType,
		FileType: string(validation.FileType),
		Token:    token.ID,
	}, nil
}

// getAllowedTypesForDocument retourne les types autorisés selon le document
func (uc *UploadFileUsecase) getAllowedTypesForDocument(docType string) ([]utils.FileType, error) {
	switch docType {
	case "identity_card", "passport", "cni":
		// Identité = images uniquement (pas PDF pour éviter faux documents)
		return []utils.FileType{utils.FileTypeJPEG, utils.FileTypePNG}, nil

	case "business_registry", "other", "proof":
		// Documents officiels = images + PDF
		return []utils.FileType{utils.FileTypeJPEG, utils.FileTypePNG, utils.FileTypePDF}, nil

	default:
		return nil, fmt.Errorf("invalid document type: %s", docType)
	}
}
