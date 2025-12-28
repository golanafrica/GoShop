// internal/app/app.go
// internal/app/app.go
package app

import (
	"database/sql"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/rs/zerolog"

	"Goshop/application/metrics"
	authusecase "Goshop/application/usecase/auth_usecase"
	authrefreshrepositoryinfra "Goshop/infrastructure/postgres/auth_refresh_repository_infra"
	"Goshop/infrastructure/postgres/customer"
	"Goshop/infrastructure/postgres/order"
	"Goshop/infrastructure/postgres/product"
	txmanager "Goshop/infrastructure/postgres/tx_manager"
	userpostgres "Goshop/infrastructure/postgres/user_postgres"

	"github.com/prometheus/client_golang/prometheus/promhttp"

	handlers "Goshop/interfaces/handler"
	customerhandler "Goshop/interfaces/handler/customer_handler"
	"Goshop/interfaces/handler/orders"
	productHandler "Goshop/interfaces/handler/product"
	refreshhandler "Goshop/interfaces/handler/refresh_handler"
	userhandler "Goshop/interfaces/handler/user_handler"
	middleware "Goshop/interfaces/middl/user_middleware"

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
	//a.Logger.Info().Msg("Configuration du router...")

	r := chi.NewRouter()

	// ============ 1. MIDDLEWARES GLOBAUX ============

	r.Use(middl.LoggerInitMiddleware(a.Logger)) // ← 1er: initialise le logger de base
	r.Use(middl.RequestIDMiddleware)
	r.Use(middl.HTTPMetricsMiddleware)   // ← 2ème: enrichit avec request_id
	r.Use(middl.LoginAuditMiddleware)    // ← 3ème: audit login
	r.Use(middl.RequestLoggerMiddleware) // ← 4ème: logue la requête
	r.Use(middl.Recovery)
	r.Use(middl.SecureHeaders)

	// ============ 2. INITIALISATION ============
	hh := handlers.HealthHandler{
		DB:     a.DB,
		Rdb:    utils.Rdb,
		Logger: a.Logger.WithComponent("health_handler"),
	}

	// -- Repositories
	txmanagerRepo := txmanager.NewTxManagerPostgresInfra(a.DB)
	postgreProductRepo := product.NewProductRepositoryInfrastructure(a.DB)
	postgresCustomerRepo := customer.NewCustomerRepoInfrastructurePostgres(a.DB)
	postgresOrderRepo := order.NewOrderPostgresInfra(a.DB)
	postgresOrderItem := order.NewOrderItemPostgresInfra(a.DB)
	postgresUserRepo := userpostgres.NewUserPostgres(a.DB)
	refreshSessionRepo := authrefreshrepositoryinfra.NewRefreshSessionPostgres(a.DB)

	// -- Usecases
	refreshUsecase := authusecase.NewRefreshUsecase(
		refreshSessionRepo,
		utils.ValidateToken,
		utils.GenerateAccessToken,
		utils.GenerateRefreshToken,
		time.Now,
		uuid.NewString,
		30*24*time.Hour,
	)

	// -- Handlers
	refreshHandler := refreshhandler.NewRefreshHandler(
		refreshUsecase,
	)

	productHandler := productHandler.NewProductHandler(
		postgreProductRepo,
		txmanagerRepo,
	)

	customerHandler := customerhandler.NewCustomerHandler(
		postgresCustomerRepo,
		txmanagerRepo,
	)

	orderHandler := orders.NewOrderHandler(
		a.DB,
		txmanagerRepo,
		postgresOrderRepo,
		postgreProductRepo,
		postgresCustomerRepo,
		postgresOrderItem,
	)

	userHandler := userhandler.NewUserHandler(
		postgresUserRepo,
		a.Logger.WithComponent("user_handler"),
	)

	// ============ 3. ROUTES PUBLIQUES ============
	//r.Use(middl.PrometheusMiddleware)

	// ✅ SUPPRIMEZ withRequestLogging() de toutes les routes !
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

	// ============ 4. ROUTE PROTÉGÉE ============
	r.With(middleware.AuthMiddleware).
		Get("/auth/me", middl.ErrorHandler(userHandler.Me))

	// ============ 5. ROUTES API PROTÉGÉES ============
	r.Route("/api", func(r chi.Router) {
		r.Use(middleware.AuthMiddleware)

		// Products
		r.Route("/products", func(r chi.Router) {
			r.Post("/", middl.ErrorHandler(productHandler.CreateProduct))
			r.Get("/", middl.ErrorHandler(productHandler.GetAllProducts))
			r.Get("/{id}", middl.ErrorHandler(productHandler.GetProductById))
			r.Put("/{id}", middl.ErrorHandler(productHandler.UpdateProduct))
			r.Delete("/{id}", middl.ErrorHandler(productHandler.DeleteProduct))
		})

		// Customers
		r.Route("/customers", func(r chi.Router) {
			r.Post("/", middl.ErrorHandler(customerHandler.CreateCustomerHandler))
			r.Get("/", middl.ErrorHandler(customerHandler.GetAllCustomersHandler))
			r.Get("/{id}", middl.ErrorHandler(customerHandler.GetCustomerByIdHandler))
			r.Put("/{id}", middl.ErrorHandler(customerHandler.UpdateCustomerHandler))
			r.Delete("/{id}", middl.ErrorHandler(customerHandler.DeleteCustomerHandler))
		})

		// Orders
		r.Route("/orders", func(r chi.Router) {
			r.Get("/", middl.ErrorHandler(orderHandler.GetAllOrderHandler))
			r.Post("/", middl.ErrorHandler(orderHandler.CreateOrderHandler))
			r.Get("/{id}", middl.ErrorHandler(orderHandler.GetOrderByIdHandler))
		})
	})

	a.Router = r

	duration := time.Since(startTime)
	a.Logger.Info().
		Dur("setup_duration_ms", duration).
		Msg("✅ Router configuré avec succès")
}

// ============ MIDDLEWARES PERSONNALISÉS ============

// requestLoggingMiddleware avec audit login intégré
// requestLoggingMiddleware avec audit login intégré

// maskEmail masque une partie de l'email pour la confidentialité
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

// determineLogLevel détermine le niveau de log selon le statut et la durée
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

// slowRequestWarning retourne un warning si la requête est lente
func slowRequestWarning(duration time.Duration) string {
	if duration > 2*time.Second {
		return "slow_request"
	}
	return ""
}

// ============ SUPPRIMEZ ou COMMMENTEZ cette fonction ============
/*
// withRequestLogging wrapper pour ajouter du logging aux handlers
// ❌ CETTE FONCTION CAUSE LE DOUBLE LOGGING AVEC CHI
func (a *App) withRequestLogging(operation string, handler http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Cette fonction cause le problème de double logging
		// Utilisez simplement handler(w, r) directement
		handler(w, r)
	}
}
*/

// responseWriter wrapper pour capturer le status et la taille
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

// ============ FONCTION POUR LES TESTS (compatible) ============

// NewRouter crée et retourne un router HTTP configuré (pour les tests)
func NewRouter(db *sql.DB) http.Handler {
	// Crée un logger minimal pour les tests
	loggingConfig := setupLogging.Config{
		Environment: "test",
		ServiceName: "goshop-api-test",
		Version:     "1.0.0",
		LogLevel:    "warn",
	}
	logger := setupLogging.NewLogger(loggingConfig)

	app := NewApp(db, logger)
	return app.Handler()
}
