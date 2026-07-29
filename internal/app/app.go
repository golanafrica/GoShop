package app

import (
	"context"
	"database/sql"
	"log" // 🆕 AJOUTÉ : Nécessaire pour le log.Fatal en cas de mauvaise config CORS
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/rs/zerolog"

	"Goshop/application/metrics"
	authusecase "Goshop/application/usecase/auth_usecase"
	collaboratorusecase "Goshop/application/usecase/collaborator_usecase"
	customerusecase "Goshop/application/usecase/customer_usecase"
	orderusecase "Goshop/application/usecase/order_usecase"
	paymentusecase "Goshop/application/usecase/payment_usecase"
	productuscase "Goshop/application/usecase/product_uscase" // 🆕 AJOUT v3.6.0
	shopusecase "Goshop/application/usecase/shop_usecase"
	tontineusecase "Goshop/application/usecase/tontine_usecase"
	walletusecase "Goshop/application/usecase/wallet_usecase"
	withdrawalusecase "Goshop/application/usecase/withdrawal_usecase"

	// 🆕 v3.0.0 : Usecases
	codusecase "Goshop/application/usecase/cod_usecase"
	creditusecase "Goshop/application/usecase/credit_usecase"
	merchantusecase "Goshop/application/usecase/merchant_usecase" // 🆕 AJOUT v3.5.0

	// 🆕 v4.1.0 : Merchant KYC Usecases
	merchantkycusecase "Goshop/application/usecase/merchant_kyc_usecase"

	// 🆕 v4.2.0 : Admin Shop Usecases
	adminshopusecase "Goshop/application/usecase/admin_shop_usecase"

	// 🆕 v4.4.0 : 2FA Usecases
	twofausecase "Goshop/application/usecase/twofa_usecase"

	// 🆕 v4.4.2 : Session Management Usecases
	sessionusecase "Goshop/application/usecase/session_usecase"

	// 🆕 v4.4.3 : API Key Management Usecases
	apikeyusecase "Goshop/application/usecase/apikey_usecase"

	// 🆕 v4.6.0 : Dispute Usecases
	disputeusecase "Goshop/application/usecase/dispute_usecase"

	// 🆕 v3.1.0 : Scheduler
	appscheduler "Goshop/application/scheduler"

	// 🆕 v4.3.2 : Configuration Email
	"Goshop/config"

	paymentinfra "Goshop/infrastructure/payment"
	"Goshop/infrastructure/payment/mock"
	"Goshop/infrastructure/postgres/collaborator"
	paymentpostgres "Goshop/infrastructure/postgres/payment"
	ratelimit "Goshop/infrastructure/rate_limit"

	authrefreshrepositoryinfra "Goshop/infrastructure/postgres/auth_refresh_repository_infra"
	"Goshop/infrastructure/postgres/customer"
	"Goshop/infrastructure/postgres/order"
	"Goshop/infrastructure/postgres/product"
	"Goshop/infrastructure/postgres/shop"
	"Goshop/infrastructure/postgres/tontine"
	txmanager "Goshop/infrastructure/postgres/tx_manager"
	userpostgres "Goshop/infrastructure/postgres/user_postgres"
	withdrawalpostgres "Goshop/infrastructure/withdrawal"

	// 🆕 v4.5.0 : WebSocket Infrastructure
	wsinfra "Goshop/infrastructure/websocket"

	// 🆕 v4.4.0 : 2FA Repository
	user2fainfra "Goshop/infrastructure/postgres/user_2fa"

	// 🆕 v4.4.2 : Session Management Repository
	usersessioninfra "Goshop/infrastructure/postgres/user_session"

	// 🆕 v4.4.3 : API Key Management Repository
	apikeyinfra "Goshop/infrastructure/postgres/api_key"

	// 🆕 v3.0.0 : Repositories PostgreSQL
	codinfra "Goshop/infrastructure/postgres/cod"
	creditinfra "Goshop/infrastructure/postgres/credit"
	escrowinfra "Goshop/infrastructure/postgres/escrow"
	freezeinfra "Goshop/infrastructure/postgres/freeze"
	walletinfra "Goshop/infrastructure/postgres/wallet"

	// 🆕 v4.6.0 : Dispute Repository
	disputeinfra "Goshop/infrastructure/postgres/dispute"

	// 🆕 v3.1.0 : Scheduler Infrastructure
	commissionbatch "Goshop/infrastructure/postgres/commission_batch"
	commissionrate "Goshop/infrastructure/postgres/commission_rate"
	infscheduler "Goshop/infrastructure/scheduler"

	"Goshop/domain/entity"
	"Goshop/domain/repository"
	"Goshop/domain/service"
	"Goshop/infrastructure/notification"

	"github.com/prometheus/client_golang/prometheus/promhttp"

	handlers "Goshop/interfaces/handler"

	// 🆕 v4.2.0 : Admin Shop Handler
	adminshophandler "Goshop/interfaces/handler/admin_shop_handler"

	// 🆕 v4.3.0 : Collaborator Handler
	collaboratorhandler "Goshop/interfaces/handler/collaborator_handler"

	commissionratehandler "Goshop/interfaces/handler/commission_rate_handler"
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
	merchanthandler "Goshop/interfaces/handler/merchant_handler" // 🆕 AJOUT v3.5.0
	wallethandler "Goshop/interfaces/handler/wallet_handler"

	// 🆕 v3.1.0 : Scheduler Handler
	schedulerhandler "Goshop/interfaces/handler/scheduler_handler"

	// 🆕 v4.1.0 : Merchant KYC Handler
	merchantkyhandler "Goshop/interfaces/handler/merchant_kyc_handler"

	// 🆕 v4.4.0 : 2FA Handler
	twofahandler "Goshop/interfaces/handler/twofa_handler"

	// 🆕 v4.4.2 : Session Management Handler
	sessionhandler "Goshop/interfaces/handler/session_handler"

	// 🆕 v4.4.3 : API Key Management Handler
	apikeyhandler "Goshop/interfaces/handler/apikey_handler"

	// 🆕 v4.6.0 : Dispute Handler
	disputehandler "Goshop/interfaces/handler/dispute_handler"

	"Goshop/config/setupLogging"
	"Goshop/interfaces/middl"
	"Goshop/interfaces/utils"

	httpSwagger "github.com/swaggo/http-swagger"
)

