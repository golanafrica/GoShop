package uploadusecase

import (
	"context"
	"fmt"
	"io"
	"path/filepath"
	"strings"

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
//   Sépare la logique métier de l'infrastructure.
//
// 🔐 Sécurité :
//   - Validation magic number (contenu réel)
//   - Validation extension
//   - Validation cohérence extension/contenu
//   - Limites de taille par type
//
// ============================================================

// UploadFileUsecase orchestre l'upload de fichiers
type UploadFileUsecase struct {
	storage *storageinfra.FileStorage
}

// NewUploadFileUsecase crée une nouvelle instance
func NewUploadFileUsecase(storage *storageinfra.FileStorage) *UploadFileUsecase {
	return &UploadFileUsecase{
		storage: storage,
	}
}

// UploadFileRequest représente la requête d'upload
type UploadFileRequest struct {
	File         io.ReadSeeker
	Filename     string
	DeclaredMIME string
	DocumentType string // identity_card, passport, business_registry, etc.
	MaxSizeBytes int64  // Taille max globale (optionnel, défaut 10MB)
}

// UploadFileResponse représente la réponse
type UploadFileResponse struct {
	FilePath string `json:"file_path"`
	FileName string `json:"file_name"`
	FileSize int64  `json:"file_size"`
	MimeType string `json:"mime_type"`
	FileType string `json:"file_type"`
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

	logger.Info().
		Str("file_path", filePath).
		Str("file_type", string(validation.FileType)).
		Int64("size", validation.Size).
		Msg("✅ Fichier uploadé avec succès")

	// 8. Retourner la réponse
	return &UploadFileResponse{
		FilePath: "/uploads/" + filePath,
		FileName: req.Filename,
		FileSize: validation.Size,
		MimeType: validation.MimeType,
		FileType: string(validation.FileType),
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
