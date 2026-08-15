package file

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"time"

	fileusecase "Goshop/application/usecase/file_usecase"
	"Goshop/domain/entity"
	storageinfra "Goshop/infrastructure/storage"
	"Goshop/interfaces/middl"
	"Goshop/interfaces/utils"

	"github.com/go-chi/chi/v5"
	"github.com/rs/zerolog"
)

// FileHandler gère les téléchargements de fichiers
type FileHandler struct {
	downloadUC  *fileusecase.DownloadFileUsecase
	fileStorage *storageinfra.FileStorage
	secretKey   string
}

// NewFileHandler crée une nouvelle instance
func NewFileHandler(
	downloadUC *fileusecase.DownloadFileUsecase,
	fileStorage *storageinfra.FileStorage,
	secretKey string,
) *FileHandler {
	return &FileHandler{
		downloadUC:  downloadUC,
		fileStorage: fileStorage,
		secretKey:   secretKey,
	}
}

// GeneratePresignedURLRequest représente la requête de génération d'URL
type GeneratePresignedURLRequest struct {
	FilePath string `json:"file_path"`
}

// GeneratePresignedURLResponse représente la réponse
type GeneratePresignedURLResponse struct {
	URL       string `json:"url"`
	ExpiresAt int64  `json:"expires_at"`
}

// @Summary Générer une URL pré-signée pour télécharger un fichier
// @Description Génère une URL temporaire (15 min) pour télécharger un fichier KYC sécurisé
// @Tags Files
// @Accept json
// @Produce json
// @Param request body GeneratePresignedURLRequest true "Chemin du fichier"
// @Success 200 {object} GeneratePresignedURLResponse
// @Failure 400 {object} utils.AppError "Payload invalide"
// @Failure 401 {object} utils.AppError "Non autorisé"
// @Failure 403 {object} utils.AppError "Accès refusé au fichier"
// @Security ApiKeyAuth
// @Router /api/files/presign [post]
func (h *FileHandler) GeneratePresignedURL(w http.ResponseWriter, r *http.Request) error {
	ctx := r.Context()
	logger := zerolog.Ctx(ctx)

	// 1. Extraire le UserID du contexte
	userID, ok := utils.UserIDFromContext(ctx)
	if !ok || userID == "" {
		logger.Warn().Msg("User ID not found in context")
		return utils.ErrUnauthorized
	}

	// 2. Parser la requête (pattern standard GoShop)
	var req GeneratePresignedURLRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		logger.Error().Err(err).Msg("❌ Erreur décodage JSON")
		return utils.ErrInvalidPayload
	}
	defer r.Body.Close()

	// 3. Valider le chemin
	if req.FilePath == "" {
		return utils.NewAppError("VALIDATION_ERROR", "file_path is required", http.StatusBadRequest)
	}

	// 4. Nettoyer le chemin (anti-path traversal)
	cleanPath := filepath.Clean(req.FilePath)
	if cleanPath == "." || cleanPath == "/" || filepath.IsAbs(cleanPath) {
		logger.Warn().
			Str("file_path", req.FilePath).
			Str("user_id", userID).
			Msg("❌ Chemin invalide (tentative path traversal)")
		return utils.NewAppError("INVALID_PATH", "Invalid file path", http.StatusBadRequest)
	}

	// 5. Vérifier que le fichier existe
	if !h.fileStorage.Exists(cleanPath) {
		logger.Warn().
			Str("file_path", cleanPath).
			Str("user_id", userID).
			Msg("❌ Fichier introuvable")
		return utils.NewAppError("FILE_NOT_FOUND", "File not found", http.StatusNotFound)
	}

	// 6. Générer l'URL pré-signée (15 min)
	ttl := 15 * time.Minute
	presigned := entity.NewPresignedURL("", userID, cleanPath, ttl)
	signature := presigned.GenerateSignature(h.secretKey)

	// 7. Construire l'URL
	baseURL := os.Getenv("API_BASE_URL")
	if baseURL == "" {
		baseURL = "http://localhost:8080"
	}

	signedURL := fmt.Sprintf(
		"%s/api/files/download?path=%s&sig=%s&exp=%d",
		baseURL,
		cleanPath,
		signature,
		presigned.ExpiresAt.Unix(),
	)

	logger.Info().
		Str("file_path", cleanPath).
		Str("user_id", userID).
		Int64("expires_at", presigned.ExpiresAt.Unix()).
		Msg("✅ URL pré-signée générée")

	// 8. Retourner la réponse
	response := GeneratePresignedURLResponse{
		URL:       signedURL,
		ExpiresAt: presigned.ExpiresAt.Unix(),
	}

	utils.WriteJSON(w, http.StatusOK, response)
	return nil
}