// ============================================================
// 🆕 WRAPPER POUR CREDIT UPDATER
// ============================================================
// Ce wrapper permet à app.go de fournir l'interface requise par ProcessWebhookUsecase
// sans créer de dépendance circulaire entre les packages usecase.

type creditUpdaterWrapper struct {
	installmentRepo repository.CreditInstallmentRepository
	contractRepo    repository.CreditContractRepository
	creditWalletUC  *walletusecase.CreditWalletUsecase
}

func (w *creditUpdaterWrapper) MarkInstallmentPaid(ctx context.Context, installmentID string, paymentID string) error {
	inst, err := w.installmentRepo.FindByID(ctx, installmentID)
	if err != nil {
		return err
	}
	if err := inst.MarkPaid(paymentID); err != nil {
		return err
	}
	return w.installmentRepo.Update(ctx, inst)
}

func (w *creditUpdaterWrapper) MarkContractDownPaymentPaid(ctx context.Context, contractID string, paymentID string) error {
	contract, err := w.contractRepo.FindByID(ctx, contractID)
	if err != nil {
		return err
	}
	if err := contract.MarkDownPaymentPaid(); err != nil {
		return err
	}
	return w.contractRepo.Update(ctx, contract)
}

func (w *creditUpdaterWrapper) CreditMerchantWallet(ctx context.Context, shopID string, amountCents int64, contractID string) error {
	_, err := w.creditWalletUC.CreditFromCreditPlan(ctx, shopID, amountCents, contractID)
	return err
}

// ============ STRUCT App ============

