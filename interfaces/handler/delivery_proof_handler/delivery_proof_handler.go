package deliveryproofhandler

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"Goshop/application/metrics"
	deliveryproofusecase "Goshop/application/usecase/delivery_proof_usecase"
	"Goshop/domain/tenant"
	"Goshop/interfaces/utils"

	"github.com/rs/zerolog"
)

// ============================================================
// DELIVERY PROOF HANDLER
// ============================================================

// DeliveryProofHandler gère les endpoints liés aux preuves de livraison
type DeliveryProofHandler struct {
	submitShippingUC        *deliveryproofusecase.SubmitShippingProofUsecase
	submitTontineShippingUC *deliveryproofusecase.SubmitTontineShippingProofUsecase
	submitDeliveryUC        *deliveryproofusecase.SubmitDeliveryProofUsecase
	submitTontineDeliveryUC *deliveryproofusecase.SubmitTontineDeliveryProofUsecase
}

// NewDeliveryProofHandler crée une nouvelle instance du handler
func NewDeliveryProofHandler(
	submitShippingUC *deliveryproofusecase.SubmitShippingProofUsecase,
	submitTontineShippingUC *deliveryproofusecase.SubmitTontineShippingProofUsecase,
	submitDeliveryUC *deliveryproofusecase.SubmitDeliveryProofUsecase,
	submitTontineDeliveryUC *deliveryproofusecase.SubmitTontineDeliveryProofUsecase,
) *DeliveryProofHandler {
	return &DeliveryProofHandler{
		submitShippingUC:        submitShippingUC,
		submitTontineShippingUC: submitTontineShippingUC,
		submitDeliveryUC:        submitDeliveryUC,
		submitTontineDeliveryUC: submitTontineDeliveryUC,
	}
}

// ============================================================
// HANDLERS : MARCHAND (EXPÉDITION)
// ============================================================

