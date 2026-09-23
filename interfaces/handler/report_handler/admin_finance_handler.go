package reporthandling

import (
	"net/http"
	"strconv"
	"time"

	reportusecase "Goshop/application/usecase/platform_revenue_report" // ✅ Chemin corrigé vers le bon dossier
	"Goshop/interfaces/utils"
)

type AdminFinanceHandler struct {
	platformRevenueReportUC *reportusecase.PlatformRevenueReportUsecase
}

func NewAdminFinanceHandler(platformRevenueReportUC *reportusecase.PlatformRevenueReportUsecase) *AdminFinanceHandler {
	return &AdminFinanceHandler{
		platformRevenueReportUC: platformRevenueReportUC,
	}
}

func (h *AdminFinanceHandler) GetBalance(w http.ResponseWriter, r *http.Request) {
	balance, err := h.platformRevenueReportUC.GetBalance(r.Context())
	if err != nil {
		utils.WriteJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	utils.WriteJSON(w, http.StatusOK, balance)
}

func (h *AdminFinanceHandler) ListCommissions(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query()
	var startDate, endDate time.Time

	if startStr := query.Get("from"); startStr != "" {
		if t, err := time.Parse(time.RFC3339, startStr); err == nil {
			startDate = t
		}
	}
	if endStr := query.Get("to"); endStr != "" {
		if t, err := time.Parse(time.RFC3339, endStr); err == nil {
			endDate = t
		}
	}

	limit := 50
	if lStr := query.Get("limit"); lStr != "" {
		if l, err := strconv.Atoi(lStr); err == nil && l > 0 {
			limit = l
		}
	}

	offset := 0
	if oStr := query.Get("offset"); oStr != "" {
		if o, err := strconv.Atoi(oStr); err == nil && o >= 0 {
			offset = o
		}
	}

	resp, err := h.platformRevenueReportUC.ListCommissions(r.Context(), startDate, endDate, limit, offset)
	if err != nil {
		utils.WriteJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	utils.WriteJSON(w, http.StatusOK, resp)
}

// ExportCommissionsCSV génère et télécharge un fichier CSV des commissions
func (h *AdminFinanceHandler) ExportCommissionsCSV(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query()
	var startDate, endDate time.Time

	if startStr := query.Get("from"); startStr != "" {
		if t, err := time.Parse(time.RFC3339, startStr); err == nil {
			startDate = t
		}
	}
	if endStr := query.Get("to"); endStr != "" {
		if t, err := time.Parse(time.RFC3339, endStr); err == nil {
			endDate = t
		}
	}

	// Pour l'export, on récupère jusqu'à 5000 lignes
	resp, err := h.platformRevenueReportUC.ListCommissions(r.Context(), startDate, endDate, 5000, 0)
	if err != nil {
		utils.WriteJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	// Note : Assure-toi que la fonction ExportPlatformCommissionsCSV est bien dans le même package 'reportusecase'
	csvData, err := reportusecase.ExportPlatformCommissionsCSV(resp.Transactions)
	if err != nil {
		utils.WriteJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to generate CSV"})
		return
	}

	w.Header().Set("Content-Type", "text/csv; charset=utf-8")
	w.Header().Set("Content-Disposition", `attachment; filename="commissions_plateforme.csv"`)
	w.Write(csvData)
}