// App représente l'application configurable
type App struct {
	Router    *chi.Mux
	DB        *sql.DB
	Logger    *setupLogging.Logger
	Scheduler *infscheduler.CronScheduler // 🆕 v3.1.0
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
	r.Use(middl.CharsetUTF8)

	// ============================================================
	// 🛡️ SÉCURITÉ : Configuration CORS stricte (Fail-Fast en Prod)
	// ============================================================
	frontendURL := os.Getenv("FRONTEND_URL")

	// En production, on refuse catégoriquement une configuration CORS ouverte ou manquante
	if os.Getenv("APP_ENV") == "production" {
		if frontendURL == "" || frontendURL == "*" {
			log.Fatal("🚨 ERREUR FATALE DE SÉCURITÉ : FRONTEND_URL est manquant ou vaut '*' en production. Veuillez définir l'URL exacte de votre frontend.")
		}
	} else {
		// En développement, on autorise tout par commodité, mais on logue un avertissement
		if frontendURL == "" {
			frontendURL = "*"
			a.Logger.Warn().Msg("⚠️ FRONTEND_URL non défini, fallback sur '*' (DEV ONLY)")
		}
	}
	r.Use(middl.CORS(frontendURL))

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

	// 🆕 v2.9.0 : Repository KYC Client
	kycDocRepo := customer.NewCustomerKYCRepositoryInfrastructure(a.DB)

	// 🆕 v3.0.0 : Repositories Credit
	creditPlanRepo := creditinfra.NewCreditPlanRepositoryInfrastructure(a.DB)
	creditAppRepo := creditinfra.NewCreditApplicationRepositoryInfrastructure(a.DB)
	creditContractRepo := creditinfra.NewCreditContractRepositoryInfrastructure(a.DB)
	creditInstallmentRepo := creditinfra.NewCreditInstallmentRepositoryInfrastructure(a.DB)
	creditScoreRepo := creditinfra.NewCreditScoreRepositoryInfrastructure(a.DB)

	// 🆕 v3.0.0 : Repositories Escrow
	escrowRepo := escrowinfra.NewEscrowAccountRepositoryInfrastructure(a.DB) // 🆕 Assigné à une variable
	_ = escrowinfra.NewDeliveryProofRepositoryInfrastructure(a.DB)

	// 🆕 v3.0.0 : Repositories Wallet
	walletRepo := walletinfra.NewMerchantWalletRepositoryInfrastructure(a.DB)
	walletTxnRepo := walletinfra.NewWalletTransactionRepositoryInfrastructure(a.DB)

	// 🆕 v3.0.0 : Repository COD
	codProofRepo := codinfra.NewCODProofRepositoryInfrastructure(a.DB)

	// 🆕 v3.0.0 : Repository Freeze
	freezeRepo := freezeinfra.NewAccountFreezeRepositoryInfrastructure(a.DB)

	// 🆕 v3.1.0 : Repository Commission Batches
	batchRepo := commissionbatch.NewCommissionBatchRepositoryPostgres(a.DB)

	// 🆕 v3.2.0 : Repository Commission Rates
	rateRepo := commissionrate.NewCommissionRateRepositoryPostgres(a.DB)

	// 🆕 v4.1.0 : Repository Shop KYC Documents (KYC Marchand)
	shopKYCDocRepo := shop.NewShopKYCDocumentRepositoryInfrastructure(a.DB)

	// 🆕 v4.2.0 : Repository Shop Admin Actions (Audit Trail)
	adminActionRepo := shop.NewShopAdminActionRepositoryInfrastructure(a.DB)

	// 🆕 v4.3.0 : Repositories Collaborateurs
	platformCollabRepo := collaborator.NewPlatformCollaboratorRepositoryInfrastructure(a.DB)
	shopCollabRepo := collaborator.NewShopCollaboratorRepositoryInfrastructure(a.DB)
	invitationRepo := collaborator.NewCollaboratorInvitationRepositoryInfrastructure(a.DB)

	// 🆕 v4.4.0 : Repository 2FA
	user2faRepo := user2fainfra.NewUser2FARepository(a.DB)

	// 🆕 v4.4.2 : Repository Session Management
	userSessionRepo := usersessioninfra.NewUserSessionRepository(a.DB)

	// 🆕 v4.4.3 : Repository API Keys
	apiKeyRepo := apikeyinfra.NewAPIKeyRepository(a.DB)

	// 🆕 v4.6.0 : Repository Dispute
	disputeRepo := disputeinfra.NewDisputeRepositoryPostgres(a.DB)

	a.Logger.Info().Msg("✅ v4.6.0 repositories initialized (all + user_2fa + user_sessions + api_keys + dispute)")

	// ============================================================
	// 🛡️ SÉCURITÉ CRITIQUE : Enregistrement des Providers de Paiement
	// ============================================================

	// 1. Yenga Pay (Réel) - Seulement si les variables d'environnement sont présentes
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
				a.Logger.Info().Msg("✅ Yenga Pay provider registered")
			}
		}
	} else {
		a.Logger.Warn().Msg("⚠️ Yenga Pay provider not configured globally (missing YENGA_PAY_API_KEY)")
	}

	// 2. Mocks Orange Money / Moov Money - STRICTEMENT INTERDIT EN PRODUCTION
	appEnv := os.Getenv("APP_ENV")
	if appEnv == "production" {
		a.Logger.Info().Msg("✅ Production environment: Mock payment providers are DISABLED")
	} else {
		// En dev/test, on enregistre les mocks.
		// La sécurité repose ici sur le fait que ce bloc ne s'exécute JAMAIS en production.
		orangeMoneyProvider := mock.NewOrangeMoneyProvider(mock.DefaultOrangeMoneyConfig())
		if err := paymentRegistry.Register(orangeMoneyProvider); err != nil {
			a.Logger.Error().Err(err).Msg("Failed to register Mock Orange Money provider")
		} else {
			a.Logger.Info().Msg("⚠️ Mock Orange Money provider registered (DEV/TEST ONLY)")
		}

		moovMoneyProvider := mock.NewMoovMoneyProvider(mock.DefaultMoovMoneyConfig())
		if err := paymentRegistry.Register(moovMoneyProvider); err != nil {
			a.Logger.Error().Err(err).Msg("Failed to register Mock Moov Money provider")
		} else {
			a.Logger.Info().Msg("⚠️ Mock Moov Money provider registered (DEV/TEST ONLY)")
		}
	}

	// ============ 🆕 v4.3.2 : EMAIL SERVICE (SMTP) ============
	emailConfig := config.LoadEmailConfig()
	var emailService service.EmailService

	smtpService, err := notification.NewSMTPService(emailConfig, a.Logger.Logger)
	if err != nil {
		a.Logger.Warn().Err(err).Msg("⚠️ Email service non disponible (mode dégradé)")
		emailService = notification.NewNoopEmailService(a.Logger.Logger)
	} else {
		emailService = smtpService
		a.Logger.Info().
			Str("provider", emailService.GetProvider()).
			Bool("configured", emailService.IsConfigured()).
			Bool("debug_mode", emailConfig.DebugMode).
			Msg("✅ v4.3.2 Email service initialized")
	}

	// ============ 🆕 v4.5.0 : WEBSOCKET HUB ============
	// On l'initialise ici pour qu'il soit disponible pour le NotificationDispatcher
	var wsHub *wsinfra.Hub
	if utils.Rdb != nil {
		if err := utils.Rdb.Ping(context.Background()).Err(); err == nil {
			wsHub = wsinfra.NewHub(utils.Rdb, a.Logger.Logger)
			a.Logger.Info().Msg("✅ v4.5.0 WebSocket Hub initialized with Redis Pub/Sub")
		}
	}
	if wsHub == nil {
		a.Logger.Warn().Msg("⚠️ v4.5.0 WebSocket Hub disabled (Redis not available)")
	}

	// ============ 🆕 v4.6.1 : EMAIL NOTIFICATION PROVIDER ============
	emailNotifProvider := notification.NewEmailNotificationProvider(
		emailService,
		"GoShop",
		os.Getenv("FRONTEND_URL"),
		os.Getenv("ADMIN_EMAIL"),
		a.Logger.Logger,
	)
	a.Logger.Info().Msg("✅ v4.6.1 Email notification provider initialized")

	// ============ 🆕 v4.5.0 : NOTIFICATION DISPATCHER ============
	var notifService service.NotificationService
	if wsHub != nil || emailService != nil {
		notifService = notification.NewNotificationDispatcher(
			wsHub,
			emailNotifProvider,
			postgresCustomerRepo,
			shopRepo,
			postgresUserRepo, // ✅ postgresUserRepo implémente bien userrepository.UserRepository
			a.Logger.Logger,
		)
		a.Logger.Info().Msg("✅ v4.6.1 Notification Dispatcher initialized (WebSocket + Email)")
	} else {
		notifService = notification.NewNoopNotificationService(a.Logger.Logger)
		a.Logger.Warn().Msg("⚠️ v4.5.0 Notification Dispatcher using Noop")
	}

	// -- Usecases (existants)
	// 🆕 v4.4.2 : Ajout de userSessionRepo comme 2ème paramètre
	refreshUsecase := authusecase.NewRefreshUsecase(
		refreshSessionRepo,
		userSessionRepo, // 🆕 v4.4.2
		utils.ValidateToken,
		utils.GenerateAccessToken,
		utils.GenerateRefreshToken,
		time.Now,
		uuid.NewString,
		30*24*time.Hour,
	)

	// Shop Usecases
	// 🆕 Injection du shopCollabRepo pour que le créateur de la boutique soit automatiquement shop_admin
	createShopUsecase := shopusecase.NewCreateShopUsecase(shopRepo, shopCollabRepo)
	listShopsUsecase := shopusecase.NewListShopsUsecase(shopRepo)
	updateShopUsecase := shopusecase.NewUpdateShopUsecase(shopRepo)

	// Configure Payment Usecase (pour config par boutique)
	configurePaymentUC := shopusecase.NewConfigurePaymentUsecase(shopRepo, shopRepo, shopRepo)

	// 🆕 v2.9.0 : Configure Tontine Usecase
	configureTontineUC := shopusecase.NewConfigureTontineUsecase(tontineSettingsRepo, postgreProductRepo)

	// Payment Usecases
	initiatePaymentUC := paymentusecase.NewInitiatePaymentUsecase(
		paymentRepo,
		postgresOrderRepo,
		paymentRegistry,
		shopRepo,
		escrowRepo,
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

	// 🆕 v3.0.0 : WALLET USECASES
	creditWalletUC := walletusecase.NewCreditWalletUsecase(walletRepo, walletTxnRepo, txmanagerRepo)
	debitWalletUC := walletusecase.NewDebitWalletUsecase(walletRepo, walletTxnRepo, txmanagerRepo)
	freezeAccountUC := walletusecase.NewFreezeAccountUsecase(walletRepo, freezeRepo, txmanagerRepo)
	unfreezeAccountUC := walletusecase.NewUnfreezeAccountUsecase(walletRepo, freezeRepo, walletTxnRepo, txmanagerRepo)

	// 🆕 Complete Payment Usecase (utilise maintenant l'Escrow)
	completePaymentUC := paymentusecase.NewCompletePaymentUsecase(
		paymentRepo,
		postgresOrderRepo,
		paymentRegistry,
		shopRepo,   // ShopPaymentSettingsRepository
		escrowRepo, // EscrowAccountRepository
	)

	// 🆕 v2.9.0 : Process Tontine Webhook Usecase
	processTontineWebhookUC := paymentusecase.NewProcessTontineWebhookUsecase(
		tontinePaymentRepo,
		tontineGroupRepo,
		tontineParticipantRepo,
		tontineVoucherRepo,
		shopRepo,
	)

	// Process Webhook Usecase (utilise maintenant l'Escrow)
	processWebhookUC := paymentusecase.NewProcessWebhookUsecase(
		paymentRepo,
		paymentRegistry,
		a.DB,
		shopRepo,
		shopRepo, // ShopPaymentSettingsRepository
		processTontineWebhookUC,
		&creditUpdaterWrapper{
			installmentRepo: creditInstallmentRepo,
			contractRepo:    creditContractRepo,
			creditWalletUC:  creditWalletUC,
		},
		escrowRepo, // 🆕 EscrowAccountRepository
	)

	// Withdrawal Usecases
	createWithdrawalUC := withdrawalusecase.NewCreateWithdrawalUsecase(
		withdrawalRepo,
		shopRepo,
		paymentRegistry,
		debitWalletUC, // ✅ AJOUT : Injection du usecase de débit pour sécuriser les retraits
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
		escrowRepo,    // 🆕 EscrowAccountRepository
		walletRepo,    // 🆕 MerchantWalletRepository
		walletTxnRepo, // 🆕 WalletTransactionRepository
		notifService,
		txmanagerRepo,
		disputeRepo,
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

	// ============ 🆕 v2.9.0 : KYC USECASES (Client) ============
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

	// ============ 🆕 v4.1.0 : MERCHANT KYC USECASES ============
	submitMerchantKYCUC := merchantkycusecase.NewSubmitMerchantKYCUsecase(
		shopRepo,
		shopKYCDocRepo,
	)

	reviewMerchantKYCUC := merchantkycusecase.NewReviewMerchantKYCUsecase(
		shopRepo,
		shopKYCDocRepo,
	)

	listPendingMerchantKYCUC := merchantkycusecase.NewListPendingMerchantKYCUsecase(
		shopRepo,
		shopKYCDocRepo,
	)

	getMerchantKYCStatusUC := merchantkycusecase.NewGetMerchantKYCStatusUsecase(
		shopRepo,
		shopKYCDocRepo,
	)

	a.Logger.Info().Msg("✅ v4.1.0 Merchant KYC usecases initialized (submit, review, list_pending, get_status)")

	// ============ 🆕 v4.2.0 : ADMIN SHOP USECASES ============
	adminListShopsUC := adminshopusecase.NewListShopsUsecase(shopRepo)
	adminGetShopDetailsUC := adminshopusecase.NewGetShopDetailsUsecase(shopRepo, shopKYCDocRepo)
	adminGetShopHealthUC := adminshopusecase.NewGetShopHealthUsecase(shopRepo)
	adminSuspendShopUC := adminshopusecase.NewSuspendShopUsecase(shopRepo, adminActionRepo)
	adminActivateShopUC := adminshopusecase.NewActivateShopUsecase(shopRepo, adminActionRepo)

	a.Logger.Info().Msg("✅ v4.2.0 Admin Shop usecases initialized (list, details, health, suspend, activate)")

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

	// 🆕 AJOUT : PayDownPayment Usecase sécurisé (initie le paiement via YengaPay)
	payDownPaymentUC := creditusecase.NewPayDownPaymentUsecase(
		creditContractRepo,
		paymentRepo,
		paymentRegistry, // Implémente l'interface locale PaymentRegistry
		txmanagerRepo,
	)

	// 🆕 AJOUT : PayInstallment Usecase
	payInstallmentUC := creditusecase.NewPayInstallmentUsecase(
		creditInstallmentRepo,
		creditContractRepo,
		paymentRepo,
		paymentRegistry, // Implémente l'interface locale GetAvailable
		txmanagerRepo,
	)

	// 🆕 v3.5.0 : MERCHANT OVERVIEW USECASE
	getMerchantOverviewUC := merchantusecase.NewGetMerchantOverviewUsecase(
		postgresOrderRepo,
		paymentRepo,
		creditContractRepo,
		creditInstallmentRepo,
		walletRepo,
		freezeRepo,
		batchRepo,
		escrowRepo, // 🆕 AJOUTÉ ICI
	)

	// 🆕 v3.6.0 : PUBLIC PRODUCT CATALOG USECASE & HANDLER
	listPublicProductsUC := productuscase.NewListPublicProductsUsecase(postgreProductRepo)
	publicProductHandler := productHandler.NewPublicProductHandler(listPublicProductsUC)

	a.Logger.Info().Msg("✅ v3.5.0 Credit & Merchant Overview usecases initialized")
	a.Logger.Info().Msg("✅ v3.6.0 Public Product Catalog usecase initialized")

	// ============ 🆕 v4.3.0 : COLLABORATOR USECASES ============
	invitePlatformUC := collaboratorusecase.NewInvitePlatformCollaboratorUsecase(
		platformCollabRepo,
		invitationRepo,
		postgresUserRepo,
		emailService,
	)

	inviteShopUC := collaboratorusecase.NewInviteShopCollaboratorUsecase(
		shopCollabRepo,
		invitationRepo,
		shopRepo,
		postgresUserRepo,
		emailService,
	)

	acceptInvitationUC := collaboratorusecase.NewAcceptInvitationUsecase(
		invitationRepo,
		platformCollabRepo,
		shopCollabRepo,
		shopRepo,
		postgresUserRepo,
	)

	listCollabsUC := collaboratorusecase.NewListCollaboratorsUsecase(
		platformCollabRepo,
		shopCollabRepo,
		shopRepo,
	)

	updateRoleUC := collaboratorusecase.NewUpdateCollaboratorRoleUsecase(
		platformCollabRepo,
		shopCollabRepo,
		shopRepo,
	)

	removeCollabUC := collaboratorusecase.NewRemoveCollaboratorUsecase(
		platformCollabRepo,
		shopCollabRepo,
		shopRepo,
	)

	a.Logger.Info().Msg("✅ v4.3.2 Collaborator usecases initialized (invite, accept, list, update, remove) + emailService")

	// ============ 🆕 v4.4.0 : 2FA USECASES ============
	setup2FAUC := twofausecase.NewSetup2FAUsecase(
		user2faRepo,
		postgresUserRepo,
	)

	verifyEnable2FAUC := twofausecase.NewVerifyAndEnable2FAUsecase(
		user2faRepo,
	)

	disable2FAUC := twofausecase.NewDisable2FAUsecase(
		user2faRepo,
	)

	getStatus2FAUC := twofausecase.NewGet2FAStatusUsecase(
		user2faRepo,
	)

	regenerateCodesUC := twofausecase.NewRegenerateRecoveryCodesUsecase(
		user2faRepo,
	)

	a.Logger.Info().Msg("✅ v4.4.0 2FA usecases initialized (setup, verify, disable, status, regenerate)")

	// ============ 🆕 v4.4.2 : SESSION MANAGEMENT USECASES ============
	listSessionsUC := sessionusecase.NewListSessionsUsecase(
		userSessionRepo,
	)

	revokeSessionUC := sessionusecase.NewRevokeSessionUsecase(
		userSessionRepo,
	)

	revokeAllSessionsUC := sessionusecase.NewRevokeAllSessionsUsecase(
		userSessionRepo,
	)

	getSessionStatsUC := sessionusecase.NewGetSessionStatsUsecase(
		userSessionRepo,
	)

	cleanupSessionsUC := sessionusecase.NewCleanupSessionsUsecase(
		userSessionRepo,
	)

	a.Logger.Info().Msg("✅ v4.4.2 Session Management usecases initialized (list, revoke, revoke-all, stats, cleanup)")

	// ============ 🆕 v4.4.3 : API KEY MANAGEMENT USECASES ============
	createAPIKeyUC := apikeyusecase.NewCreateAPIKeyUsecase(
		apiKeyRepo,
	)

	listAPIKeysUC := apikeyusecase.NewListAPIKeysUsecase(
		apiKeyRepo,
	)

	revokeAPIKeyUC := apikeyusecase.NewRevokeAPIKeyUsecase(
		apiKeyRepo,
	)

	revokeAllAPIKeysUC := apikeyusecase.NewRevokeAllAPIKeysUsecase(
		apiKeyRepo,
	)

	getAPIKeyStatsUC := apikeyusecase.NewGetAPIKeyStatsUsecase(
		apiKeyRepo,
	)

	a.Logger.Info().Msg("✅ v4.4.3 API Key Management usecases initialized (create, list, revoke, revoke-all, stats)")

	// ============ 🆕 v4.6.0 : DISPUTE USECASES ============
	openDisputeUC := disputeusecase.NewOpenDisputeUsecase(
		disputeRepo,
		postgresOrderRepo,
		escrowRepo,
	)

	resolveDisputeUC := disputeusecase.NewResolveDisputeUsecase(
		disputeRepo,
		escrowRepo,
		walletRepo,
		walletTxnRepo,
		txmanagerRepo,
		paymentRepo,       // 🆕 AJOUTÉ : pour retrouver la transaction à rembourser
		paymentRegistry,   // 🆕 AJOUTÉ : pour appeler la méthode Refund du provider
		postgresOrderRepo, // 🆕 AJOUTE CET ARGUMENT
		notifService,
	)

	a.Logger.Info().Msg("✅ v4.6.0 Dispute usecases initialized")

	// ============ 🆕 CLIENT DASHBOARD USECASE ============
	getDashboardUC := customerusecase.NewGetClientDashboardUsecase(postgresCustomerRepo)
	a.Logger.Info().Msg("✅ Client Dashboard usecase initialized")

	// ============ 🆕 v3.1.0 : COMMISSION SCHEDULER (COD) ============
	commissionSched := appscheduler.NewCommissionScheduler(
		codProofRepo,
		batchRepo,
		collectCommissionUC,
		shopRepo,
		a.Logger.Logger,
	)

	a.Logger.Info().Msg("✅ v3.1.0 Commission scheduler initialized")

	// ============ 🆕 v3.2.0 : ONLINE PAYMENT SCHEDULER ============
	onlinePaymentSched := appscheduler.NewOnlinePaymentScheduler(
		paymentRepo,
		batchRepo,
		rateRepo,
		walletRepo,
		walletTxnRepo,
		debitWalletUC,
		freezeAccountUC,
		a.Logger.Logger,
	)

	a.Logger.Info().Msg("✅ v3.2.0 Online payment scheduler initialized")

	// ============ 🆕 v3.3.0 : TONTINE SCHEDULER ============
	tontineSched := appscheduler.NewTontineScheduler(
		tontinePaymentRepo,
		tontineGroupRepo,
		batchRepo,
		rateRepo,
		debitWalletUC,
		freezeAccountUC,
		a.Logger.Logger,
	)

	a.Logger.Info().Msg("✅ v3.3.0 Tontine scheduler initialized")

	// ============ 🆕 v3.4.0 : CREDIT SCHEDULER ============
	creditSched := appscheduler.NewCreditScheduler(
		creditInstallmentRepo, // 1. installmentRepo
		creditContractRepo,    // 2. contractRepo (NOUVEAU)
		postgresCustomerRepo,  // 3. customerRepo (NOUVEAU)
		payInstallmentUC,      // 4. payInstallmentUC (NOUVEAU)
		batchRepo,             // 5. batchRepo
		rateRepo,              // 6. rateRepo
		debitWalletUC,         // 7. debitUC
		freezeAccountUC,       // 8. freezeUC
		a.Logger.Logger,       // 9. logger
	)

	a.Logger.Info().Msg("✅ v3.4.0 Credit scheduler initialized (with auto-trigger capability)")

	// -- Handlers (existants)
	refreshHandler := refreshhandler.NewRefreshHandler(refreshUsecase)

	productHandler := productHandler.NewProductHandler(
		postgreProductRepo,
		txmanagerRepo,
	)

	customerHandler := customerhandler.NewCustomerHandler(
		postgresCustomerRepo,
		postgresUserRepo,
		txmanagerRepo,
	)

	// 🆕 Client Dashboard Handler
	clientDashboardHandler := customerhandler.NewCustomerDashboardHandler(getDashboardUC, postgresCustomerRepo)

	orderHandler := ordershandler.NewOrderHandler(
		a.DB,
		txmanagerRepo,
		postgresOrderRepo,
		postgreProductRepo,
		postgresCustomerRepo,
		postgresOrderItem,
		codProofRepo,
	)

	// Cash Order Handler
	cashOrderHandler := ordershandler.NewCashOrderHandler(
		acceptOrderUC,
		rejectOrderUC,
		outForDeliveryUC,
		deliverOrderUC,
		cancelOrderUC,
	)

	// 🆕 v4.4.21 : Initialisation du Rate Limiter pour le login
	var loginRateLimiter service.LoginRateLimiter
	if utils.Rdb != nil {
		if err := utils.Rdb.Ping(context.Background()).Err(); err == nil {
			loginRateLimiter = ratelimit.NewLoginRateLimiterRedis(utils.Rdb)
			a.Logger.Info().Msg("✅ v4.4.21 Login rate limiter initialisé (Redis)")
		}
	}
	if loginRateLimiter == nil {
		loginRateLimiter = ratelimit.NewLoginRateLimiterMemory()
		a.Logger.Warn().Msg("⚠️ v4.4.21 Login rate limiter initialisé (mémoire - fallback)")
	}

	// 🆕 v4.4.21 : Ajout de rateLimiter comme 3ème paramètre
	userHandler := userhandler.NewUserHandler(
		postgresUserRepo,
		userSessionRepo,  // 🆕 v4.4.2
		loginRateLimiter, // 🆕 v4.4.21
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
		postgresCustomerRepo, // 🆕 AJOUTÉ
	)

	// 🆕 v2.9.0 : KYC Handler (Client)
	kycHandler := customerhandler.NewKYCHandler(
		uploadKYCUC,
		getKYCStatusUC,
		reviewKYCUC,
		listPendingKYCUC,
		postgresCustomerRepo, // 🆕 AJOUTÉ
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

	// 🆕 AJOUT : Injection de payDownPaymentUC et payInstallmentUC dans le CreditHandler
	creditHandler := credithandler.NewCreditHandler(
		configureCreditPlanUC,
		applyForCreditUC,
		approveCreditUC,
		rejectCreditUC,
		payDownPaymentUC,
		payInstallmentUC,
	)

	// 🆕 v3.5.0 : MERCHANT OVERVIEW HANDLER
	merchantOverviewHandler := merchanthandler.NewMerchantOverviewHandler(getMerchantOverviewUC)

	// ============ 🆕 v3.1.0 : SCHEDULER HANDLER ============
	schedulerHandler := schedulerhandler.NewSchedulerHandler(
		commissionSched,
		batchRepo,
	)

	// ============ 🆕 v3.3.0 : COMMISSION RATE HANDLER ============
	commissionRateHandler := commissionratehandler.NewCommissionRateHandler(
		rateRepo,
		onlinePaymentSched,
		tontineSched,
		creditSched,
	)

	// ============ 🆕 v4.1.0 : MERCHANT KYC HANDLER ============
	merchantKYCHandler := merchantkyhandler.NewMerchantKYCHandler(
		submitMerchantKYCUC,
		reviewMerchantKYCUC,
		listPendingMerchantKYCUC,
		getMerchantKYCStatusUC,
	)

	// ============ 🆕 v4.2.0 : ADMIN SHOP HANDLER ============
	adminShopHandler := adminshophandler.NewAdminShopHandler(
		adminListShopsUC,
		adminGetShopDetailsUC,
		adminGetShopHealthUC,
		adminSuspendShopUC,
		adminActivateShopUC,
	)

	// ============ 🆕 v4.3.0 : COLLABORATOR HANDLER ============
	collaboratorHandler := collaboratorhandler.NewCollaboratorHandler(
		invitePlatformUC,
		inviteShopUC,
		acceptInvitationUC,
		listCollabsUC,
		updateRoleUC,
		removeCollabUC,
	)

	// ============ 🆕 v4.4.0 : 2FA HANDLER ============
	twoFAHandler := twofahandler.NewTwoFAHandler(
		setup2FAUC,
		verifyEnable2FAUC,
		disable2FAUC,
		getStatus2FAUC,
		regenerateCodesUC,
	)

	// ============ 🆕 v4.4.2 : SESSION MANAGEMENT HANDLER ============
	sessionHandler := sessionhandler.NewSessionHandler(
		listSessionsUC,
		revokeSessionUC,
		revokeAllSessionsUC,
		getSessionStatsUC,
		cleanupSessionsUC,
	)

	// ============ 🆕 v4.4.3 : API KEY MANAGEMENT HANDLER ============
	apiKeyHandler := apikeyhandler.NewAPIKeyHandler(
		createAPIKeyUC,
		listAPIKeysUC,
		revokeAPIKeyUC,
		revokeAllAPIKeysUC,
		getAPIKeyStatsUC,
	)

	// ============ 🆕 v4.6.0 : DISPUTE HANDLER ============
	disputeHandler := disputehandler.NewDisputeHandler(
		openDisputeUC,
		resolveDisputeUC,
		disputeRepo,
	)

	// ============ 🆕 v4.5.0 : WEBSOCKET HANDLER ============
	var wsHandler *handlers.WSHandler
	if wsHub != nil {
		wsHandler = handlers.NewWSHandler(wsHub)
	}

	a.Logger.Info().Msg("✅ v4.6.0 handlers initialized (websocket, wallet, cod, credit, scheduler, commission_rate, merchant_kyc, admin_shop, collaborator, 2fa, sessions, api_keys, merchant_overview, public_products, dispute)")

	// ============================================================
	// 🆕 v4.4.2 : Middleware Auth avec vérification de session
	// ============================================================
	authMiddlewareWithSession := middleware.NewAuthMiddleware(middleware.AuthMiddlewareConfig{
		SessionRepo: userSessionRepo,
	})

	// ============ 3. ROUTES PUBLIQUES ============
	r.Use(middl.PrometheusMiddleware)

	r.Get("/health/live", hh.Live)
	r.Get("/health/ready", hh.Ready)

	r.Post("/auth/refresh", middl.ErrorHandler(refreshHandler.Refresh))
	r.Post("/register", middl.ErrorHandler(userHandler.Register))
	r.Post("/login", middl.ErrorHandler(userHandler.Login))

	// 🆕 v4.4.2 : Logout route (nécessite auth + session active)
	r.With(authMiddlewareWithSession).
		Post("/logout", middl.ErrorHandler(userHandler.Logout))

	r.Get("/help", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("Goshop API est en ligne !"))
	})

	// 🆕 v3.6.0 : Catalogue public (Pas d'authentification requise)
	r.Get("/api/public/products", middl.ErrorHandler(publicProductHandler.GetPublicProducts))

	r.Handle("/metrics", promhttp.Handler())
	r.Get("/swagger/*", httpSwagger.Handler())

	// Webhooks (public, pas d'auth requise)
	r.Post("/webhooks/{provider}", middl.ErrorHandler(webhookHandler.HandleWebhook))

	// ============================================================
	// 🆕 v4.4.3 : ROUTES PROTÉGÉES PAR API KEY (pour intégrations tierces)
	// ============================================================
	// Ces routes peuvent être appelées avec une API Key au lieu d'un JWT
	// Exemple : curl -H "X-API-Key: gsk_live_..." https://api.goshop.com/v1/public/products

	// Route publique avec authentification API Key (lecture produits)
	r.With(middl.APIKeyAuth(middl.APIKeyAuthConfig{
		APIKeyRepo:    apiKeyRepo,
		RequiredScope: entity.ScopeReadProducts,
	})).Get("/v1/public/products", func(w http.ResponseWriter, r *http.Request) {
		utils.WriteJSON(w, http.StatusOK, map[string]interface{}{
			"success":  true,
			"message":  "API Key authentifiée avec succès",
			"scope":    "read:products",
			"endpoint": "/v1/public/products",
		})
	})

	// Route publique avec authentification API Key (lecture shops)
	r.With(middl.APIKeyAuth(middl.APIKeyAuthConfig{
		APIKeyRepo:    apiKeyRepo,
		RequiredScope: entity.ScopeReadShops,
	})).Get("/v1/public/shops", func(w http.ResponseWriter, r *http.Request) {
		utils.WriteJSON(w, http.StatusOK, map[string]interface{}{
			"success":  true,
			"message":  "API Key authentifiée avec succès",
			"scope":    "read:shops",
			"endpoint": "/v1/public/shops",
		})
	})

	// ============ 🆕 v4.3.0 : PUBLIC COLLABORATOR INVITATION ROUTES ============
	r.Route("/api/collaborators/invitations", func(r chi.Router) {
		r.Use(middl.RateLimiter)
		collaboratorHandler.RegisterPublicRoutes(r)
	})

	// ============ 4. ROUTE PROTÉGÉE (user authentifié) ============
	r.With(authMiddlewareWithSession).
		Get("/auth/me", middl.ErrorHandler(userHandler.Me))

	// 🆕 v4.5.0 : WebSocket Notifications Route (nécessite Auth)
	if wsHandler != nil {
		r.With(authMiddlewareWithSession).
			Get("/ws/notifications", wsHandler.HandleNotifications)
	}

	// ============ 5. ROUTES API PROTÉGÉES + MULTI-TENANT ============
	r.Route("/api", func(r chi.Router) {
		r.Use(authMiddlewareWithSession)

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

		// ---------------------------------------------------------
		// ✅ FIX AUDIT #2 : GROUPE A - Routes Marchand (Nécessite RequireShopAccess)
		// ---------------------------------------------------------
		r.Group(func(r chi.Router) {
			// 1. Résoudre la boutique à partir du header/sous-domaine
			r.Use(middl.TenantResolver(shopRepo, a.Logger.Logger))

			// 🛡️ SÉCURITÉ CRITIQUE (IDOR) : Vérifier que l'utilisateur est owner ou collaborateur
			r.Use(middl.RequireShopAccess(shopCollabRepo))

			// Products
			r.Route("/products", func(r chi.Router) {
				r.Post("/", middl.ErrorHandler(productHandler.CreateProduct))
				r.Get("/", middl.ErrorHandler(productHandler.GetAllProducts))
				r.Get("/{id}", middl.ErrorHandler(productHandler.GetProductById))
				r.Put("/{id}", middl.ErrorHandler(productHandler.UpdateProduct))
				r.Delete("/{id}", middl.ErrorHandler(productHandler.DeleteProduct))
			})

			// Customers (CRUD only, NOT kyc/upload)
			r.Route("/customers", func(r chi.Router) {
				r.Post("/", middl.ErrorHandler(customerHandler.CreateCustomerHandler))
				r.Get("/", middl.ErrorHandler(customerHandler.GetAllCustomersHandler))
				r.Get("/{id}", middl.ErrorHandler(customerHandler.GetCustomerByIdHandler))
				r.Put("/{id}", middl.ErrorHandler(customerHandler.UpdateCustomerHandler))
				r.Delete("/{id}", middl.ErrorHandler(customerHandler.DeleteCustomerHandler))

				r.Get("/{customer_id}/kyc/status", middl.ErrorHandler(kycHandler.GetKYCStatus))
			})

			// Orders (existant + cash workflow + dispute)
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

			// 🆕 v2.9.0 : Merchant KYC routes
			r.Route("/merchant/kyc", func(r chi.Router) {
				r.Get("/pending", middl.ErrorHandler(kycHandler.ListPendingKYC))
				r.Post("/{customer_id}/review", middl.ErrorHandler(kycHandler.ReviewKYC))
			})

			// ============ 🆕 v4.1.0 : MERCHANT KYC ROUTES ============
			r.Route("/merchant-kyc", func(r chi.Router) {
				merchantKYCHandler.RegisterMerchantRoutes(r)
			})

			// ============ 🆕 v3.0.0 : WALLET ROUTES ============
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

			// ============ 🆕 v3.5.0 : MERCHANT OVERVIEW ROUTE ============
			r.Route("/merchant", func(r chi.Router) {
				r.Get("/overview", middl.ErrorHandler(merchantOverviewHandler.GetOverview))
			})
		})

		// ---------------------------------------------------------
		// ✅ FIX AUDIT #2 : GROUPE B - Routes Client "Self-Service"
		// ---------------------------------------------------------
		// Ces routes sont appelées par le client final (rôle "user").
		// Elles ont besoin du TenantResolver, mais PAS de RequireShopAccess,
		// sinon le client reçoit un 403 Forbidden.
		r.Group(func(r chi.Router) {
			r.Use(middl.TenantResolver(shopRepo, a.Logger.Logger))
			// PAS de middl.RequireShopAccess ici !

			// Dashboard financier du client connecté
			r.Get("/client/dashboard", middl.ErrorHandler(clientDashboardHandler.GetDashboard))

			// Upload KYC par le client pour lui-même
			r.Post("/customers/kyc/upload", middl.ErrorHandler(kycHandler.UploadKYC))

			// Actions Tontine pour le client
			r.Post("/tontine/groups/join", middl.ErrorHandler(tontineHandler.JoinGroup))
			r.Post("/tontine/groups/{group_id}/pay", middl.ErrorHandler(tontineHandler.PayCycle))
			r.Get("/tontine/groups/{group_id}/payments", middl.ErrorHandler(tontineHandler.ListCustomerPayments))

			r.Post("/orders/{id}/dispute", middl.ErrorHandler(disputeHandler.OpenDispute))
		})

		// ============================================================
		// 🆕 v4.0.0 : ADMIN ROUTES - PROTÉGÉES PAR RBAC
		// ============================================================

		// ============ ADMIN SCHEDULER ROUTES ============
		r.Route("/admin/scheduler", func(r chi.Router) {
			r.Use(middl.RequireRoles("super_admin", "admin"))
			r.Post("/trigger", middl.ErrorHandler(schedulerHandler.TriggerManualCollection))
			r.Get("/batches", middl.ErrorHandler(schedulerHandler.GetRecentBatches))
			r.Get("/batches/{id}", middl.ErrorHandler(schedulerHandler.GetBatchDetails))
			r.Get("/stats", middl.ErrorHandler(schedulerHandler.GetDailyStats))
		})

		// ============ COMMISSION RATES ROUTES ============
		r.Route("/admin/commission-rates", func(r chi.Router) {
			r.Use(middl.RequireRoles("super_admin", "admin"))
			r.Put("/", middl.ErrorHandler(commissionRateHandler.UpdateRate))
			r.Get("/", middl.ErrorHandler(commissionRateHandler.GetRates))
			r.Post("/trigger-online", middl.ErrorHandler(commissionRateHandler.TriggerOnlineCollection))
			r.Post("/trigger-tontine", middl.ErrorHandler(commissionRateHandler.TriggerTontineCollection))
			r.Post("/trigger-credit", middl.ErrorHandler(commissionRateHandler.TriggerCreditCollection))
		})

		// ============ 🆕 v4.1.0 : ADMIN MERCHANT KYC ROUTES ============
		r.Route("/admin/merchant-kyc", func(r chi.Router) {
			r.Use(middl.RequireRoles("super_admin", "admin"))
			merchantKYCHandler.RegisterAdminRoutes(r)
		})

		// ============ 🆕 v4.2.0 : ADMIN SHOP ROUTES ============
		r.Route("/admin/shops", func(r chi.Router) {
			r.Use(middl.RequireRoles("super_admin", "admin"))
			adminShopHandler.RegisterRoutes(r)
		})

		// ============ 🆕 v4.3.0 : ADMIN COLLABORATOR PLATFORM ROUTES ============
		r.Route("/admin/collaborators/platform", func(r chi.Router) {
			r.Use(middl.RequireRoles("super_admin", "admin"))
			collaboratorHandler.RegisterAdminPlatformRoutes(r)
		})

		// ============ 🆕 v4.3.0 : SHOP COLLABORATOR ROUTES ============
		r.Route("/shops/{shop_id}/collaborators", func(r chi.Router) {
			r.Use(middl.RequireRoles("merchant", "super_admin"))
			collaboratorHandler.RegisterShopRoutes(r)
		})

		// ============ 🆕 v4.4.0 : 2FA ROUTES ============
		r.Route("/admin/2fa", func(r chi.Router) {
			r.Use(middl.RequireRoles("super_admin", "admin"))
			twoFAHandler.RegisterRoutes(r)
		})

		// ============ 🆕 v4.4.2 : SESSION MANAGEMENT ROUTES ============
		r.Route("/admin/sessions", func(r chi.Router) {
			r.Use(middl.RequireRoles("super_admin", "admin"))
			sessionHandler.RegisterRoutes(r)
		})

		// ============ 🆕 v4.4.3 : API KEY MANAGEMENT ROUTES ============
		r.Route("/admin/api-keys", func(r chi.Router) {
			r.Use(middl.RequireRoles("super_admin", "admin"))
			apiKeyHandler.RegisterRoutes(r)
		})

		// ============ 🆕 v4.6.0 : ADMIN DISPUTE ROUTES ============
		r.Route("/admin", func(r chi.Router) {
			r.Use(middl.RequireRoles("super_admin", "admin"))

			// Résoudre un litige par dispute_id
			r.Post("/disputes/{id}/resolve", middl.ErrorHandler(disputeHandler.ResolveDispute))

			// 🆕 Résoudre un litige par order_id (utilisé par le script E2E)
			r.Post("/orders/{id}/dispute/resolve", middl.ErrorHandler(disputeHandler.ResolveOrderByDispute))
		})
	})

	// ============ 🆕 v4.0.0 : INITIALISATION DU CRON SCHEDULER ============
	cronSchedule := os.Getenv("COMMISSION_SCHEDULE")
	if cronSchedule == "" {
		cronSchedule = "0 2 * * *"
	}

	onlinePaymentSchedule := os.Getenv("ONLINE_PAYMENT_SCHEDULE")
	if onlinePaymentSchedule == "" {
		onlinePaymentSchedule = "0 */1 * * *"
	}

	tontineSchedule := os.Getenv("TONTINE_SCHEDULE")
	if tontineSchedule == "" {
		tontineSchedule = "*/30 * * * *"
	}

	creditSchedule := os.Getenv("CREDIT_SCHEDULE")
	if creditSchedule == "" {
		creditSchedule = "0 3 * * *"
	}

	a.Scheduler = infscheduler.NewCronScheduler(
		commissionSched,
		onlinePaymentSched,
		tontineSched,
		creditSched,
		a.Logger.Logger,
		cronSchedule,
		onlinePaymentSchedule,
		tontineSchedule,
		creditSchedule,
	)

	if err := a.Scheduler.Start(); err != nil {
		a.Logger.Error().Err(err).Msg("❌ Failed to start scheduler")
	} else {
		a.Logger.Info().
			Str("cod_schedule", cronSchedule).
			Str("online_payment_schedule", onlinePaymentSchedule).
			Str("tontine_schedule", tontineSchedule).
			Str("credit_schedule", creditSchedule).
			Msg("✅ v4.6.0 Commission schedulers started")
	}

	a.Router = r

	duration := time.Since(startTime)
	a.Logger.Info().
		Dur("setup_duration_ms", duration).
		Msg("✅ Router configuré avec succès (v4.6.0: + Dispute System + CORS + Proxy IP Fix + Client Route Separation)")
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
		Version:     "4.6.0", // 🆕 v4.6.0
		LogLevel:    "warn",
	}
	logger := setupLogging.NewLogger(loggingConfig)
	app := NewApp(db, logger)
	return app.Router
}