// @Summary Télécharger un fichier avec URL pré-signée
// @Description Télécharge un fichier en vérifiant la signature HMAC et l'expiration
// @Tags Files
// @Produce application/octet-stream
// @Param path query string true "Chemin du fichier"
// @Param sig query string true "Signature HMAC-SHA256"
// @Param exp query int true "Timestamp d'expiration (Unix)"
// @Success 200 {file} binary "Contenu du fichier"
// @Failure 400 {object} utils.AppError "Paramètres manquants"
// @Failure 401 {object} utils.AppError "Signature invalide ou expirée"
// @Failure 404 {object} utils.AppError "Fichier introuvable"
// @Security ApiKeyAuth
// @Router /api/files/download [get]
func (h *FileHandler) DownloadFile(w http.ResponseWriter, r *http.Request) error {
	ctx := r.Context()
	logger := zerolog.Ctx(ctx)

	// 1. Extraire les paramètres query
	filePath := r.URL.Query().Get("path")
	signature := r.URL.Query().Get("sig")
	expStr := r.URL.Query().Get("exp")

	if filePath == "" || signature == "" || expStr == "" {
		logger.Warn().Msg("Missing required query parameters")
		return utils.NewAppError("MISSING_PARAMS", "path, sig, and exp are required", http.StatusBadRequest)
	}

	// 2. Nettoyer le chemin (anti-path traversal)
	cleanPath := filepath.Clean(filePath)
	if cleanPath == "." || cleanPath == "/" || filepath.IsAbs(cleanPath) {
		logger.Warn().
			Str("file_path", filePath).
			Msg("❌ Chemin invalide (tentative path traversal)")
		return utils.NewAppError("INVALID_PATH", "Invalid file path", http.StatusBadRequest)
	}

	// 3. Parser le timestamp
	expiresAt, err := strconv.ParseInt(expStr, 10, 64)
	if err != nil {
		logger.Error().Err(err).Msg("Invalid expiration timestamp")
		return utils.NewAppError("INVALID_EXP", "Invalid expiration timestamp", http.StatusBadRequest)
	}

	// 4. Vérifier l'expiration
	if time.Now().Unix() > expiresAt {
		logger.Warn().
			Str("file_path", cleanPath).
			Int64("expires_at", expiresAt).
			Msg("❌ URL expirée")
		return utils.NewAppError("URL_EXPIRED", "Presigned URL has expired", http.StatusUnauthorized)
	}

	// 5. Vérifier la signature HMAC
	data := fmt.Sprintf("%s:%d", cleanPath, expiresAt)
	mac := hmac.New(sha256.New, []byte(h.secretKey))
	mac.Write([]byte(data))
	expectedSignature := hex.EncodeToString(mac.Sum(nil))

	if !hmac.Equal([]byte(expectedSignature), []byte(signature)) {
		logger.Warn().
			Str("file_path", cleanPath).
			Msg("❌ Signature invalide")
		return utils.NewAppError("INVALID_SIGNATURE", "Invalid signature", http.StatusUnauthorized)
	}

	// 6. Vérifier que le fichier existe
	if !h.fileStorage.Exists(cleanPath) {
		logger.Error().
			Str("file_path", cleanPath).
			Msg("❌ Fichier introuvable")
		return utils.NewAppError("FILE_NOT_FOUND", "File not found", http.StatusNotFound)
	}

	// 7. Ouvrir le fichier
	fullPath := h.fileStorage.GetFullPath(cleanPath)
	file, err := os.Open(fullPath)
	if err != nil {
		logger.Error().Err(err).Msg("❌ Erreur ouverture fichier")
		return utils.ErrInternalServer
	}
	defer file.Close()

	// 8. Obtenir les métadonnées
	stat, err := file.Stat()
	if err != nil {
		logger.Error().Err(err).Msg("❌ Erreur stat fichier")
		return utils.ErrInternalServer
	}

	// 9. Déterminer le MIME type
	ext := filepath.Ext(cleanPath)
	mimeType := getMimeType(ext)

	// 10. Configurer les headers
	w.Header().Set("Content-Type", mimeType)
	w.Header().Set("Content-Length", strconv.FormatInt(stat.Size(), 10))
	w.Header().Set("Content-Disposition", fmt.Sprintf("inline; filename=%q", filepath.Base(cleanPath)))
	w.Header().Set("Cache-Control", "private, no-cache, no-store, must-revalidate")
	w.Header().Set("X-Content-Type-Options", "nosniff")

	// 11. Streamer le fichier
	logger.Info().
		Str("file_path", cleanPath).
		Str("mime_type", mimeType).
		Int64("size", stat.Size()).
		Msg("✅ Fichier téléchargé avec succès")

	if _, err := io.Copy(w, file); err != nil {
		logger.Error().Err(err).Msg("❌ Erreur streaming fichier")
		return utils.ErrInternalServer
	}

	return nil
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

// RegisterRoutes enregistre les routes de fichiers
func (h *FileHandler) RegisterRoutes(r chi.Router) {
	r.Post("/presign", middl.ErrorHandler(h.GeneratePresignedURL))
	r.Get("/download", middl.ErrorHandler(h.DownloadFile))
}
