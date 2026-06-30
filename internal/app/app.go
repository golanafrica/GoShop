// internal/app/app.go
package app

import (
	"database/sql"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/rs/zerolog"

	"Goshop/application/metrics"
	authusecase "Goshop/application/usecase/auth_usecase"
	customerusecase "Goshop/application/usecase/customer_usecase"
	orderusecase "Goshop/application/usecase/order_usecase"
	paymentusecase "Goshop/application/usecase/payment_usecase"
	shopusecase "Goshop/application/usecase/shop_usecase"
	tontineusecase "Goshop/application/usecase/tontine_usecase"
	withdrawalusecase "Goshop/application/usecase/withdrawal_usecase"

	// 🆕 v3.0.0 : Usecases
	codusecase "Goshop/application/usecase/cod_usecase"
	creditusecase "Goshop/application/usecase/credit_usecase"
	walletusecase "Goshop/application/usecase/wallet_usecase"

	paymentinfra "Goshop/infrastructure/payment"
	"Goshop/infrastructure/payment/mock"
	paymentpostgres "Goshop/infrastructure/postgres/payment"

	authrefreshrepositoryinfra "Goshop/infrastructure/postgres/auth_refresh_repository_infra"
	"Goshop/infrastructure/postgres/customer"
	"Goshop/infrastructure/postgres/order"
	"Goshop/infrastructure/postgres/product"
	"Goshop/infrastructure/postgres/shop"
	"Goshop/infrastructure/postgres/tontine"
	txmanager "Goshop/infrastructure/postgres/tx_manager"
	userpostgres "Goshop/infrastructure/postgres/user_postgres"
	withdrawalpostgres "Goshop/infrastructure/withdrawal"

	// 🆕 v3.0.0 : Repositories PostgreSQL
	codinfra "Goshop/infrastructure/postgres/cod"
	creditinfra "Goshop/infrastructure/postgres/credit"
	escrowinfra "Goshop/infrastructure/postgres/escrow"
	freezeinfra "Goshop/infrastructure/postgres/freeze"
	walletinfra "Goshop/infrastructure/postgres/wallet"

	"Goshop/domain/service"
	"Goshop/infrastructure/notification"

	"github.com/prometheus/client_golang/prometheus/promhttp"

	handlers "Goshop/interfaces/handler"
	customerhandler "Goshop/interfaces/handler/customer_handler"
	ordershandler "Goshop/interfaces/handler/orders"
	paymenthandler "Goshop/interfaces/handler/payment_handler"
	productHandler "Goshop/interfaces/handler/product"
	refreshhandler "Goshop/interfaces/handler/refresh_handler"
	shophandler "Goshop/interfaces/handler/shop_handler"
	tontinehandler "Goshop/interfaces/handler/tontine_handler"
	userhandler "Goshop/interfaces/handler/user_handler"
	withdrawalhandler "Goshop/interfaces/handler/withdrawal_handler"
	middleware "Goshop/interfaces/middl/user_middleware"

	// 🆕 v3.0.0 : Handlers
	codhandler "Goshop/interfaces/handler/cod_handler"
	credithandler "Goshop/interfaces/handler/credit_handler"
	wallethandler "Goshop/interfaces/handler/wallet_handler"

	"Goshop/config/setupLogging"
	"Goshop/interfaces/middl"
	"Goshop/interfaces/utils"

	httpSwagger "github.com/swaggo/http-swagger"
)

// ============ STRUCT App ============

// App représente l'application configurable
type App struct {
	Router *chi.Mux
	DB     *sql.DB
	Logger *setupLogging.Logger
}

// NewApp crée une nouvelle instance de l'application avec logging
func NewApp(db *sql.DB, logger *setupLogging.Logger) *App {
	metrics.RegisterMetrics()
	app := &App{
		DB:     db,
		Logger: logger.WithComponent("app"),
	}
	app.setupRouter()
	return app
}

