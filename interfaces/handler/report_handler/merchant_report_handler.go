package reporthandling

import (
	"net/http"
	"time"

	reportusecase "Goshop/application/usecase/platform_revenue_report"
	"Goshop/domain/tenant"
	"Goshop/interfaces/utils"
)

type MerchantReportHandler struct {
	merchantStatementUC *reportusecase.MerchantWalletStatementUsecase
}

func NewMerchantReportHandler(merchantStatementUC *reportusecase.MerchantWalletStatementUsecase) *MerchantReportHandler {
	return &MerchantReportHandler{
		merchantStatementUC: merchantStatementUC,
	}
}

func (h *MerchantReportHandler) GetStatement(w http.ResponseWriter, r *http.Request) {
	_, err := tenant.FromContext(r.Context())
	if err != nil {
		utils.WriteJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
		return
	}

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

	stmt, err := h.merchantStatementUC.GetStatement(r.Context(), startDate, endDate)
	if err != nil {
		utils.WriteJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	utils.WriteJSON(w, http.StatusOK, stmt)
}

func (h *MerchantReportHandler) ExportStatementCSV(w http.ResponseWriter, r *http.Request) {
	_, err := tenant.FromContext(r.Context())
	if err != nil {
		utils.WriteJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
		return
	}

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

	stmt, err := h.merchantStatementUC.GetStatement(r.Context(), startDate, endDate)
	if err != nil {
		utils.WriteJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	csvData, err := reportusecase.ExportWalletTransactionsCSV(stmt.Transactions)
	if err != nil {
		utils.WriteJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to generate CSV"})
		return
	}

	w.Header().Set("Content-Type", "text/csv; charset=utf-8")
	w.Header().Set("Content-Disposition", `attachment; filename="releve_marchand.csv"`)
	w.Write(csvData)
}
