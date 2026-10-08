package freezejobhandler

import (
	"encoding/json"
	"net/http"

	freezejobusecase "Goshop/application/usecase/freeze_job_usecase"
	"Goshop/interfaces/utils"
)

type FreezeJobHandler struct {
	uc *freezejobusecase.FreezeJobUsecase
}

func NewFreezeJobHandler(uc *freezejobusecase.FreezeJobUsecase) *FreezeJobHandler {
	return &FreezeJobHandler{uc: uc}
}

func (h *FreezeJobHandler) GetConfig(w http.ResponseWriter, r *http.Request) error {
	cfg, err := h.uc.GetConfig(r.Context())
	if err != nil {
		return utils.NewAppError("GET_CONFIG_FAILED", err.Error(), http.StatusInternalServerError)
	}
	utils.WriteJSON(w, http.StatusOK, map[string]interface{}{"success": true, "config": cfg})
	return nil
}

func (h *FreezeJobHandler) UpdateConfig(w http.ResponseWriter, r *http.Request) error {
	var cfg freezejobusecase.FreezeJobConfig
	if err := json.NewDecoder(r.Body).Decode(&cfg); err != nil {
		return utils.NewAppError("INVALID_PAYLOAD", err.Error(), http.StatusBadRequest)
	}

	if err := h.uc.UpdateConfig(r.Context(), &cfg); err != nil {
		return utils.NewAppError("UPDATE_CONFIG_FAILED", err.Error(), http.StatusInternalServerError)
	}

	utils.WriteJSON(w, http.StatusOK, map[string]interface{}{"success": true, "message": "Configuration updated successfully"})
	return nil
}

func (h *FreezeJobHandler) RunJob(w http.ResponseWriter, r *http.Request) error {
	mode := r.URL.Query().Get("mode")
	if mode == "" {
		mode = "live"
	}
	if mode != "live" && mode != "dry_run" {
		return utils.NewAppError("INVALID_MODE", "mode must be 'live' or 'dry_run'", http.StatusBadRequest)
	}

	h.uc.RunJob(r.Context(), mode)
	utils.WriteJSON(w, http.StatusAccepted, map[string]interface{}{"success": true, "message": "Job triggered successfully in background", "mode": mode})
	return nil
}

func (h *FreezeJobHandler) ListGraceExpired(w http.ResponseWriter, r *http.Request) error {
	wallets, err := h.uc.ListGraceExpired(r.Context())
	if err != nil {
		return utils.NewAppError("LIST_EXPIRED_FAILED", err.Error(), http.StatusInternalServerError)
	}
	utils.WriteJSON(w, http.StatusOK, map[string]interface{}{"success": true, "wallets": wallets, "count": len(wallets)})
	return nil
}
