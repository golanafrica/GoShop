package customerhandler

import (
	"encoding/json"
	"net/http"

	customerusecase "Goshop/application/usecase/customer_usecase"
	"Goshop/interfaces/middl"
	"Goshop/interfaces/utils"

	"github.com/go-chi/chi/v5"
	"github.com/rs/zerolog"
)

// KYCHandler gère les routes KYC
type KYCHandler struct {
	uploadKYCUC      *customerusecase.UploadKYCDocumentUsecase
	getKYCStatusUC   *customerusecase.GetKYCStatusUsecase
	reviewKYCUC      *customerusecase.ReviewKYCUsecase
	listPendingKYCUC *customerusecase.ListPendingKYCUsecase
}

// NewKYCHandler crée une nouvelle instance
func NewKYCHandler(
	uploadKYCUC *customerusecase.UploadKYCDocumentUsecase,
	getKYCStatusUC *customerusecase.GetKYCStatusUsecase,
	reviewKYCUC *customerusecase.ReviewKYCUsecase,
	listPendingKYCUC *customerusecase.ListPendingKYCUsecase,
) *KYCHandler {
	return &KYCHandler{
		uploadKYCUC:      uploadKYCUC,
		getKYCStatusUC:   getKYCStatusUC,
		reviewKYCUC:      reviewKYCUC,
		listPendingKYCUC: listPendingKYCUC,
	}
}

// @Summary Soumettre un document KYC (Client)
// @Description Permet à un client de soumettre un document d'identité pour vérification. L'ID du client est automatiquement extrait du token JWT pour prévenir les failles IDOR.
// @Tags Customer KYC
// @Accept json
// @Produce json
// @Param request body customerusecase.UploadKYCRequest true "Détails du document à uploader (le customer_id du body sera ignoré et remplacé par celui du token)"
// @Success 201 {object} map[string]interface{}
// @Failure 400 {object} utils.AppError "Payload invalide ou échec de l'upload"
// @Failure 401 {object} utils.AppError "Non autorisé"
// @Failure 500 {object} utils.AppError "Erreur interne du serveur"
// @Security ApiKeyAuth
// @Router /api/customers/kyc/upload [post]
func (h *KYCHandler) UploadKYC(w http.ResponseWriter, r *http.Request) error {
	ctx := r.Context()
	logger := zerolog.Ctx(ctx)

	// 🛡️ SÉCURITÉ CRITIQUE : Récupérer l'ID de l'utilisateur authentifié depuis le contexte JWT
	authUserID, ok := utils.UserIDFromContext(ctx)
	if !ok || authUserID == "" {
		logger.Warn().Msg("User ID not found in context")
		return utils.ErrUnauthorized
	}

	var req customerusecase.UploadKYCRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		return utils.ErrInvalidPayload
	}
	defer r.Body.Close()

	// 🛡️ SÉCURITÉ CRITIQUE : Écraser le CustomerID de la requête avec l'ID authentifié.
	// Cela empêche un client malveillant de soumettre un document KYC pour le compte d'un autre client (prévention IDOR).
	req.CustomerID = authUserID

	logger.Info().
		Str("customer_id", authUserID).
		Str("document_type", string(req.DocumentType)).
		Msg("Processing KYC upload for authenticated user")

	doc, err := h.uploadKYCUC.Execute(ctx, &req)
	if err != nil {
		return utils.NewAppError("UPLOAD_KYC_FAILED", err.Error(), http.StatusBadRequest)
	}

	utils.WriteJSON(w, http.StatusCreated, doc)
	return nil
}

