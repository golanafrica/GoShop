package utils

import (
	"fmt"
	"strings"
)

// ============================================================
// HELPERS MONÉTAIRES - CONVENTION CENTIMES
// ============================================================
//
// Convention :
// - Stockage : amount_cents (centimes, entiers)
// - Calcul : Toujours en centimes (pas de floats)
// - Affichage : Division par 100 pour FCFA avec arrondi
// - Exemple : 5000000 centimes = 50 000 FCFA
//
// Arrondi :
// - Standard : (cents + 50) / 100
// - Exemple : 5499 centimes → 55 FCFA
// - Exemple : 5400 centimes → 54 FCFA
// - Exemple : 5450 centimes → 55 FCFA
// ============================================================

// FormatMoney formate un montant en centimes pour affichage FCFA avec arrondi
// Exemple : 5499 centimes → "55 FCFA"
// Exemple : 5000000 centimes → "50 000 FCFA"
func FormatMoney(cents int64) string {
	if cents < 0 {
		return "-" + FormatMoney(-cents)
	}
	fcfa := (cents + 50) / 100 // Arrondi au lieu de troncature
	return formatNumber(fcfa) + " FCFA"
}

// FormatMoneyNoRound formate sans arrondi (troncature)
// Exemple : 5499 centimes → "54 FCFA"
func FormatMoneyNoRound(cents int64) string {
	if cents < 0 {
		return "-" + FormatMoneyNoRound(-cents)
	}
	fcfa := cents / 100
	return formatNumber(fcfa) + " FCFA"
}

// FormatMoneyWithDecimals formate avec 2 décimales (pour debug)
// Exemple : 5499 centimes → "54.99 FCFA"
func FormatMoneyWithDecimals(cents int64) string {
	fcfa := float64(cents) / 100.0
	return fmt.Sprintf("%.2f FCFA", fcfa)
}

// RoundMoney arrondit un montant en centimes au FCFA le plus proche
// Retourne le montant en centimes arrondi
// Exemple : 5499 centimes → 5500 centimes
func RoundMoney(cents int64) int64 {
	return ((cents + 50) / 100) * 100
}

// CentsToFCFA convertit des centimes en FCFA avec arrondi
// Exemple : 5499 centimes → 55 FCFA
func CentsToFCFA(cents int64) int64 {
	return (cents + 50) / 100
}

// FCFAtoCents convertit des FCFA en centimes
// Exemple : 55 FCFA → 5500 centimes
func FCFAtoCents(fcfa int64) int64 {
	return fcfa * 100
}

// FormatCommission formate spécifiquement une commission
// Exemple : 1833 centimes → "18 FCFA"
func FormatCommission(cents int64) string {
	return FormatMoney(cents)
}

// FormatBalance formate un solde (peut être négatif)
// Exemple : -45000 centimes → "-450 FCFA"
func FormatBalance(cents int64) string {
	return FormatMoney(cents)
}

// ============================================================
// HELPERS INTERNES
// ============================================================

// formatNumber formate un nombre avec séparateurs de milliers (convention française)
// Exemple : 50000 → "50 000"
// Exemple : 1234567 → "1 234 567"
func formatNumber(n int64) string {
	if n < 0 {
		return "-" + formatNumber(-n)
	}

	str := fmt.Sprintf("%d", n)

	// Pas besoin de séparateurs pour les petits nombres
	if n < 1000 {
		return str
	}

	// Ajouter les séparateurs d'espace (convention française)
	var result strings.Builder
	count := 0

	for i := len(str) - 1; i >= 0; i-- {
		if count > 0 && count%3 == 0 {
			result.WriteString(" ")
		}
		result.WriteByte(str[i])
		count++
	}

	// Inverser la chaîne
	reversed := result.String()
	runes := []rune(reversed)
	for i, j := 0, len(runes)-1; i < j; i, j = i+1, j-1 {
		runes[i], runes[j] = runes[j], runes[i]
	}

	return string(runes)
}
