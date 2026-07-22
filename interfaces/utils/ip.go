package utils

import (
	"net"
	"net/http"
	"strings"
)

// ============================================================
// 🆕 v4.5.1 : EXTRACTION D'IP COMPATIBLE PROXY
// ============================================================
//
// PROBLÈME RÉSOLU :
// Lorsque l'API est derrière un reverse proxy (Nginx, AWS ALB, Traefik, etc.),
// r.RemoteAddr renvoie toujours l'IP du proxy (ex: 10.0.0.5).
// Cela casse le Rate Limiting car TOUS les clients partagent le même quota.
//
// SOLUTION :
// Lire les headers HTTP standards (X-Forwarded-For, X-Real-IP) pour extraire
// la véritable IP du client.
// ============================================================

// GetClientIP extrait la véritable adresse IP du client.
// Ordre de priorité :
//  1. X-Forwarded-For (première IP de la liste)
//  2. X-Real-IP
//  3. RemoteAddr (fallback direct)
func GetClientIP(r *http.Request) string {
	// 1. Vérifier X-Forwarded-For (standard pour les load balancers)
	// Exemple de valeur : "203.0.113.195, 70.41.3.18, 150.172.238.178"
	// La première IP est toujours le client original
	xff := r.Header.Get("X-Forwarded-For")
	if xff != "" {
		ips := strings.Split(xff, ",")
		clientIP := strings.TrimSpace(ips[0])
		if net.ParseIP(clientIP) != nil {
			return clientIP
		}
	}

	// 2. Vérifier X-Real-IP (souvent utilisé par Nginx)
	xri := r.Header.Get("X-Real-IP")
	if xri != "" {
		if net.ParseIP(xri) != nil {
			return xri
		}
	}

	// 3. Fallback sur RemoteAddr (en retirant le port si présent)
	// Exemple : "192.168.1.1:54321" → "192.168.1.1"
	ip, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr // Si pas de port, retourner tel quel
	}

	return ip
}