// @Summary Soumettre une preuve d'expédition (commande)
// @Description Permet au marchand de soumettre la preuve d'expédition pour une commande payée en ligne
// @Tags Delivery Proof
// @Accept json
// @Produce json
// @Param request body deliveryproofusecase.SubmitShippingProofRequest true "Détails de la preuve d'expédition"
// @Success 200 {object} deliveryproofusecase.SubmitShippingProofResponse
// @Failure 400 {object} utils.AppError "Payload invalide ou champs manquants"
// @Failure 401 {object} utils.AppError "Contexte multi-tenant requis"
// @Failure 403 {object} utils.AppError "Accès refusé"
// @Failure 500 {object} utils.AppError "Erreur interne du serveur"
// @Security ApiKeyAuth
// @Router /api/delivery/proof/shipping [post]
func (h *DeliveryProofHandler) SubmitShippingProof(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	start := time.Now()
	logger := zerolog.Ctx(ctx)

	_, err := tenant.FromContext(ctx)
	if err != nil {
		// 📊 MÉTRIQUE : Erreur tenant
		metrics.DeliveryProofTenantErrors.Inc()
		metrics.DeliveryProofSubmitTotal.WithLabelValues("shipping", "error").Inc()
		utils.WriteError(w, http.StatusUnauthorized, "Multi-tenant context required")
		return
	}

	var req deliveryproofusecase.SubmitShippingProofRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		// 📊 MÉTRIQUE : Erreur payload
		metrics.DeliveryProofPayloadErrors.Inc()
		metrics.DeliveryProofSubmitTotal.WithLabelValues("shipping", "error").Inc()
		utils.WriteError(w, http.StatusBadRequest, "Invalid JSON body")
		return
	}
	defer r.Body.Close()

	resp, err := h.submitShippingUC.Execute(ctx, &req)
	duration := time.Since(start).Seconds()

	if err != nil {
		// 📊 MÉTRIQUES : Échec soumission
		metrics.DeliveryProofSubmitTotal.WithLabelValues("shipping", "error").Inc()
		metrics.DeliveryProofDuration.WithLabelValues("shipping").Observe(duration)
		metrics.ApplicationErrorsTotal.WithLabelValues("delivery_proof_shipping", "delivery_proof_handler").Inc()

		logger.Error().Err(err).
			Str("order_id", req.OrderID).
			Float64("duration_seconds", duration).
			Msg("Failed to submit shipping proof")

		utils.WriteError(w, http.StatusBadRequest, fmt.Sprintf("Failed to submit shipping proof: %v", err))
		return
	}

	// 📊 MÉTRIQUES : Succès soumission
	metrics.DeliveryProofSubmitTotal.WithLabelValues("shipping", "success").Inc()
	metrics.DeliveryProofDuration.WithLabelValues("shipping").Observe(duration)

	logger.Info().
		Str("order_id", req.OrderID).
		Str("proof_id", resp.ProofID).
		Float64("duration_seconds", duration).
		Msg("✅ Shipping proof submitted successfully")

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
// @Failure 403 {object} utils.AppError "Accès refusé"
// @Failure 500 {object} utils.AppError "Erreur interne du serveur"
// @Security ApiKeyAuth
// @Router /api/delivery/proof/tontine-shipping [post]
func (h *DeliveryProofHandler) SubmitTontineShippingProof(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	start := time.Now()
	logger := zerolog.Ctx(ctx)

	_, err := tenant.FromContext(ctx)
	if err != nil {
		// 📊 MÉTRIQUE : Erreur tenant
		metrics.DeliveryProofTenantErrors.Inc()
		metrics.DeliveryProofSubmitTotal.WithLabelValues("tontine_shipping", "error").Inc()
		utils.WriteError(w, http.StatusUnauthorized, "Multi-tenant context required")
		return
	}

	var req deliveryproofusecase.SubmitTontineShippingProofRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		// 📊 MÉTRIQUE : Erreur payload
		metrics.DeliveryProofPayloadErrors.Inc()
		metrics.DeliveryProofSubmitTotal.WithLabelValues("tontine_shipping", "error").Inc()
		utils.WriteError(w, http.StatusBadRequest, "Invalid JSON body")
		return
	}
	defer r.Body.Close()

	resp, err := h.submitTontineShippingUC.Execute(ctx, &req)
	duration := time.Since(start).Seconds()

	if err != nil {
		// 📊 MÉTRIQUES : Échec soumission
		metrics.DeliveryProofSubmitTotal.WithLabelValues("tontine_shipping", "error").Inc()
		metrics.DeliveryProofDuration.WithLabelValues("tontine_shipping").Observe(duration)
		metrics.ApplicationErrorsTotal.WithLabelValues("delivery_proof_tontine_shipping", "delivery_proof_handler").Inc()

		logger.Error().Err(err).
			Str("voucher_id", req.VoucherID).
			Float64("duration_seconds", duration).
			Msg("Failed to submit tontine shipping proof")

		utils.WriteError(w, http.StatusBadRequest, fmt.Sprintf("Failed to submit tontine shipping proof: %v", err))
		return
	}

	// 📊 MÉTRIQUES : Succès soumission
	metrics.DeliveryProofSubmitTotal.WithLabelValues("tontine_shipping", "success").Inc()
	metrics.DeliveryProofDuration.WithLabelValues("tontine_shipping").Observe(duration)

	logger.Info().
		Str("voucher_id", req.VoucherID).
		Str("proof_id", resp.ProofID).
		Float64("duration_seconds", duration).
		Msg("✅ Tontine shipping proof submitted successfully")

	utils.WriteJSON(w, http.StatusOK, resp)
}

// ============================================================
// HANDLERS : CLIENT (RÉCEPTION)
// ============================================================

// @Summary Confirmer la réception d'une commande
// @Description Permet au client de confirmer la réception d'une commande (optionnel, déclenche la fenêtre de litige de 72h)
// @Tags Delivery Proof
// @Accept json
// @Produce json
// @Param request body deliveryproofusecase.SubmitDeliveryProofRequest true "Détails de la confirmation de réception"
// @Success 200 {object} deliveryproofusecase.SubmitDeliveryProofResponse
// @Failure 400 {object} utils.AppError "Payload invalide ou champs manquants"
// @Failure 401 {object} utils.AppError "Contexte multi-tenant requis"
// @Failure 403 {object} utils.AppError "Accès refusé"
// @Failure 500 {object} utils.AppError "Erreur interne du serveur"
// @Security ApiKeyAuth
// @Router /api/delivery/proof/delivery [post]
func (h *DeliveryProofHandler) SubmitDeliveryProof(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	start := time.Now()
	logger := zerolog.Ctx(ctx)

	_, err := tenant.FromContext(ctx)
	if err != nil {
		// 📊 MÉTRIQUE : Erreur tenant
		metrics.DeliveryProofTenantErrors.Inc()
		metrics.DeliveryProofSubmitTotal.WithLabelValues("delivery", "error").Inc()
		utils.WriteError(w, http.StatusUnauthorized, "Multi-tenant context required")
		return
	}

	var req deliveryproofusecase.SubmitDeliveryProofRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		// 📊 MÉTRIQUE : Erreur payload
		metrics.DeliveryProofPayloadErrors.Inc()
		metrics.DeliveryProofSubmitTotal.WithLabelValues("delivery", "error").Inc()
		utils.WriteError(w, http.StatusBadRequest, "Invalid JSON body")
		return
	}
	defer r.Body.Close()

	resp, err := h.submitDeliveryUC.Execute(ctx, &req)
	duration := time.Since(start).Seconds()

	if err != nil {
		// 📊 MÉTRIQUES : Échec soumission
		metrics.DeliveryProofSubmitTotal.WithLabelValues("delivery", "error").Inc()
		metrics.DeliveryProofDuration.WithLabelValues("delivery").Observe(duration)
		metrics.ApplicationErrorsTotal.WithLabelValues("delivery_proof_delivery", "delivery_proof_handler").Inc()

		logger.Error().Err(err).
			Str("order_id", req.OrderID).
			Float64("duration_seconds", duration).
			Msg("Failed to submit delivery proof")

		utils.WriteError(w, http.StatusBadRequest, fmt.Sprintf("Failed to submit delivery proof: %v", err))
		return
	}

	// 📊 MÉTRIQUES : Succès soumission
	metrics.DeliveryProofSubmitTotal.WithLabelValues("delivery", "success").Inc()
	metrics.DeliveryProofDuration.WithLabelValues("delivery").Observe(duration)

	logger.Info().
		Str("order_id", req.OrderID).
		Str("proof_id", resp.ProofID).
		Float64("duration_seconds", duration).
		Msg("✅ Delivery proof submitted successfully by customer")

	utils.WriteJSON(w, http.StatusOK, resp)
}

