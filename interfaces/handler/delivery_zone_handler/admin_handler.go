package delivery_zone_handler

import (
	"encoding/json"
	"net/http"

	deliveryzonedto "Goshop/application/dto/delivery_zone_dto"
	delivery_zone_usecase "Goshop/application/usecase/delivery_zone_usecase"
	"Goshop/interfaces/utils"
)

type AdminDeliveryZoneHandler struct {
	uc *delivery_zone_usecase.AdminDeliveryZoneUsecase
}

func NewAdminDeliveryZoneHandler(uc *delivery_zone_usecase.AdminDeliveryZoneUsecase) *AdminDeliveryZoneHandler {
	return &AdminDeliveryZoneHandler{uc: uc}
}

// CreateZone godoc
// @Summary Créer une nouvelle zone de livraison
// @Tags Admin Delivery Zones
// @Accept json
// @Produce json
// @Param request body delivery_zone_dto.CreateZoneRequest true "Détails de la zone"
// @Success 201 {object} delivery_zone_dto.ZoneResponse
// @Failure 400 {object} utils.AppError
// @Failure 500 {object} utils.AppError
// @Security ApiKeyAuth
// @Router /api/admin/delivery-zones [post]
func (h *AdminDeliveryZoneHandler) CreateZone(w http.ResponseWriter, r *http.Request) error {
	var req deliveryzonedto.CreateZoneRequest

	// Correction 1 : Utiliser json.NewDecoder (méthode standard de ton projet)
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		return utils.NewAppError("INVALID_PAYLOAD", "Payload invalide", http.StatusBadRequest)
	}

	// Correction 2 : GetUserID retourne (string, bool), il faut vérifier le booléen
	adminID, ok := utils.GetUserID(r.Context())
	if !ok {
		return utils.NewAppError("UNAUTHORIZED", "Utilisateur non authentifié", http.StatusUnauthorized)
	}

	zone, err := h.uc.CreateZone(r.Context(), req, adminID)
	if err != nil {
		return utils.NewAppError("CREATE_FAILED", err.Error(), http.StatusInternalServerError)
	}

	// Correction 3 : WriteJSON ne retourne rien, on l'appelle puis on retourne nil
	utils.WriteJSON(w, http.StatusCreated, zone)
	return nil
}

// ListZones godoc
// @Summary Lister toutes les zones de livraison actives
// @Tags Admin Delivery Zones
// @Produce json
// @Success 200 {array} delivery_zone_dto.ZoneResponse
// @Failure 500 {object} utils.AppError
// @Security ApiKeyAuth
// @Router /api/admin/delivery-zones [get]
func (h *AdminDeliveryZoneHandler) ListZones(w http.ResponseWriter, r *http.Request) error {
	zones, err := h.uc.ListZones(r.Context())
	if err != nil {
		return utils.NewAppError("FETCH_FAILED", err.Error(), http.StatusInternalServerError)
	}

	utils.WriteJSON(w, http.StatusOK, zones)
	return nil
}

// UpdateZone godoc
// @Summary Mettre à jour une zone de livraison
// @Tags Admin Delivery Zones
// @Accept json
// @Produce json
// @Param id path string true "ID de la zone"
// @Param request body delivery_zone_dto.UpdateZoneRequest true "Détails de la mise à jour"
// @Success 200 {object} delivery_zone_dto.ZoneResponse
// @Failure 400 {object} utils.AppError
// @Failure 500 {object} utils.AppError
// @Security ApiKeyAuth
// @Router /api/admin/delivery-zones/{id} [put]
func (h *AdminDeliveryZoneHandler) UpdateZone(w http.ResponseWriter, r *http.Request) error {
	// Pour chi v5.x : r.PathValue("id"). Pour chi v4.x : chi.URLParam(r, "id")
	id := r.PathValue("id")

	var req deliveryzonedto.UpdateZoneRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		return utils.NewAppError("INVALID_PAYLOAD", "Payload invalide", http.StatusBadRequest)
	}

	adminID, ok := utils.GetUserID(r.Context())
	if !ok {
		return utils.NewAppError("UNAUTHORIZED", "Utilisateur non authentifié", http.StatusUnauthorized)
	}

	zone, err := h.uc.UpdateZone(r.Context(), id, req, adminID)
	if err != nil {
		return utils.NewAppError("UPDATE_FAILED", err.Error(), http.StatusInternalServerError)
	}

	utils.WriteJSON(w, http.StatusOK, zone)
	return nil
}

// DeleteZone godoc
// @Summary Supprimer une zone de livraison
// @Tags Admin Delivery Zones
// @Produce json
// @Param id path string true "ID de la zone"
// @Success 200 {object} map[string]string
// @Failure 400 {object} utils.AppError
// @Failure 500 {object} utils.AppError
// @Security ApiKeyAuth
// @Router /api/admin/delivery-zones/{id} [delete]
func (h *AdminDeliveryZoneHandler) DeleteZone(w http.ResponseWriter, r *http.Request) error {
	id := r.PathValue("id")

	if err := h.uc.DeleteZone(r.Context(), id); err != nil {
		return utils.NewAppError("DELETE_FAILED", err.Error(), http.StatusInternalServerError)
	}

	utils.WriteJSON(w, http.StatusOK, map[string]string{"message": "Zone supprimée avec succès"})
	return nil
}
