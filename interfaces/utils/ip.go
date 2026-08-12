package utils

import (
	"net"
	"net/http"
	"os"
	"strings"
	"sync"
)

// ============================================================
// EXTRACTION D'IP COMPATIBLE PROXY (hardened)
// ============================================================
//
// PROBLÈME :
// Derrière un reverse proxy, RemoteAddr = IP du proxy.
// Sans garde-fou, un client peut spoof X-Forwarded-For / X-Real-IP
// et contourner rate-limit / audit.
//
// RÈGLE :
// On ne lit X-Forwarded-For / X-Real-IP QUE si RemoteAddr
// appartient à TRUSTED_PROXIES (CIDR ou IP, séparés par des virgules).
//
// Exemple .env :
//   TRUSTED_PROXIES=10.0.0.0/8,192.168.0.0/16,127.0.0.1,::1
//
// Si TRUSTED_PROXIES est vide → on ignore les headers (sécurisé par défaut).
// ============================================================

var (
	trustedOnce  sync.Once
	trustedNets  []*net.IPNet
	trustedExact map[string]struct{}
)

func loadTrustedProxies() {
	trustedOnce.Do(func() {
		trustedExact = make(map[string]struct{})
		raw := strings.TrimSpace(os.Getenv("TRUSTED_PROXIES"))
		if raw == "" {
			return
		}
		for _, part := range strings.Split(raw, ",") {
			part = strings.TrimSpace(part)
			if part == "" {
				continue
			}
			if strings.Contains(part, "/") {
				_, network, err := net.ParseCIDR(part)
				if err == nil && network != nil {
					trustedNets = append(trustedNets, network)
				}
				continue
			}
			ip := net.ParseIP(part)
			if ip != nil {
				trustedExact[ip.String()] = struct{}{}
			}
		}
	})
}

func isTrustedProxy(remoteAddr string) bool {
	loadTrustedProxies()
	if len(trustedNets) == 0 && len(trustedExact) == 0 {
		return false
	}

	host := remoteAddr
	if h, _, err := net.SplitHostPort(remoteAddr); err == nil {
		host = h
	}
	ip := net.ParseIP(host)
	if ip == nil {
		return false
	}

	if _, ok := trustedExact[ip.String()]; ok {
		return true
	}
	for _, n := range trustedNets {
		if n.Contains(ip) {
			return true
		}
	}
	return false
}

func remoteIP(r *http.Request) string {
	ip, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return ip
}

// GetClientIP extrait l'IP client.
// Headers X-Forwarded-For / X-Real-IP uniquement si le hop immédiat est un proxy de confiance.
func GetClientIP(r *http.Request) string {
	if isTrustedProxy(r.RemoteAddr) {
		// 1. X-Forwarded-For : première IP = client d'origine (chaîne proxy)
		if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
			parts := strings.Split(xff, ",")
			clientIP := strings.TrimSpace(parts[0])
			if net.ParseIP(clientIP) != nil {
				return clientIP
			}
		}
		// 2. X-Real-IP (Nginx)
		if xri := strings.TrimSpace(r.Header.Get("X-Real-IP")); xri != "" {
			if net.ParseIP(xri) != nil {
				return xri
			}
		}
	}

	// 3. Fallback : connexion directe
	return remoteIP(r)
}