// @Summary Confirmer la réception d'un voucher tontine
// @Description Permet au participant de confirmer la réception d'un bien via voucher tontine (optionnel, déclenche la fenêtre de litige de 72h)
// @Tags Delivery Proof
// @Accept json
// @Produce json
// @Param request body deliveryproofusecase.SubmitTontineDeliveryProofRequest true "Détails de la confirmation de réception"
// @Success 200 {object} deliveryproofusecase.SubmitTontineDeliveryProofResponse
// @Failure 400 {object} utils.AppError "Payload invalide ou champs manquants"
// @Failure 401 {object} utils.AppError "Contexte multi-tenant requis"
// @Failure 403 {object} utils.AppError "Accès refusé"
// @Failure 500 {object} utils.AppError "Erreur interne du serveur"
// @Security ApiKeyAuth
// @Router /api/delivery/proof/tontine-delivery [post]
func (h *DeliveryProofHandler) SubmitTontineDeliveryProof(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	start := time.Now()
	logger := zerolog.Ctx(ctx)

	_, err := tenant.FromContext(ctx)
	if err != nil {
		// 📊 MÉTRIQUE : Erreur tenant
		metrics.DeliveryProofTenantErrors.Inc()
		metrics.DeliveryProofSubmitTotal.WithLabelValues("tontine_delivery", "error").Inc()
		utils.WriteError(w, http.StatusUnauthorized, "Multi-tenant context required")
		return
	}

	var req deliveryproofusecase.SubmitTontineDeliveryProofRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		// 📊 MÉTRIQUE : Erreur payload
		metrics.DeliveryProofPayloadErrors.Inc()
		metrics.DeliveryProofSubmitTotal.WithLabelValues("tontine_delivery", "error").Inc()
		utils.WriteError(w, http.StatusBadRequest, "Invalid JSON body")
		return
	}
	defer r.Body.Close()

	resp, err := h.submitTontineDeliveryUC.Execute(ctx, &req)
	duration := time.Since(start).Seconds()

	if err != nil {
		// 📊 MÉTRIQUES : Échec soumission
		metrics.DeliveryProofSubmitTotal.WithLabelValues("tontine_delivery", "error").Inc()
		metrics.DeliveryProofDuration.WithLabelValues("tontine_delivery").Observe(duration)
		metrics.ApplicationErrorsTotal.WithLabelValues("delivery_proof_tontine_delivery", "delivery_proof_handler").Inc()

		logger.Error().Err(err).
			Str("voucher_id", req.VoucherID).
			Float64("duration_seconds", duration).
			Msg("Failed to submit tontine delivery proof")

		utils.WriteError(w, http.StatusBadRequest, fmt.Sprintf("Failed to submit tontine delivery proof: %v", err))
		return
	}

	// 📊 MÉTRIQUES : Succès soumission
	metrics.DeliveryProofSubmitTotal.WithLabelValues("tontine_delivery", "success").Inc()
	metrics.DeliveryProofDuration.WithLabelValues("tontine_delivery").Observe(duration)

	logger.Info().
		Str("voucher_id", req.VoucherID).
		Str("proof_id", resp.ProofID).
		Float64("duration_seconds", duration).
		Msg("✅ Tontine delivery proof submitted successfully by participant")

	utils.WriteJSON(w, http.StatusOK, resp)
}