// setupRouter configure toutes les routes avec logging
func (a *App) setupRouter() {

	startTime := time.Now()
	r := chi.NewRouter()

	// ============ 1. MIDDLEWARES GLOBAUX ============

	r.Use(middl.LoggerInitMiddleware(a.Logger))
	r.Use(middl.RequestIDMiddleware)
	r.Use(middl.HTTPMetricsMiddleware)
	r.Use(middl.LoginAuditMiddleware)
	r.Use(middl.RequestLoggerMiddleware)
	r.Use(middl.Recovery)
	r.Use(middl.SecureHeaders)

	// ============ 2. INITIALISATION ============
	hh := handlers.HealthHandler{
		DB:     a.DB,
		Rdb:    utils.Rdb,
		Logger: a.Logger.WithComponent("health_handler"),
	}

	// -- Repositories (existants)
	txmanagerRepo := txmanager.NewTxManagerPostgresInfra(a.DB)
	postgreProductRepo := product.NewProductRepositoryInfrastructure(a.DB)
	postgresCustomerRepo := customer.NewCustomerRepoInfrastructurePostgres(a.DB)
	postgresOrderRepo := order.NewOrderPostgresInfra(a.DB)
	postgresOrderItem := order.NewOrderItemPostgresInfra(a.DB)
	postgresUserRepo := userpostgres.NewUserPostgres(a.DB)
	refreshSessionRepo := authrefreshrepositoryinfra.NewRefreshSessionPostgres(a.DB)
	shopRepo := shop.NewShopRepositoryInfrastructure(a.DB)

	paymentRepo := paymentpostgres.NewPaymentRepositoryPostgres(a.DB)
	withdrawalRepo := withdrawalpostgres.NewWithdrawalRepositoryPostgres(a.DB)
	paymentRegistry := paymentinfra.NewRegistry()

	// 🆕 v2.9.0 : Repositories Tontine
	tontineSettingsRepo := tontine.NewProductTontineSettingsRepositoryInfrastructure(a.DB)
	tontineGroupRepo := tontine.NewTontineGroupRepositoryInfrastructure(a.DB)
	tontineParticipantRepo := tontine.NewTontineParticipantRepositoryInfrastructure(a.DB)
	tontinePaymentRepo := tontine.NewTontinePaymentRepositoryInfrastructure(a.DB)
	tontineVoucherRepo := tontine.NewTontineVoucherRepositoryInfrastructure(a.DB)

	// 🆕 v2.9.0 : Repository KYC
	kycDocRepo := customer.NewCustomerKYCRepositoryInfrastructure(a.DB)

	// 🆕 v3.0.0 : Repositories Credit
	creditPlanRepo := creditinfra.NewCreditPlanRepositoryInfrastructure(a.DB)
	creditAppRepo := creditinfra.NewCreditApplicationRepositoryInfrastructure(a.DB)
	creditContractRepo := creditinfra.NewCreditContractRepositoryInfrastructure(a.DB)
	creditInstallmentRepo := creditinfra.NewCreditInstallmentRepositoryInfrastructure(a.DB)
	creditScoreRepo := creditinfra.NewCreditScoreRepositoryInfrastructure(a.DB)

	// 🆕 v3.0.0 : Repositories Escrow
	_ = escrowinfra.NewEscrowAccountRepositoryInfrastructure(a.DB)
	_ = escrowinfra.NewDeliveryProofRepositoryInfrastructure(a.DB)

	// 🆕 v3.0.0 : Repositories Wallet
	walletRepo := walletinfra.NewMerchantWalletRepositoryInfrastructure(a.DB)
	walletTxnRepo := walletinfra.NewWalletTransactionRepositoryInfrastructure(a.DB)

	// 🆕 v3.0.0 : Repository COD
	codProofRepo := codinfra.NewCODProofRepositoryInfrastructure(a.DB)

	// 🆕 v3.0.0 : Repository Freeze
	freezeRepo := freezeinfra.NewAccountFreezeRepositoryInfrastructure(a.DB)

	a.Logger.Info().Msg("✅ v3.0.0 repositories initialized (credit, escrow, wallet, cod, freeze)")

	// Mock Orange Money Provider
	orangeMoneyProvider := mock.NewOrangeMoneyProvider(mock.DefaultOrangeMoneyConfig())
	if err := paymentRegistry.Register(orangeMoneyProvider); err != nil {
		a.Logger.Error().Err(err).Msg("Failed to register Orange Money provider")
	}

	// Mock Moov Money Provider
	moovMoneyProvider := mock.NewMoovMoneyProvider(mock.DefaultMoovMoneyConfig())
	if err := paymentRegistry.Register(moovMoneyProvider); err != nil {
		a.Logger.Error().Err(err).Msg("Failed to register Moov Money provider")
	}

	// Yenga Pay Provider (API réelle) - config globale par défaut
	yengaPayConfig := paymentinfra.YengaPayConfig{
		APIKey:         os.Getenv("YENGA_PAY_API_KEY"),
		OrganizationID: os.Getenv("YENGA_PAY_ORGANIZATION_ID"),
		ProjectID:      os.Getenv("YENGA_PAY_PROJECT_ID"),
		WebhookSecret:  os.Getenv("YENGA_PAY_WEBHOOK_SECRET"),
		Env:            os.Getenv("YENGA_PAY_ENV"),
	}

	if yengaPayConfig.APIKey != "" {
		yengaPayProvider, err := paymentinfra.NewYengaPayProvider(yengaPayConfig)
		if err != nil {
			a.Logger.Error().Err(err).Msg("Failed to create Yenga Pay provider")
		} else {
			if err := paymentRegistry.Register(yengaPayProvider); err != nil {
				a.Logger.Error().Err(err).Msg("Failed to register Yenga Pay provider")
			} else {
				a.Logger.Info().Msg("✅ Yenga Pay provider registered (global config)")
			}
		}
	} else {
		a.Logger.Warn().Msg("⚠️ Yenga Pay provider not configured globally (missing YENGA_PAY_API_KEY)")
	}

	// ============ 🆕 NOTIFICATION SERVICE ============
	notifService := service.NotificationService(notification.NewNoopNotificationService(a.Logger.Logger))
	a.Logger.Info().Msg("✅ Notification service initialized (no-op mode)")

	// -- Usecases (existants)
	refreshUsecase := authusecase.NewRefreshUsecase(
		refreshSessionRepo,
		utils.ValidateToken,
		utils.GenerateAccessToken,
		utils.GenerateRefreshToken,
		time.Now,
		uuid.NewString,
		30*24*time.Hour,
	)

	// Shop Usecases
	createShopUsecase := shopusecase.NewCreateShopUsecase(shopRepo)
	listShopsUsecase := shopusecase.NewListShopsUsecase(shopRepo)
	updateShopUsecase := shopusecase.NewUpdateShopUsecase(shopRepo)

	// Configure Payment Usecase (pour config par boutique)
	configurePaymentUC := shopusecase.NewConfigurePaymentUsecase(shopRepo, shopRepo, shopRepo)

	// 🆕 v2.9.0 : Configure Tontine Usecase
	configureTontineUC := shopusecase.NewConfigureTontineUsecase(tontineSettingsRepo, postgreProductRepo)

	// Payment Usecases
	initiatePaymentUC := paymentusecase.NewInitiatePaymentUsecaseWithShopSettings(
		paymentRepo,
		postgresOrderRepo,
		paymentRegistry,
		shopRepo,
	)
	checkPaymentStatusUC := paymentusecase.NewCheckPaymentStatusUsecase(
		paymentRepo,
		postgresOrderRepo,
		paymentRegistry,
	)
	listPaymentsUC := paymentusecase.NewListPaymentsUsecase(paymentRepo)
	refundPaymentUC := paymentusecase.NewRefundPaymentUsecase(
		paymentRepo,
		paymentRegistry,
	)
	completePaymentUC := paymentusecase.NewCompletePaymentUsecase(paymentRepo, paymentRegistry)

	// 🆕 v2.9.0 : Process Tontine Webhook Usecase (doit être créé AVANT processWebhookUC)
	processTontineWebhookUC := paymentusecase.NewProcessTontineWebhookUsecase(
		tontinePaymentRepo,
		tontineGroupRepo,
		tontineParticipantRepo,
		tontineVoucherRepo,
		shopRepo,
	)

	// Process Webhook Usecase (modifié pour inclure tontine)
	processWebhookUC := paymentusecase.NewProcessWebhookUsecase(
		paymentRepo,
		paymentRegistry,
		a.DB,
		shopRepo,
		processTontineWebhookUC, // 🆕 v2.9.0
	)

	// Withdrawal Usecases
	createWithdrawalUC := withdrawalusecase.NewCreateWithdrawalUsecase(
		withdrawalRepo,
		shopRepo,
		paymentRegistry,
	)
	listWithdrawalsUC := withdrawalusecase.NewListWithdrawalsUsecase(withdrawalRepo)

	// ============ 🆕 CASH ORDER USECASES ============
	acceptOrderUC := orderusecase.NewAcceptOrderUsecase(
		postgresOrderRepo,
		postgreProductRepo,
		notifService,
		txmanagerRepo,
	)

	rejectOrderUC := orderusecase.NewRejectOrderUsecase(
		postgresOrderRepo,
		postgreProductRepo,
		notifService,
		txmanagerRepo,
	)

	outForDeliveryUC := orderusecase.NewOutForDeliveryUsecase(
		postgresOrderRepo,
		txmanagerRepo,
	)

	deliverOrderUC := orderusecase.NewDeliverOrderUsecase(
		postgresOrderRepo,
		paymentRepo,
		shopRepo,
		notifService,
		txmanagerRepo,
	)

	cancelOrderUC := orderusecase.NewCancelOrderUsecase(
		postgresOrderRepo,
		postgreProductRepo,
		notifService,
		txmanagerRepo,
	)

	a.Logger.Info().Msg("✅ Cash order usecases initialized (accept, reject, out_for_delivery, deliver, cancel)")

	// ============ 🆕 v2.9.0 : TONTINE USECASES ============
	createTontineGroupUC := tontineusecase.NewCreateTontineGroupUsecase(
		tontineGroupRepo,
		tontineParticipantRepo,
		tontineSettingsRepo,
		postgreProductRepo,
		postgresCustomerRepo,
	)

	joinTontineGroupUC := tontineusecase.NewJoinTontineGroupUsecase(
		tontineGroupRepo,
		tontineParticipantRepo,
		postgresCustomerRepo,
	)

	payCycleUC := tontineusecase.NewPayCycleUsecase(
		tontineGroupRepo,
		tontineParticipantRepo,
		tontinePaymentRepo,
		shopRepo,
		txmanagerRepo,
	)

	listCustomerPaymentsUC := tontineusecase.NewListCustomerPaymentsUsecase(
		tontinePaymentRepo,
		tontineGroupRepo,
	)

	a.Logger.Info().Msg("✅ Tontine usecases initialized (create_group, join_group, pay_cycle, list_payments)")

	// ============ 🆕 v2.9.0 : KYC USECASES ============
	uploadKYCUC := customerusecase.NewUploadKYCDocumentUsecase(
		kycDocRepo,
		postgresCustomerRepo,
		txmanagerRepo,
	)

	getKYCStatusUC := customerusecase.NewGetKYCStatusUsecase(
		postgresCustomerRepo,
		kycDocRepo,
	)

	reviewKYCUC := customerusecase.NewReviewKYCUsecase(
		postgresCustomerRepo,
		kycDocRepo,
		notifService,
		txmanagerRepo,
	)

	listPendingKYCUC := customerusecase.NewListPendingKYCUsecase(
		kycDocRepo,
		postgresCustomerRepo,
	)

	a.Logger.Info().Msg("✅ KYC usecases initialized (upload, get_status, review, list_pending)")

	// ============ 🆕 v3.0.0 : WALLET USECASES ============
	creditWalletUC := walletusecase.NewCreditWalletUsecase(walletRepo, walletTxnRepo, txmanagerRepo)
	debitWalletUC := walletusecase.NewDebitWalletUsecase(walletRepo, walletTxnRepo, txmanagerRepo)
	freezeAccountUC := walletusecase.NewFreezeAccountUsecase(walletRepo, freezeRepo, txmanagerRepo)
	unfreezeAccountUC := walletusecase.NewUnfreezeAccountUsecase(walletRepo, freezeRepo, walletTxnRepo, txmanagerRepo)

	a.Logger.Info().Msg("✅ v3.0.0 Wallet usecases initialized (credit, debit, freeze, unfreeze)")

	// ============ 🆕 v3.0.0 : COD USECASES ============
	submitClientProofUC := codusecase.NewSubmitClientProofUsecase(codProofRepo, postgresOrderRepo, txmanagerRepo)
	submitMerchantProofUC := codusecase.NewSubmitMerchantProofUsecase(codProofRepo, postgresOrderRepo, txmanagerRepo)
	collectCommissionUC := codusecase.NewCollectCommissionUsecase(
		codProofRepo,
		postgresOrderRepo,
		debitWalletUC,
		freezeAccountUC,
		txmanagerRepo,
	)

	a.Logger.Info().Msg("✅ v3.0.0 COD usecases initialized (submit_client, submit_merchant, collect_commission)")

	// ============ 🆕 v3.0.0 : CREDIT USECASES ============
	configureCreditPlanUC := creditusecase.NewConfigureCreditPlanUsecase(
		creditPlanRepo,
		postgreProductRepo,
		walletRepo,
		txmanagerRepo,
	)
	applyForCreditUC := creditusecase.NewApplyForCreditUsecase(
		creditAppRepo,
		creditPlanRepo,
		postgreProductRepo,
		creditScoreRepo,
		txmanagerRepo,
	)
	approveCreditUC := creditusecase.NewApproveCreditUsecase(
		creditAppRepo,
		creditContractRepo,
		creditInstallmentRepo,
		creditScoreRepo,
		txmanagerRepo,
	)
	rejectCreditUC := creditusecase.NewRejectCreditUsecase(
		creditAppRepo,
		creditScoreRepo,
		txmanagerRepo,
	)
	payDownPaymentUC := creditusecase.NewPayDownPaymentUsecase(
		creditContractRepo,
		creditInstallmentRepo,
		creditScoreRepo,
		txmanagerRepo,
	)

	a.Logger.Info().Msg("✅ v3.0.0 Credit usecases initialized (configure, apply, approve, reject, pay_down)")

	// -- Handlers (existants)
	refreshHandler := refreshhandler.NewRefreshHandler(refreshUsecase)

	productHandler := productHandler.NewProductHandler(
		postgreProductRepo,
		txmanagerRepo,
	)

	customerHandler := customerhandler.NewCustomerHandler(
		postgresCustomerRepo,
		txmanagerRepo,
	)

	orderHandler := ordershandler.NewOrderHandler(
		a.DB,
		txmanagerRepo,
		postgresOrderRepo,
		postgreProductRepo,
		postgresCustomerRepo,
		postgresOrderItem,
	)

	// Cash Order Handler
	cashOrderHandler := ordershandler.NewCashOrderHandler(
		acceptOrderUC,
		rejectOrderUC,
		outForDeliveryUC,
		deliverOrderUC,
		cancelOrderUC,
	)

	userHandler := userhandler.NewUserHandler(
		postgresUserRepo,
		a.Logger.WithComponent("user_handler"),
	)

	shopHandler := shophandler.NewShopHandler(
		createShopUsecase,
		listShopsUsecase,
		updateShopUsecase,
	)

	// Payment Settings Handler
	paymentSettingsHandler := shophandler.NewPaymentSettingsHandler(configurePaymentUC)

	// 🆕 v2.9.0 : Tontine Settings Handler (handler dédié)
	tontineSettingsHandler := shophandler.NewTontineSettingsHandler(configureTontineUC, shopRepo)

	// Payment Handlers
	paymentHandler := paymenthandler.NewPaymentHandler(
		initiatePaymentUC,
		checkPaymentStatusUC,
		listPaymentsUC,
		refundPaymentUC,
		completePaymentUC,
	)
	webhookHandler := paymenthandler.NewWebhookHandler(processWebhookUC)

	// Withdrawal Handler
	withdrawalHandler := withdrawalhandler.NewWithdrawalHandler(
		createWithdrawalUC,
		listWithdrawalsUC,
	)

	// 🆕 v2.9.0 : Tontine Handler
	tontineHandler := tontinehandler.NewTontineHandler(
		createTontineGroupUC,
		joinTontineGroupUC,
		payCycleUC,
		listCustomerPaymentsUC,
	)

	// 🆕 v2.9.0 : KYC Handler
	kycHandler := customerhandler.NewKYCHandler(
		uploadKYCUC,
		getKYCStatusUC,
		reviewKYCUC,
		listPendingKYCUC,
	)

	a.Logger.Info().Msg("✅ Tontine and KYC handlers initialized")

	// ============ 🆕 v3.0.0 : HANDLERS ============
	walletHandler := wallethandler.NewWalletHandler(
		creditWalletUC,
		debitWalletUC,
		freezeAccountUC,
		unfreezeAccountUC,
	)

	codHandler := codhandler.NewCODHandler(
		submitClientProofUC,
		submitMerchantProofUC,
		collectCommissionUC,
		codProofRepo,
	)

	creditHandler := credithandler.NewCreditHandler(
		configureCreditPlanUC,
		applyForCreditUC,
		approveCreditUC,
		rejectCreditUC,
		payDownPaymentUC,
	)

	a.Logger.Info().Msg("✅ v3.0.0 handlers initialized (wallet, cod, credit)")

	// ============ 3. ROUTES PUBLIQUES ============
	r.Use(middl.PrometheusMiddleware)

	r.Get("/health/live", hh.Live)
	r.Get("/health/ready", hh.Ready)

	r.Post("/auth/refresh", middl.ErrorHandler(refreshHandler.Refresh))
	r.Post("/register", middl.ErrorHandler(userHandler.Register))
	r.Post("/login", middl.ErrorHandler(userHandler.Login))

	r.Get("/help", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("Goshop API est en ligne !"))
	})

	r.Handle("/metrics", promhttp.Handler())
	r.Get("/swagger/*", httpSwagger.Handler())

	// Webhooks (public, pas d'auth requise)
	r.Post("/webhooks/{provider}", middl.ErrorHandler(webhookHandler.HandleWebhook))

	// ============ 4. ROUTE PROTÉGÉE (user authentifié) ============
	r.With(middleware.AuthMiddleware).
		Get("/auth/me", middl.ErrorHandler(userHandler.Me))

	// ============ 5. ROUTES API PROTÉGÉES + MULTI-TENANT ============
	r.Route("/api", func(r chi.Router) {
		r.Use(middleware.AuthMiddleware)

		// Routes de gestion des shops (SANS TenantResolver)
		r.Route("/shops", func(r chi.Router) {
			r.Post("/", middl.ErrorHandler(shopHandler.CreateShop))
			r.Get("/", middl.ErrorHandler(shopHandler.ListShops))
			r.Put("/{id}", middl.ErrorHandler(shopHandler.UpdateShop))

			// Routes de configuration des paiements par boutique
			r.Get("/{id}/payment-settings", middl.ErrorHandler(paymentSettingsHandler.GetPaymentSettings))
			r.Put("/{id}/payment-settings", middl.ErrorHandler(paymentSettingsHandler.UpdatePaymentSettings))

			// 🆕 v2.9.0 : Routes de configuration tontine par boutique
			r.Get("/{id}/tontine-settings", middl.ErrorHandler(tontineSettingsHandler.GetTontineSettings))
			r.Put("/{id}/tontine-settings", middl.ErrorHandler(tontineSettingsHandler.UpdateTontineSettings))
		})

		// Routes multi-tenant (AVEC TenantResolver)
		r.Group(func(r chi.Router) {
			r.Use(middl.TenantResolver(shopRepo, a.Logger.Logger))

			// Products
			r.Route("/products", func(r chi.Router) {
				r.Post("/", middl.ErrorHandler(productHandler.CreateProduct))
				r.Get("/", middl.ErrorHandler(productHandler.GetAllProducts))
				r.Get("/{id}", middl.ErrorHandler(productHandler.GetProductById))
				r.Put("/{id}", middl.ErrorHandler(productHandler.UpdateProduct))
				r.Delete("/{id}", middl.ErrorHandler(productHandler.DeleteProduct))
			})

			// Customers (existants + 🆕 KYC)
			r.Route("/customers", func(r chi.Router) {
				r.Post("/", middl.ErrorHandler(customerHandler.CreateCustomerHandler))
				r.Get("/", middl.ErrorHandler(customerHandler.GetAllCustomersHandler))
				r.Get("/{id}", middl.ErrorHandler(customerHandler.GetCustomerByIdHandler))
				r.Put("/{id}", middl.ErrorHandler(customerHandler.UpdateCustomerHandler))
				r.Delete("/{id}", middl.ErrorHandler(customerHandler.DeleteCustomerHandler))

				// 🆕 v2.9.0 : KYC routes (côté client)
				r.Post("/kyc/upload", middl.ErrorHandler(kycHandler.UploadKYC))
				r.Get("/{customer_id}/kyc/status", middl.ErrorHandler(kycHandler.GetKYCStatus))
			})

			// Orders (existant + cash workflow)
			r.Route("/orders", func(r chi.Router) {
				r.Get("/", middl.ErrorHandler(orderHandler.GetAllOrderHandler))
				r.Post("/", middl.ErrorHandler(orderHandler.CreateOrderHandler))
				r.Get("/{id}", middl.ErrorHandler(orderHandler.GetOrderByIdHandler))
				r.Post("/{id}/pay", middl.ErrorHandler(paymentHandler.InitiatePayment))
				r.Post("/{id}/accept", middl.ErrorHandler(cashOrderHandler.AcceptOrder))
				r.Post("/{id}/reject", middl.ErrorHandler(cashOrderHandler.RejectOrder))
				r.Post("/{id}/out-for-delivery", middl.ErrorHandler(cashOrderHandler.OutForDelivery))
				r.Post("/{id}/deliver", middl.ErrorHandler(cashOrderHandler.DeliverOrder))
				r.Post("/{id}/cancel", middl.ErrorHandler(cashOrderHandler.CancelOrder))
			})

			// Payments
			r.Route("/payments", func(r chi.Router) {
				r.Get("/", middl.ErrorHandler(paymentHandler.ListPayments))
				r.Get("/{id}", middl.ErrorHandler(paymentHandler.GetPayment))
				r.Post("/{id}/refund", middl.ErrorHandler(paymentHandler.RefundPayment))
				r.Post("/{id}/complete", middl.ErrorHandler(paymentHandler.CompletePayment))
			})

			// Withdrawals (cash-out)
			r.Route("/withdrawals", func(r chi.Router) {
				r.Post("/", middl.ErrorHandler(withdrawalHandler.CreateWithdrawal))
				r.Get("/", middl.ErrorHandler(withdrawalHandler.ListWithdrawals))
				r.Get("/{id}", middl.ErrorHandler(withdrawalHandler.GetWithdrawal))
			})

			// 🆕 v2.9.0 : Tontine routes (côté client)
			r.Route("/tontine", func(r chi.Router) {
				r.Post("/groups", middl.ErrorHandler(tontineHandler.CreateGroup))
				r.Post("/groups/join", middl.ErrorHandler(tontineHandler.JoinGroup))
				r.Post("/groups/{group_id}/pay", middl.ErrorHandler(tontineHandler.PayCycle))
				r.Get("/groups/{group_id}/payments", middl.ErrorHandler(tontineHandler.ListCustomerPayments))
			})

			// 🆕 v2.9.0 : Merchant KYC routes (côté marchand)
			r.Route("/merchant/kyc", func(r chi.Router) {
				r.Get("/pending", middl.ErrorHandler(kycHandler.ListPendingKYC))
				r.Post("/{customer_id}/review", middl.ErrorHandler(kycHandler.ReviewKYC))
			})

			// ============ 🆕 v3.0.0 : WALLET ROUTES ============
			// Les handlers v3.0.0 gèrent leurs propres erreurs via utils.WriteError
			// Donc on les appelle directement SANS middl.ErrorHandler
			r.Route("/wallet", func(r chi.Router) {
				walletHandler.RegisterRoutes(r)
			})

			// ============ 🆕 v3.0.0 : COD ROUTES ============
			r.Route("/cod", func(r chi.Router) {
				codHandler.RegisterRoutes(r)
			})

			// ============ 🆕 v3.0.0 : CREDIT ROUTES ============
			r.Route("/credit", func(r chi.Router) {
				creditHandler.RegisterRoutes(r)
			})
		})
	})

	a.Router = r

	duration := time.Since(startTime)
	a.Logger.Info().
		Dur("setup_duration_ms", duration).
		Msg("✅ Router configuré avec succès (v3.0.0: + wallet + cod + credit)")
}

