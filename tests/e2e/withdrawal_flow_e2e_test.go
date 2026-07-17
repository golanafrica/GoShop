package e2e

import (
	"fmt"
	"net/http"
	"testing"
	"time"

	"Goshop/tests/testutils"
)

func TestWithdrawalFlowE2E(t *testing.T) {
	t.Log("======================================================================")
	t.Log("💸 E2E TEST : Flux de retrait marchand (Vérification KYC & Validation)")
	t.Log("======================================================================")

	server := testutils.NewTestServer(t)
	client := testutils.NewHTTPClient(server.URL)

	// =========================================================================
	// ÉTAPE 0 : Authentification et création de la boutique
	// =========================================================================
	t.Log("\n📋 ÉTAPE 0 : Authentification et création de la boutique")
	merchantEmail := fmt.Sprintf("merchant_withdraw_%d@example.com", time.Now().UnixNano())

	resp := client.MustDoRequest(t, "POST", "/register", map[string]interface{}{
		"email":    merchantEmail,
		"password": "Password123!",
	})
	testutils.AssertStatus(t, resp, http.StatusCreated)

	resp = client.MustDoRequest(t, "POST", "/login", map[string]interface{}{
		"email":    merchantEmail,
		"password": "Password123!",
	})
	testutils.AssertStatus(t, resp, http.StatusOK)

	var loginResp map[string]interface{}
	testutils.ParseJSONBody(t, resp, &loginResp)
	client.SetToken(loginResp["token"].(string))

	shopSlug := fmt.Sprintf("withdraw-shop-%d", time.Now().UnixNano())
	resp = client.MustDoRequest(t, "POST", "/api/shops", map[string]interface{}{
		"name": "Boutique Retrait Test",
		"slug": shopSlug,
	})
	testutils.AssertStatus(t, resp, http.StatusCreated)

	var shopResp map[string]interface{}
	testutils.ParseJSONBody(t, resp, &shopResp)

	// ✅ Récupérer le slug exact depuis la réponse JSON
	actualSlug := shopResp["slug"].(string)
	t.Logf("✅ Boutique créée: %s (slug: %s)", shopResp["id"].(string), actualSlug)

	// Définir le header pour TOUTES les requêtes multi-tenant suivantes
	client.SetDefaultHeader("X-Shop-Slug", actualSlug)

	// =========================================================================
	// ÉTAPE 1 : Tentative de retrait sans KYC (Doit échouer avec 400)
	// =========================================================================
	t.Log("\n📋 ÉTAPE 1 : Tentative de retrait sans KYC (Doit échouer)")
	withdrawData := map[string]interface{}{
		"amount_cents":       50000,
		"payment_method":     "ORANGE_MONEY",
		"destination_number": "70000000",
	}

	resp = client.MustDoRequest(t, "POST", "/api/withdrawals", withdrawData)
	testutils.AssertStatus(t, resp, http.StatusBadRequest)

	var errResp map[string]interface{}
	testutils.ParseJSONBody(t, resp, &errResp)
	t.Logf("✅ Retrait correctement bloqué : %v", errResp["message"])

	// =========================================================================
	// ÉTAPE 2 : Soumission du KYC par le marchand
	// =========================================================================
	t.Log("\n📋 ÉTAPE 2 : Soumission du KYC par le marchand")

	// ✅ CORRECTION : Ajout du document d'identité obligatoire (identity_card ou passport)
	kycData := map[string]interface{}{
		"documents": []map[string]interface{}{
			{
				"document_type":   "identity_card", // ✅ Document d'identité obligatoire
				"file_path":       "/uploads/kyc/test_id.pdf",
				"file_name":       "id_card.pdf",
				"file_size_bytes": 102400,
				"mime_type":       "application/pdf",
			},
			{
				"document_type":   "business_registry", // ✅ Registre de commerce
				"file_path":       "/uploads/kyc/test_registry.pdf",
				"file_name":       "registry.pdf",
				"file_size_bytes": 102400,
				"mime_type":       "application/pdf",
			},
		},
	}

	resp = client.MustDoRequest(t, "POST", "/api/merchant-kyc/submit", kycData)
	testutils.AssertStatus(t, resp, http.StatusOK)
	t.Log("✅ Documents KYC soumis avec succès (statut passé à: pending)")

	// =========================================================================
	// ÉTAPE 3 : Tentative de retrait avec KYC PENDING (Doit encore échouer)
	// =========================================================================
	t.Log("\n📋 ÉTAPE 3 : Tentative de retrait avec KYC PENDING (Doit encore échouer)")
	resp = client.MustDoRequest(t, "POST", "/api/withdrawals", withdrawData)
	testutils.AssertStatus(t, resp, http.StatusBadRequest)
	t.Log("✅ Retrait correctement bloqué : KYC en attente de vérification")

	// =========================================================================
	// ÉTAPE 4 : Tentative de retrait avec montant invalide (Validation)
	// =========================================================================
	t.Log("\n📋 ÉTAPE 4 : Tentative de retrait avec montant invalide (Validation)")
	invalidWithdrawData := map[string]interface{}{
		"amount_cents":       0, // Montant invalide
		"payment_method":     "ORANGE_MONEY",
		"destination_number": "70000000",
	}
	resp = client.MustDoRequest(t, "POST", "/api/withdrawals", invalidWithdrawData)
	testutils.AssertStatus(t, resp, http.StatusBadRequest)
	t.Log("✅ Retrait correctement rejeté : montant invalide")

	t.Log("\n======================================================================")
	t.Log("🎉 TOUS LES TESTS DE FLUX DE RETRAIT PASSENT !")
	t.Log("======================================================================")
}
