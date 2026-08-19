// application/metrics/registry.go
package metrics

import (
	"sync"

	"github.com/prometheus/client_golang/prometheus"
)

var registerOnce sync.Once

// RegisterMetrics enregistre toutes les métriques (UNE SEULE FOIS)
// 99 métriques au total
func RegisterMetrics() {
	registerOnce.Do(func() {
		// ========== Authentification (6) ==========
		prometheus.MustRegister(AuthLoginTotal)
		prometheus.MustRegister(AuthLoginFailedTotal)
		prometheus.MustRegister(AuthRegisterTotal)
		prometheus.MustRegister(AuthLoginDuration)
		prometheus.MustRegister(AuthRegisterDuration)
		prometheus.MustRegister(AuthProfileDuration)

		// ========== Produits (11) ==========
		prometheus.MustRegister(ProductsCreatedTotal)
		prometheus.MustRegister(ProductsUpdatedTotal)
		prometheus.MustRegister(ProductsDeletedTotal)
		prometheus.MustRegister(ProductsCreateDuration)
		prometheus.MustRegister(ProductsGetDuration)
		prometheus.MustRegister(ProductsListDuration)
		prometheus.MustRegister(ProductsOperationDuration)
		prometheus.MustRegister(ProductsOperationErrors)
		prometheus.MustRegister(ProductsListedCount)
		prometheus.MustRegister(PublicProductsListedTotal)
		prometheus.MustRegister(PublicProductsListedCount)

		// ========== Commandes (5) ==========
		prometheus.MustRegister(OrdersCreatedTotal)
		prometheus.MustRegister(OrdersRevenueCentsTotal)
		prometheus.MustRegister(OrdersCreateDuration)
		prometheus.MustRegister(OrdersGetDuration)
		prometheus.MustRegister(OrdersListDuration)

		// ========== COD (3) ==========
		prometheus.MustRegister(CODOperationTotal)
		prometheus.MustRegister(CODOperationDuration)
		prometheus.MustRegister(CODDeliveryAmountCents)

		// ========== Paiements (4) ==========
		prometheus.MustRegister(PaymentSuccessTotal)
		prometheus.MustRegister(PaymentFailedTotal)
		prometheus.MustRegister(PaymentDuration)
		prometheus.MustRegister(PaymentAmountCents)

		// ========== Tontine (12) ==========
		prometheus.MustRegister(TontinePaymentSuccessTotal)
		prometheus.MustRegister(TontinePaymentFailedTotal)
		prometheus.MustRegister(TontineCycleCompletedTotal)
		prometheus.MustRegister(TontineGroupCreatedTotal)
		prometheus.MustRegister(TontineVoucherRedeemedTotal)
		prometheus.MustRegister(TontineGroupJoinTotal)
		prometheus.MustRegister(TontinePaymentInitiatedTotal)
		prometheus.MustRegister(TontinePaymentDuration)
		prometheus.MustRegister(TontineSyncPaymentTotal)
		prometheus.MustRegister(TontineVoucherListedTotal)
		prometheus.MustRegister(TontineVoucherRedeemFailedTotal)
		prometheus.MustRegister(TontineOperationDuration)

		// ========== Tontine Settings (2) ==========
		prometheus.MustRegister(TontineSettingsOperationTotal)
		prometheus.MustRegister(TontineSettingsDuration)

		// ========== Webhooks (3) ==========
		prometheus.MustRegister(WebhookReceivedTotal)
		prometheus.MustRegister(WebhookProcessedTotal)
		prometheus.MustRegister(WebhookProcessingDuration)

		// ========== Wallet (3) ==========
		prometheus.MustRegister(WalletCreditTotal)
		prometheus.MustRegister(WalletDebitTotal)
		prometheus.MustRegister(WalletAmountCents)

		// ========== Erreurs & DB (3) ==========
		prometheus.MustRegister(ApplicationErrorsTotal)
		prometheus.MustRegister(DatabaseQueryDuration)
		prometheus.MustRegister(DatabaseConnectionPoolActive)

		// ========== Sessions (5) ==========
		prometheus.MustRegister(SessionOperationTotal)
		prometheus.MustRegister(SessionOperationDuration)
		prometheus.MustRegister(SessionRevokedTotal)
		prometheus.MustRegister(SessionsCleanedTotal)
		prometheus.MustRegister(SessionsListedCount)

		// ========== Shops (6) ==========
		prometheus.MustRegister(ShopCreatedTotal)
		prometheus.MustRegister(ShopCreationFailedTotal)
		prometheus.MustRegister(ShopUpdatedTotal)
		prometheus.MustRegister(ShopListedTotal)
		prometheus.MustRegister(ShopOperationDuration)
		prometheus.MustRegister(ShopListedCount)

		// ========== Merchant KYC (7) ==========
		prometheus.MustRegister(MerchantKYCSubmitTotal)
		prometheus.MustRegister(MerchantKYCReviewTotal)
		prometheus.MustRegister(MerchantKYCStatusCheckTotal)
		prometheus.MustRegister(MerchantKYCPendingListTotal)
		prometheus.MustRegister(MerchantKYCOperationDuration)
		prometheus.MustRegister(MerchantKYCDocumentsPerSubmit)
		prometheus.MustRegister(MerchantKYCPendingCount)

		// ========== Merchant Overview (2) ==========
		prometheus.MustRegister(MerchantOverviewRequestTotal)
		prometheus.MustRegister(MerchantOverviewDuration)

		// ========== Disputes (7) ==========
		prometheus.MustRegister(DisputeOpenTotal)
		prometheus.MustRegister(DisputeResolveTotal)
		prometheus.MustRegister(DisputeListTotal)
		prometheus.MustRegister(DisputeGetTotal)
		prometheus.MustRegister(DisputeOperationDuration)
		prometheus.MustRegister(DisputeConflictTotal)
		prometheus.MustRegister(DisputeListedCount)

		// ========== Delivery Proof (4) ==========
		prometheus.MustRegister(DeliveryProofSubmitTotal)
		prometheus.MustRegister(DeliveryProofDuration)
		prometheus.MustRegister(DeliveryProofTenantErrors)
		prometheus.MustRegister(DeliveryProofPayloadErrors)

		// ========== Customer (6) ==========
		prometheus.MustRegister(CustomerCreatedTotal)
		prometheus.MustRegister(CustomerUpdatedTotal)
		prometheus.MustRegister(CustomerDeletedTotal)
		prometheus.MustRegister(CustomerOperationDuration)
		prometheus.MustRegister(CustomerOperationErrors)
		prometheus.MustRegister(CustomerListedCount)

		// ========== Customer KYC (5) ==========
		prometheus.MustRegister(CustomerKYCUploadTotal)
		prometheus.MustRegister(CustomerKYCStatusCheckTotal)
		prometheus.MustRegister(CustomerKYCReviewTotal)
		prometheus.MustRegister(CustomerKYCPendingListTotal)
		prometheus.MustRegister(CustomerKYCOperationDuration)

		// ========== Customer Dashboard (2) ==========
		prometheus.MustRegister(CustomerDashboardRequestTotal)
		prometheus.MustRegister(CustomerDashboardDuration)

		// ========== HTTP (4) ==========
		prometheus.MustRegister(HTTPRequestDuration)
		prometheus.MustRegister(HTTPRequestsTotal)
		prometheus.MustRegister(HTTPRequestSizeBytes)
		prometheus.MustRegister(HTTPResponseSizeBytes)
	})

	// ========== Commission Rates (6) ==========
	prometheus.MustRegister(CommissionRateUpdateTotal)
	prometheus.MustRegister(CommissionRateGetTotal)
	prometheus.MustRegister(CommissionRateOperationDuration)
	prometheus.MustRegister(CommissionRateConfigured)
	prometheus.MustRegister(CommissionTriggerTotal)
	prometheus.MustRegister(CommissionRateValueBps)

	// ========== Collaborator (8) ==========
	prometheus.MustRegister(CollaboratorInvitationTotal)
	prometheus.MustRegister(CollaboratorListTotal)
	prometheus.MustRegister(CollaboratorRoleUpdateTotal)
	prometheus.MustRegister(CollaboratorRemovalTotal)
	prometheus.MustRegister(CollaboratorInvitationAcceptTotal)
	prometheus.MustRegister(CollaboratorInvitationPreviewTotal)
	prometheus.MustRegister(CollaboratorOperationDuration)
	prometheus.MustRegister(CollaboratorActiveCount)

	// ========== COD Proofs & Commissions (11) ==========
	prometheus.MustRegister(CODProofSubmitTotal)
	prometheus.MustRegister(CODProofGetTotal)
	prometheus.MustRegister(CODProofListTotal)
	prometheus.MustRegister(CODCommissionCollectTotal)
	prometheus.MustRegister(CODCommissionRetryTotal)
	prometheus.MustRegister(CODCommissionStatsTotal)
	prometheus.MustRegister(CODDueCommissionsTotal)
	prometheus.MustRegister(CODProofOperationDuration)
	prometheus.MustRegister(CODCommissionAmountCents)
	prometheus.MustRegister(CODProofTenantErrors)
	prometheus.MustRegister(CODProofPayloadErrors)

	// ========== API Keys (6) ==========
	prometheus.MustRegister(APIKeyCreateTotal)
	prometheus.MustRegister(APIKeyListTotal)
	prometheus.MustRegister(APIKeyRevokeTotal)
	prometheus.MustRegister(APIKeyStatsTotal)
	prometheus.MustRegister(APIKeyOperationDuration)
	prometheus.MustRegister(APIKeyScopesCount)

	// ========== Admin Shop (6) ==========
	prometheus.MustRegister(AdminShopOperationTotal)
	prometheus.MustRegister(AdminShopOperationDuration)
	prometheus.MustRegister(AdminShopSuspendTotal)
	prometheus.MustRegister(AdminShopActivateTotal)
	prometheus.MustRegister(AdminShopListedCount)
	prometheus.MustRegister(AdminShopHealthChecksTotal)

	// ========== WebSocket (6) ==========
	prometheus.MustRegister(WebSocketConnectionsTotal)
	prometheus.MustRegister(WebSocketConnectionsActive)
	prometheus.MustRegister(WebSocketConnectionDuration)
	prometheus.MustRegister(WebSocketMessagesReceived)
	prometheus.MustRegister(WebSocketUpgradeErrors)
	prometheus.MustRegister(WebSocketUnexpectedCloses)

	// ========== Health Checks (5) ==========
	prometheus.MustRegister(HealthCheckTotal)
	prometheus.MustRegister(HealthCheckDuration)
	prometheus.MustRegister(HealthDependenciesStatus)
	prometheus.MustRegister(HealthDatabasePingDuration)
	prometheus.MustRegister(HealthRedisPingDuration)

	// ========== Withdrawals (7) ==========
	prometheus.MustRegister(WithdrawalCreateTotal)
	prometheus.MustRegister(WithdrawalListTotal)
	prometheus.MustRegister(WithdrawalGetTotal)
	prometheus.MustRegister(WithdrawalOperationDuration)
	prometheus.MustRegister(WithdrawalAmountCents)
	prometheus.MustRegister(WithdrawalHeldCentsRejections)
	prometheus.MustRegister(WithdrawalListedCount)

}
