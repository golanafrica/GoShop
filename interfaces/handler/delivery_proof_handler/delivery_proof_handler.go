package deliveryproofhandler

import (
	"encoding/json"
	"fmt"
	"net/http"

	deliveryproofusecase "Goshop/application/usecase/delivery_proof_usecase"
	"Goshop/domain/tenant"
	"Goshop/interfaces/utils"

	"github.com/go-chi/chi/v5"
	"github.com/rs/zerolog"
)

// ============================================================
// DELIVERY PROOF HANDLER
// ============================================================

// DeliveryProofHandler gère les endpoints liés aux preuves de livraison
type DeliveryProofHandler struct {
	submitShippingUC        *deliveryproofusecase.SubmitShippingProofUsecase
	submitTontineShippingUC *deliveryproofusecase.SubmitTontineShippingProofUsecase
}

// NewDeliveryProofHandler crée une nouvelle instance du handler
func NewDeliveryProofHandler(
	submitShippingUC *deliveryproofusecase.SubmitShippingProofUsecase,
	submitTontineShippingUC *deliveryproofusecase.SubmitTontineShippingProofUsecase,
) *DeliveryProofHandler {
	return &DeliveryProofHandler{
		submitShippingUC:        submitShippingUC,
		submitTontineShippingUC: submitTontineShippingUC,
	}
}

// ============================================================
// HANDLERS : SUBMIT SHIPPING PROOF
// ============================================================

// @Summary Soumettre une preuve d'expédition (commande)
// @Description Permet au marchand de soumettre la preuve d'expédition pour une commande payée en ligne (Mobile Money, Wave, etc.)
// @Tags Delivery Proof
// @Accept json
// @Produce json
// @Param request body deliveryproofusecase.SubmitShippingProofRequest true "Détails de la preuve d'expédition"
// @Success 200 {object} deliveryproofusecase.SubmitShippingProofResponse
// @Failure 400 {object} utils.AppError "Payload invalide ou champs manquants"
// @Failure 401 {object} utils.AppError "Contexte multi-tenant requis"
// @Failure 403 {object} utils.AppError "Accès refusé : la commande n'appartient pas à votre boutique"
// @Failure 500 {object} utils.AppError "Erreur interne du serveur"
// @Security ApiKeyAuth
// @Router /api/delivery/proof/shipping [post]
func (h *DeliveryProofHandler) SubmitShippingProof(w http.ResponseWriter, r *http.Request) {
	logger := zerolog.Ctx(r.Context())

	// 1. Récupérer le shop
	_, err := tenant.FromContext(r.Context())
	if err != nil {
		utils.WriteError(w, http.StatusUnauthorized, "Multi-tenant context required")
		return
	}

	// 2. Parser la requête
	var req deliveryproofusecase.SubmitShippingProofRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		utils.WriteError(w, http.StatusBadRequest, "Invalid JSON body")
		return
	}
	defer r.Body.Close()

	// 3. Appeler le usecase
	resp, err := h.submitShippingUC.Execute(r.Context(), &req)
	if err != nil {
		logger.Error().Err(err).Msg("Failed to submit shipping proof")
		utils.WriteError(w, http.StatusBadRequest, fmt.Sprintf("Failed to submit shipping proof: %v", err))
		return
	}

	// 4. Logger et retourner
	logger.Info().
		Str("order_id", req.OrderID).
		Str("proof_id", resp.ProofID).
		Msg("Shipping proof submitted successfully")

	utils.WriteJSON(w, http.StatusOK, resp)
}

// @Summary Soumettre une preuve d'expédition (voucher tontine)
// @Description Permet au marchand de soumettre la preuve d'expédition pour un voucher tontine
// @Tags Delivery Proof
// @Accept json
// @Produce json
// @Param request body deliveryproofusecase.SubmitTontineShippingProofRequest true "Détails de la preuve d'expédition"
// @Success 200 {object} deliveryproofusecase.SubmitTontineShippingProofResponse
// @Failure 400 {object} utils.AppError "Payload invalide ou champs manquants"
// @Failure 401 {object} utils.AppError "Contexte multi-tenant requis"
// @Failure 403 {object} utils.AppError "Accès refusé : le voucher n'appartient pas à votre boutique"
// @Failure 500 {object} utils.AppError "Erreur interne du serveur"
// @Security ApiKeyAuth
// @Router /api/delivery/proof/tontine-shipping [post]
func (h *DeliveryProofHandler) SubmitTontineShippingProof(w http.ResponseWriter, r *http.Request) {
	logger := zerolog.Ctx(r.Context())

	// 1. Récupérer le shop
	_, err := tenant.FromContext(r.Context())
	if err != nil {
		utils.WriteError(w, http.StatusUnauthorized, "Multi-tenant context required")
		return
	}

	// 2. Parser la requête
	var req deliveryproofusecase.SubmitTontineShippingProofRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		utils.WriteError(w, http.StatusBadRequest, "Invalid JSON body")
		return
	}
	defer r.Body.Close()

	// 3. Appeler le usecase
	resp, err := h.submitTontineShippingUC.Execute(r.Context(), &req)
	if err != nil {
		logger.Error().Err(err).Msg("Failed to submit tontine shipping proof")
		utils.WriteError(w, http.StatusBadRequest, fmt.Sprintf("Failed to submit tontine shipping proof: %v", err))
		return
	}

	// 4. Logger et retourner
	logger.Info().
		Str("voucher_id", req.VoucherID).
		Str("proof_id", resp.ProofID).
		Msg("Tontine shipping proof submitted successfully")

	utils.WriteJSON(w, http.StatusOK, resp)
}

// ============================================================
// ROUTER SETUP
// ============================================================

// RegisterRoutes enregistre les routes du delivery proof handler
func (h *DeliveryProofHandler) RegisterRoutes(r chi.Router) {
	r.Post("/shipping", h.SubmitShippingProof)
	r.Post("/tontine-shipping", h.SubmitTontineShippingProof)
}
