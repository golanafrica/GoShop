// application/metrics/registry.go
// application/metrics/registry.go
package metrics

import (
	"sync"

	"github.com/prometheus/client_golang/prometheus"
)

var registerOnce sync.Once

// RegisterMetrics enregistre toutes les métriques (UNE SEULE FOIS)
func RegisterMetrics() {
	registerOnce.Do(func() {
		// Authentification
		prometheus.MustRegister(AuthLoginTotal)
		prometheus.MustRegister(AuthLoginFailedTotal)
		prometheus.MustRegister(AuthRegisterTotal)

		// Histogrammes auth
		prometheus.MustRegister(AuthLoginDuration)
		prometheus.MustRegister(AuthRegisterDuration)
		prometheus.MustRegister(AuthProfileDuration)

		// Produits
		prometheus.MustRegister(ProductsCreatedTotal)
		prometheus.MustRegister(ProductsCreateDuration)
		prometheus.MustRegister(ProductsGetDuration)
		prometheus.MustRegister(ProductsListDuration)

		// Commandes
		prometheus.MustRegister(OrdersCreatedTotal)
		prometheus.MustRegister(OrdersRevenueCentsTotal)
		prometheus.MustRegister(OrdersCreateDuration)
		prometheus.MustRegister(OrdersGetDuration)
		prometheus.MustRegister(OrdersListDuration)

		// HTTP
		prometheus.MustRegister(HTTPRequestDuration)
	})
}
