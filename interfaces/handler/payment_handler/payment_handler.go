package paymenthandler

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	paymentdto "Goshop/application/dto/payment_dto"
	paymentusecase "Goshop/application/usecase/payment_usecase"
	"Goshop/domain/entity"
	"Goshop/interfaces/utils"

	"github.com/go-chi/chi/v5"
	"github.com/rs/zerolog"
)

// ============ INTERFACES POUR LES USECASES ============

// InitiatePaymentUseCaseInterface définit le contrat
type InitiatePaymentUseCaseInterface interface {
	Execute(ctx context.Context, req *paymentdto.InitiatePaymentRequest) (*paymentdto.InitiatePaymentResponse, error)
}

// CheckPaymentStatusUseCaseInterface définit le contrat
type CheckPaymentStatusUseCaseInterface interface {
	Execute(ctx context.Context, paymentID string) (*paymentdto.PaymentResponse, error)
}

// ListPaymentsUseCaseInterface définit le contrat
type ListPaymentsUseCaseInterface interface {
	Execute(ctx context.Context, req *paymentusecase.ListPaymentsRequest) ([]*paymentdto.PaymentResponse, error)
}

// RefundPaymentUseCaseInterface définit le contrat
type RefundPaymentUseCaseInterface interface {
	Execute(ctx context.Context, req *paymentdto.RefundPaymentRequest) (*paymentdto.PaymentResponse, error)
}

// CompletePaymentUseCaseInterface définit le contrat
type CompletePaymentUseCaseInterface interface {
	Execute(ctx context.Context, req *paymentdto.CompletePaymentRequest) (*paymentdto.PaymentResponse, error)
}

// ============ HANDLER ============

// PaymentHandler gère les requêtes HTTP pour les paiements
type PaymentHandler struct {
	initiateUC InitiatePaymentUseCaseInterface
	checkUC    CheckPaymentStatusUseCaseInterface
	listUC     ListPaymentsUseCaseInterface
	refundUC   RefundPaymentUseCaseInterface
	completeUC CompletePaymentUseCaseInterface // 🆕 Ajouté
}

// NewPaymentHandler crée une nouvelle instance
func NewPaymentHandler(
	initiateUC InitiatePaymentUseCaseInterface,
	checkUC CheckPaymentStatusUseCaseInterface,
	listUC ListPaymentsUseCaseInterface,
	refundUC RefundPaymentUseCaseInterface,
	completeUC CompletePaymentUseCaseInterface, // 🆕 Ajouté
) *PaymentHandler {
	return &PaymentHandler{
		initiateUC: initiateUC,
		checkUC:    checkUC,
		listUC:     listUC,
		refundUC:   refundUC,
		completeUC: completeUC,
	}
}

// @Summary Initier un paiement pour une commande
// @Description Lance le processus de paiement pour une commande donnée via le fournisseur de paiement choisi.
// @Tags Payments
// @Accept json
// @Produce json
// @Param id path string true "ID de la commande (UUID)"
// @Param request body paymentdto.InitiatePaymentRequest true "Détails du paiement (provider, phone_number, etc.)"
// @Success 201 {object} paymentdto.InitiatePaymentResponse
// @Failure 400 {object} utils.AppError "Payload invalide ou commande non éligible"
// @Failure 401 {object} utils.AppError "Non autorisé"
// @Failure 404 {object} utils.AppError "Commande introuvable"
// @Failure 409 {object} utils.AppError "Un paiement est déjà en cours pour cette commande"
// @Failure 500 {object} utils.AppError "Erreur interne du serveur"
// @Security ApiKeyAuth
// @Router /api/orders/{id}/pay [post]
func (h *PaymentHandler) InitiatePayment(w http.ResponseWriter, r *http.Request) error {
	ctx := r.Context()
	logger := zerolog.Ctx(ctx)

	orderID := chi.URLParam(r, "id")
	if orderID == "" {
		return utils.ErrInvalidPayload
	}

	var req paymentdto.InitiatePaymentRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		logger.Error().Err(err).Msg("Invalid JSON payload")
		return utils.ErrInvalidPayload
	}

	req.OrderID = orderID

	if err := req.Validate(); err != nil {
		logger.Warn().Err(err).Msg("Validation failed")
		return utils.ErrValidationFailed
	}

	resp, err := h.initiateUC.Execute(ctx, &req)
	if err != nil {
		logger.Error().Err(err).Msg("Failed to initiate payment")

		errMsg := err.Error()
		switch {
		case errors.Is(err, context.DeadlineExceeded):
			return utils.ErrInternalServer
		case errMsg == "order already has an active payment":
			return utils.NewAppError("PAYMENT_ALREADY_EXISTS", errMsg, http.StatusConflict)
		case errMsg == "order not found":
			return utils.ErrOrderNotFound
		case errMsg == "provider not available":
			return utils.NewAppError("PROVIDER_UNAVAILABLE", errMsg, http.StatusServiceUnavailable)
		default:
			return utils.NewAppError("PAYMENT_INITIATION_FAILED", errMsg, http.StatusBadRequest)
		}
	}

	utils.WriteJSON(w, http.StatusCreated, resp)
	return nil
}

