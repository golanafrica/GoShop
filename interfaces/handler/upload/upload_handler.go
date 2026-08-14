package uploadhandler

import (
	"net/http"

	uploadusecase "Goshop/application/usecase/upload_usecase"
	"Goshop/interfaces/middl"
	"Goshop/interfaces/utils"

	"github.com/go-chi/chi/v5"
	"github.com/rs/zerolog"
)

// UploadHandler gère les uploads de fichiers
type UploadHandler struct {
	uploadUC *uploadusecase.UploadFileUsecase
}

// NewUploadHandler crée une nouvelle instance
func NewUploadHandler(uploadUC *uploadusecase.UploadFileUsecase) *UploadHandler {
	return &UploadHandler{uploadUC: uploadUC}
}

// UploadResponse représente la réponse d'un upload réussi
type UploadResponse struct {
	Success  bool   `json:"success"`
	FilePath string `json:"file_path"`
	FileName string `json:"file_name"`
	FileSize int64  `json:"file_size"`
	MimeType string `json:"mime_type"`
	FileType string `json:"file_type"`
	Message  string `json:"message"`
}

// @Summary Upload un fichier KYC
// @Description Upload sécurisé d'un fichier KYC avec validation magic number
// @Tags Upload
// @Accept multipart/form-data
// @Produce json
// @Param file formData file true "Fichier à uploader (JPEG, PNG, PDF)"
// @Param type formData string true "Type de document (identity_card, passport, business_registry)"
// @Success 201 {object} UploadResponse
// @Failure 400 {object} utils.AppError "Fichier invalide ou trop volumineux"
// @Failure 401 {object} utils.AppError "Non autorisé"
// @Failure 500 {object} utils.AppError "Erreur interne"
// @Security ApiKeyAuth
// @Router /api/upload/kyc [post]
func (h *UploadHandler) UploadKYC(w http.ResponseWriter, r *http.Request) error {
	ctx := r.Context()
	logger := zerolog.Ctx(ctx)

	// 1. Parser le multipart form (max 10 MB)
	if err := r.ParseMultipartForm(10 << 20); err != nil {
		logger.Error().Err(err).Msg("Failed to parse multipart form")
		return utils.NewAppError("PARSE_MULTIPART_FAILED", "Invalid multipart form", http.StatusBadRequest)
	}

	// 2. Récupérer le fichier
	file, header, err := r.FormFile("file")
	if err != nil {
		logger.Error().Err(err).Msg("Failed to get file from form")
		return utils.NewAppError("FILE_MISSING", "File is required", http.StatusBadRequest)
	}
	defer file.Close()

	// 3. Récupérer le type de document
	docType := r.FormValue("type")
	if docType == "" {
		return utils.NewAppError("TYPE_MISSING", "Document type is required", http.StatusBadRequest)
	}

	// 4. Créer la requête usecase
	ucReq := &uploadusecase.UploadFileRequest{
		File:         file,
		Filename:     header.Filename,
		DeclaredMIME: header.Header.Get("Content-Type"),
		DocumentType: docType,
		MaxSizeBytes: 10 * 1024 * 1024, // 10 MB
	}

	// 5. Exécuter le usecase
	response, err := h.uploadUC.Execute(ctx, ucReq)
	if err != nil {
		logger.Error().Err(err).Msg("❌ Erreur upload usecase")
		return utils.NewAppError("UPLOAD_FAILED", err.Error(), http.StatusBadRequest)
	}

	logger.Info().
		Str("file_path", response.FilePath).
		Str("file_type", response.FileType).
		Int64("size", response.FileSize).
		Msg("✅ Upload réussi via usecase")

	// 6. Retourner la réponse HTTP
	httpResponse := UploadResponse{
		Success:  true,
		FilePath: response.FilePath,
		FileName: response.FileName,
		FileSize: response.FileSize,
		MimeType: response.MimeType,
		FileType: response.FileType,
		Message:  "File uploaded successfully",
	}

	utils.WriteJSON(w, http.StatusCreated, httpResponse)
	return nil
}

// RegisterRoutes enregistre les routes d'upload
func (h *UploadHandler) RegisterRoutes(r chi.Router) {
	r.Post("/kyc", middl.ErrorHandler(h.UploadKYC))
}