// ============ MIDDLEWARES PERSONNALISÉS ============

func maskEmail(email string) string {
	parts := strings.Split(email, "@")
	if len(parts) != 2 {
		return "***@***"
	}
	localPart := parts[0]
	domain := parts[1]
	if len(localPart) <= 2 {
		return localPart[:1] + "***@" + domain
	} else if len(localPart) <= 4 {
		return localPart[:2] + "***@" + domain
	}
	return localPart[:3] + "***@" + domain
}

func determineLogLevel(status int, duration time.Duration) zerolog.Level {
	switch {
	case status >= 500:
		return zerolog.ErrorLevel
	case status >= 400:
		return zerolog.WarnLevel
	case duration > 1*time.Second:
		return zerolog.WarnLevel
	default:
		return zerolog.InfoLevel
	}
}

func slowRequestWarning(duration time.Duration) string {
	if duration > 2*time.Second {
		return "slow_request"
	}
	return ""
}

type responseWriter struct {
	http.ResponseWriter
	statusCode int
	bodySize   int
}

func (rw *responseWriter) WriteHeader(code int) {
	rw.statusCode = code
	rw.ResponseWriter.WriteHeader(code)
}

func (rw *responseWriter) Write(b []byte) (int, error) {
	n, err := rw.ResponseWriter.Write(b)
	rw.bodySize += n
	return n, err
}

// Handler retourne le handler HTTP
func (a *App) Handler() http.Handler {
	return a.Router
}

// NewRouter crée et retourne un router HTTP configuré (pour les tests)
func NewRouter(db *sql.DB) http.Handler {
	loggingConfig := setupLogging.Config{
		Environment: "test",
		ServiceName: "goshop-api-test",
		Version:     "3.0.0",
		LogLevel:    "warn",
	}
	logger := setupLogging.NewLogger(loggingConfig)
	app := NewApp(db, logger)
	return app.Router
}
