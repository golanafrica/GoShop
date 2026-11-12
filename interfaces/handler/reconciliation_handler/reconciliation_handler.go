package reconciliationhandler

import (
	"net/http"
	"strconv"

	reconciliationusecase "Goshop/application/usecase/reconciliation_usecase"
	"Goshop/interfaces/utils"

	"github.com/go-chi/chi/v5"
)

type ReconciliationHandler struct {
	uc *reconciliationusecase.ReconciliationUsecase
}

func NewReconciliationHandler(uc *reconciliationusecase.ReconciliationUsecase) *ReconciliationHandler {
	return &ReconciliationHandler{
		uc: uc,
	}
}

// GetShopReconciliation vérifie la cohérence financière d'un shop spécifique
func (h *ReconciliationHandler) GetShopReconciliation(w http.ResponseWriter, r *http.Request) {
	shopID := chi.URLParam(r, "shop_id")
	if shopID == "" {
		utils.WriteJSON(w, http.StatusBadRequest, map[string]string{"error": "shop_id is required"})
		return
	}

	resp, err := h.uc.CheckShop(r.Context(), shopID)
	if err != nil {
		utils.WriteJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	utils.WriteJSON(w, http.StatusOK, resp)
}

// GetGlobalReconciliation retourne un résumé agrégé pour le tableau de bord admin
func (h *ReconciliationHandler) GetGlobalReconciliation(w http.ResponseWriter, r *http.Request) {
	limitStr := r.URL.Query().Get("limit")
	limit := 500 // Défaut raisonnable pour éviter de surcharger la DB
	if limitStr != "" {
		if parsed, err := strconv.Atoi(limitStr); err == nil && parsed > 0 {
			limit = parsed
		}
	}

	resp, err := h.uc.CheckGlobal(r.Context(), limit)
	if err != nil {
		utils.WriteJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	utils.WriteJSON(w, http.StatusOK, resp)
}
