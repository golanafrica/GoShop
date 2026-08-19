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

	"Goshop/application/metrics"
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
func (h *FileHandler) GeneratePresignedURL(w http.ResponseWriter, r *http.Request) error {
	ctx := r.Context()
	start := time.Now()
	logger := zerolog.Ctx(ctx)

	// 1. Extraire le UserID du contexte
	userID, ok := utils.UserIDFromContext(ctx)
	if !ok || userID == "" {
		logger.Warn().Msg("User ID not found in context")
		return utils.ErrUnauthorized
	}

	// 2. Parser la requête
	var req GeneratePresignedURLRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		// 📊 MÉTRIQUE : Erreur payload
		metrics.FilePresignedURLGenerated.WithLabelValues("validation_error").Inc()
		metrics.FileOperationDuration.WithLabelValues("presign").Observe(time.Since(start).Seconds())

		logger.Error().Err(err).Msg("❌ Erreur décodage JSON")
		return utils.ErrInvalidPayload
	}
	defer r.Body.Close()

	// 3. Valider le chemin
	if req.FilePath == "" {
		metrics.FilePresignedURLGenerated.WithLabelValues("validation_error").Inc()
		metrics.FileOperationDuration.WithLabelValues("presign").Observe(time.Since(start).Seconds())
		return utils.NewAppError("VALIDATION_ERROR", "file_path is required", http.StatusBadRequest)
	}

	// 4. Nettoyer le chemin (anti-path traversal)
	cleanPath := filepath.Clean(req.FilePath)
	if cleanPath == "." || cleanPath == "/" || filepath.IsAbs(cleanPath) {
		// 📊 MÉTRIQUE : Tentative path traversal
		metrics.FileSecurityViolations.WithLabelValues("path_traversal").Inc()
		metrics.FilePresignedURLGenerated.WithLabelValues("security_violation").Inc()
		metrics.FileOperationDuration.WithLabelValues("presign").Observe(time.Since(start).Seconds())

		logger.Warn().
			Str("file_path", req.FilePath).
			Str("user_id", userID).
			Msg("❌ Chemin invalide (tentative path traversal)")
		return utils.NewAppError("INVALID_PATH", "Invalid file path", http.StatusBadRequest)
	}

	// 5. Vérifier que le fichier existe
	if !h.fileStorage.Exists(cleanPath) {
		metrics.FilePresignedURLGenerated.WithLabelValues("not_found").Inc()
		metrics.FileOperationDuration.WithLabelValues("presign").Observe(time.Since(start).Seconds())

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

	duration := time.Since(start).Seconds()

	// 📊 MÉTRIQUES : Succès
	metrics.FilePresignedURLGenerated.WithLabelValues("success").Inc()
	metrics.FileOperationDuration.WithLabelValues("presign").Observe(duration)

	logger.Info().
		Str("file_path", cleanPath).
		Str("user_id", userID).
		Int64("expires_at", presigned.ExpiresAt.Unix()).
		Float64("duration_seconds", duration).
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
func (h *FileHandler) DownloadFile(w http.ResponseWriter, r *http.Request) error {
	ctx := r.Context()
	start := time.Now()
	logger := zerolog.Ctx(ctx)

	// 1. Extraire les paramètres query
	filePath := r.URL.Query().Get("path")
	signature := r.URL.Query().Get("sig")
	expStr := r.URL.Query().Get("exp")

	if filePath == "" || signature == "" || expStr == "" {
		metrics.FileDownloadTotal.WithLabelValues("validation_error", "unknown").Inc()
		metrics.FileOperationDuration.WithLabelValues("download").Observe(time.Since(start).Seconds())

		logger.Warn().Msg("Missing required query parameters")
		return utils.NewAppError("MISSING_PARAMS", "path, sig, and exp are required", http.StatusBadRequest)
	}

	// 2. Nettoyer le chemin (anti-path traversal)
	cleanPath := filepath.Clean(filePath)
	if cleanPath == "." || cleanPath == "/" || filepath.IsAbs(cleanPath) {
		// 📊 MÉTRIQUE : Tentative path traversal
		metrics.FileSecurityViolations.WithLabelValues("path_traversal").Inc()
		metrics.FileDownloadTotal.WithLabelValues("security_violation", "unknown").Inc()
		metrics.FileOperationDuration.WithLabelValues("download").Observe(time.Since(start).Seconds())

		logger.Warn().
			Str("file_path", filePath).
			Msg("❌ Chemin invalide (tentative path traversal)")
		return utils.NewAppError("INVALID_PATH", "Invalid file path", http.StatusBadRequest)
	}

	// 3. Parser le timestamp
	expiresAt, err := strconv.ParseInt(expStr, 10, 64)
	if err != nil {
		metrics.FileDownloadTotal.WithLabelValues("validation_error", "unknown").Inc()
		metrics.FileOperationDuration.WithLabelValues("download").Observe(time.Since(start).Seconds())

		logger.Error().Err(err).Msg("Invalid expiration timestamp")
		return utils.NewAppError("INVALID_EXP", "Invalid expiration timestamp", http.StatusBadRequest)
	}

	// 4. Vérifier l'expiration
	if time.Now().Unix() > expiresAt {
		// 📊 MÉTRIQUE : URL expirée
		metrics.FileExpiredURLs.Inc()
		metrics.FileDownloadTotal.WithLabelValues("expired", "unknown").Inc()
		metrics.FileOperationDuration.WithLabelValues("download").Observe(time.Since(start).Seconds())

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
		// 📊 MÉTRIQUE : Signature invalide
		metrics.FileInvalidSignatures.Inc()
		metrics.FileSecurityViolations.WithLabelValues("invalid_signature").Inc()
		metrics.FileDownloadTotal.WithLabelValues("invalid_signature", "unknown").Inc()
		metrics.FileOperationDuration.WithLabelValues("download").Observe(time.Since(start).Seconds())

		logger.Warn().
			Str("file_path", cleanPath).
			Msg("❌ Signature invalide")
		return utils.NewAppError("INVALID_SIGNATURE", "Invalid signature", http.StatusUnauthorized)
	}

	// 6. Vérifier que le fichier existe
	if !h.fileStorage.Exists(cleanPath) {
		metrics.FileDownloadTotal.WithLabelValues("not_found", "unknown").Inc()
		metrics.FileOperationDuration.WithLabelValues("download").Observe(time.Since(start).Seconds())

		logger.Error().
			Str("file_path", cleanPath).
			Msg("❌ Fichier introuvable")
		return utils.NewAppError("FILE_NOT_FOUND", "File not found", http.StatusNotFound)
	}

	// 7. Ouvrir le fichier
	fullPath := h.fileStorage.GetFullPath(cleanPath)
	file, err := os.Open(fullPath)
	if err != nil {
		metrics.FileDownloadTotal.WithLabelValues("error", "unknown").Inc()
		metrics.FileOperationDuration.WithLabelValues("download").Observe(time.Since(start).Seconds())
		metrics.ApplicationErrorsTotal.WithLabelValues("file_download", "file_handler").Inc()

		logger.Error().Err(err).Msg("❌ Erreur ouverture fichier")
		return utils.ErrInternalServer
	}
	defer file.Close()

	// 8. Obtenir les métadonnées
	stat, err := file.Stat()
	if err != nil {
		metrics.FileDownloadTotal.WithLabelValues("error", "unknown").Inc()
		metrics.FileOperationDuration.WithLabelValues("download").Observe(time.Since(start).Seconds())
		metrics.ApplicationErrorsTotal.WithLabelValues("file_download", "file_handler").Inc()

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
	duration := time.Since(start).Seconds()

	// 📊 MÉTRIQUES : Succès
	metrics.FileDownloadTotal.WithLabelValues("success", mimeType).Inc()
	metrics.FileOperationDuration.WithLabelValues("download").Observe(duration)
	metrics.FileDownloadSizeBytes.Observe(float64(stat.Size()))

	logger.Info().
		Str("file_path", cleanPath).
		Str("mime_type", mimeType).
		Int64("size", stat.Size()).
		Float64("duration_seconds", duration).
		Msg("✅ Fichier téléchargé avec succès")

	if _, err := io.Copy(w, file); err != nil {
		metrics.ApplicationErrorsTotal.WithLabelValues("file_stream", "file_handler").Inc()
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