// @Summary Vérifier le statut KYC d'un client
// @Description Retourne le statut actuel de la vérification d'identité d'un client spécifique.
// @Tags Customer KYC
// @Accept json
// @Produce json
// @Param customer_id path string true "ID du client (UUID)"
// @Success 200 {object} customerusecase.KYCStatusResponse
// @Failure 400 {object} utils.AppError "ID client manquant"
// @Failure 401 {object} utils.AppError "Non autorisé"
// @Failure 404 {object} utils.AppError "Client ou statut introuvable"
// @Failure 500 {object} utils.AppError "Erreur interne du serveur"
// @Security ApiKeyAuth
// @Router /api/customers/{customer_id}/kyc/status [get]
func (h *KYCHandler) GetKYCStatus(w http.ResponseWriter, r *http.Request) error {
	ctx := r.Context()

	customerID := chi.URLParam(r, "customer_id")
	if customerID == "" {
		return utils.ErrInvalidPayload
	}

	response, err := h.getKYCStatusUC.Execute(ctx, customerID)
	if err != nil {
		return utils.NewAppError("GET_KYC_STATUS_FAILED", err.Error(), http.StatusBadRequest)
	}

	utils.WriteJSON(w, http.StatusOK, response)
	return nil
}

// @Summary Réviser une demande KYC (Marchand)
// @Description Permet à un marchand d'approuver ou de rejeter un document KYC soumis par un client.
// @Tags Merchant KYC
// @Accept json
// @Produce json
// @Param customer_id path string true "ID du client (UUID)"
// @Param request body customerusecase.ReviewKYCRequest true "Décision de révision (approved/rejected) et motif"
// @Success 200 {object} customerusecase.ReviewKYCResponse
// @Failure 400 {object} utils.AppError "Payload invalide ou ID manquant"
// @Failure 401 {object} utils.AppError "Non autorisé"
// @Failure 403 {object} utils.AppError "Interdit (droits insuffisants)"
// @Failure 404 {object} utils.AppError "Client ou demande KYC introuvable"
// @Failure 500 {object} utils.AppError "Erreur interne du serveur"
// @Security ApiKeyAuth
// @Router /api/merchant/kyc/{customer_id}/review [post]
func (h *KYCHandler) ReviewKYC(w http.ResponseWriter, r *http.Request) error {
	ctx := r.Context()

	customerID := chi.URLParam(r, "customer_id")
	if customerID == "" {
		return utils.ErrInvalidPayload
	}

	var req customerusecase.ReviewKYCRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		return utils.ErrInvalidPayload
	}
	defer r.Body.Close()

	req.CustomerID = customerID

	response, err := h.reviewKYCUC.Execute(ctx, &req)
	if err != nil {
		return utils.NewAppError("REVIEW_KYC_FAILED", err.Error(), http.StatusBadRequest)
	}

	utils.WriteJSON(w, http.StatusOK, response)
	return nil
}

// @Summary Lister les demandes KYC en attente (Marchand)
// @Description Retourne la liste de tous les clients dont les documents KYC sont en attente de validation par le marchand.
// @Tags Merchant KYC
// @Accept json
// @Produce json
// @Success 200 {array} customerusecase.PendingKYCItem
// @Failure 401 {object} utils.AppError "Non autorisé"
// @Failure 403 {object} utils.AppError "Interdit (droits insuffisants)"
// @Failure 500 {object} utils.AppError "Erreur interne du serveur"
// @Security ApiKeyAuth
// @Router /api/merchant/kyc/pending [get]
func (h *KYCHandler) ListPendingKYC(w http.ResponseWriter, r *http.Request) error {
	ctx := r.Context()

	items, err := h.listPendingKYCUC.Execute(ctx)
	if err != nil {
		return utils.NewAppError("LIST_PENDING_KYC_FAILED", err.Error(), http.StatusBadRequest)
	}

	utils.WriteJSON(w, http.StatusOK, items)
	return nil
}

// RegisterRoutes enregistre les routes KYC dans le router
func (h *KYCHandler) RegisterRoutes(r chi.Router) {
	// Routes client (upload + status)
	r.Route("/customers", func(r chi.Router) {
		r.Post("/kyc/upload", middl.ErrorHandler(h.UploadKYC))
		r.Get("/{customer_id}/kyc/status", middl.ErrorHandler(h.GetKYCStatus))
	})

	// Routes marchand (review + list pending)
	r.Route("/merchant/kyc", func(r chi.Router) {
		r.Get("/pending", middl.ErrorHandler(h.ListPendingKYC))
		r.Post("/{customer_id}/review", middl.ErrorHandler(h.ReviewKYC))
	})
}