// @Summary Récupérer les détails d'un paiement
// @Description Retourne le statut et les informations complètes d'une transaction de paiement spécifique.
// @Tags Payments
// @Accept json
// @Produce json
// @Param id path string true "ID du paiement (UUID)"
// @Success 200 {object} paymentdto.PaymentResponse
// @Failure 400 {object} utils.AppError "ID de paiement manquant"
// @Failure 401 {object} utils.AppError "Non autorisé"
// @Failure 404 {object} utils.AppError "Paiement introuvable"
// @Failure 500 {object} utils.AppError "Erreur interne du serveur"
// @Security ApiKeyAuth
// @Router /api/payments/{id} [get]
func (h *PaymentHandler) GetPayment(w http.ResponseWriter, r *http.Request) error {
	ctx := r.Context()
	logger := zerolog.Ctx(ctx)

	paymentID := chi.URLParam(r, "id")
	if paymentID == "" {
		return utils.ErrInvalidPayload
	}

	resp, err := h.checkUC.Execute(ctx, paymentID)
	if err != nil {
		logger.Error().Err(err).Msg("Failed to get payment")

		errMsg := err.Error()
		switch {
		case errMsg == "payment not found":
			return utils.NewAppError("PAYMENT_NOT_FOUND", errMsg, http.StatusNotFound)
		default:
			return utils.ErrInternalServer
		}
	}

	utils.WriteJSON(w, http.StatusOK, resp)
	return nil
}

// @Summary Lister les paiements
// @Description Retourne la liste paginée et filtrée des paiements pour la boutique active.
// @Tags Payments
// @Accept json
// @Produce json
// @Param status query string false "Filtrer par statut de paiement (ex: success, pending, failed)"
// @Param provider query string false "Filtrer par fournisseur (ex: wave, orange_money)"
// @Success 200 {array} paymentdto.PaymentResponse
// @Failure 401 {object} utils.AppError "Non autorisé"
// @Failure 500 {object} utils.AppError "Erreur interne du serveur"
// @Security ApiKeyAuth
// @Router /api/payments [get]
func (h *PaymentHandler) ListPayments(w http.ResponseWriter, r *http.Request) error {
	ctx := r.Context()
	logger := zerolog.Ctx(ctx)

	req := &paymentusecase.ListPaymentsRequest{
		Limit:  50,
		Offset: 0,
	}

	if status := r.URL.Query().Get("status"); status != "" {
		s := entity.PaymentStatus(status)
		req.Status = &s
	}

	if provider := r.URL.Query().Get("provider"); provider != "" {
		p := entity.PaymentProvider(provider)
		req.Provider = &p
	}

	resp, err := h.listUC.Execute(ctx, req)
	if err != nil {
		logger.Error().Err(err).Msg("Failed to list payments")
		return utils.ErrInternalServer
	}

	utils.WriteJSON(w, http.StatusOK, resp)
	return nil
}

// @Summary Rembourser un paiement
// @Description Lance une demande de remboursement pour une transaction de paiement réussie.
// @Tags Payments
// @Accept json
// @Produce json
// @Param id path string true "ID du paiement (UUID)"
// @Param request body paymentdto.RefundPaymentRequest true "Détails du remboursement (montant, motif)"
// @Success 200 {object} paymentdto.PaymentResponse
// @Failure 400 {object} utils.AppError "Payload invalide ou remboursement impossible"
// @Failure 401 {object} utils.AppError "Non autorisé"
// @Failure 403 {object} utils.AppError "Le paiement n'appartient pas à votre boutique"
// @Failure 404 {object} utils.AppError "Paiement introuvable"
// @Failure 500 {object} utils.AppError "Erreur interne du serveur"
// @Security ApiKeyAuth
// @Router /api/payments/{id}/refund [post]
func (h *PaymentHandler) RefundPayment(w http.ResponseWriter, r *http.Request) error {
	ctx := r.Context()
	logger := zerolog.Ctx(ctx)

	paymentID := chi.URLParam(r, "id")
	if paymentID == "" {
		return utils.ErrInvalidPayload
	}

	var req paymentdto.RefundPaymentRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		logger.Error().Err(err).Msg("Invalid JSON payload")
		return utils.ErrInvalidPayload
	}

	req.PaymentID = paymentID

	if err := req.Validate(); err != nil {
		logger.Warn().Err(err).Msg("Validation failed")
		return utils.ErrValidationFailed
	}

	resp, err := h.refundUC.Execute(ctx, &req)
	if err != nil {
		logger.Error().Err(err).Msg("Failed to refund payment")

		errMsg := err.Error()
		switch {
		case errMsg == "payment not found":
			return utils.NewAppError("PAYMENT_NOT_FOUND", errMsg, http.StatusNotFound)
		case errMsg == "payment does not belong to current shop":
			return utils.ErrForbidden
		default:
			return utils.NewAppError("REFUND_FAILED", errMsg, http.StatusBadRequest)
		}
	}

	utils.WriteJSON(w, http.StatusOK, resp)
	return nil
}

// @Summary Compléter un paiement en deux étapes (OTP)
// @Description Finalise un paiement qui nécessite une validation OTP (ex: certains flux Mobile Money).
// @Tags Payments
// @Accept json
// @Produce json
// @Param id path string true "ID du paiement (UUID)"
// @Param request body paymentdto.CompletePaymentRequest true "Code OTP de validation"
// @Success 200 {object} paymentdto.PaymentResponse
// @Failure 400 {object} utils.AppError "Payload invalide, OTP incorrect ou opération non supportée"
// @Failure 401 {object} utils.AppError "Non autorisé"
// @Failure 403 {object} utils.AppError "Le paiement n'appartient pas à votre boutique"
// @Failure 404 {object} utils.AppError "Paiement introuvable"
// @Failure 500 {object} utils.AppError "Erreur interne du serveur"
// @Security ApiKeyAuth
// @Router /api/payments/{id}/complete [post]
func (h *PaymentHandler) CompletePayment(w http.ResponseWriter, r *http.Request) error {
	ctx := r.Context()
	logger := zerolog.Ctx(ctx)

	paymentID := chi.URLParam(r, "id")
	if paymentID == "" {
		return utils.ErrInvalidPayload
	}

	var req paymentdto.CompletePaymentRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		logger.Error().Err(err).Msg("Invalid JSON payload")
		return utils.ErrInvalidPayload
	}

	req.PaymentID = paymentID

	if err := req.Validate(); err != nil {
		logger.Warn().Err(err).Msg("Validation failed")
		return utils.ErrValidationFailed
	}

	resp, err := h.completeUC.Execute(ctx, &req)
	if err != nil {
		logger.Error().Err(err).Msg("Failed to complete payment")

		errMsg := err.Error()
		switch {
		case errMsg == "payment not found":
			return utils.NewAppError("PAYMENT_NOT_FOUND", errMsg, http.StatusNotFound)
		case errMsg == "payment does not belong to current shop":
			return utils.ErrForbidden
		default:
			if strings.Contains(errMsg, "does not support payment completion") {
				return utils.NewAppError("OPERATION_NOT_SUPPORTED", errMsg, http.StatusBadRequest)
			}
			return utils.NewAppError("PAYMENT_COMPLETION_FAILED", errMsg, http.StatusBadRequest)
		}
	}

	utils.WriteJSON(w, http.StatusOK, resp)
	return nil
}
