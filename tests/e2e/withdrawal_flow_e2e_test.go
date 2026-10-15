package e2e

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"
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

	actualSlug := shopResp["slug"].(string)
	shopID := shopResp["id"].(string)
	t.Logf("✅ Boutique créée: %s (slug: %s)", shopID, actualSlug)

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
	// ÉTAPE 2 : Soumission KYC (paths serveur J3 + fichiers réels PDF)
	// =========================================================================
	t.Log("\n📋 ÉTAPE 2 : Soumission du KYC par le marchand")

	dir := filepath.Join("uploads", "kyc", "merchant", shopID)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir kyc: %v", err)
	}

	pdfMinimal := []byte("%PDF-1.4\n1 0 obj<<>>endobj\ntrailer<<>>\n%%EOF\n")
	idPath := filepath.Join(dir, "id_card.pdf")
	regPath := filepath.Join(dir, "registry.pdf")
	if err := os.WriteFile(idPath, pdfMinimal, 0o644); err != nil {
		t.Fatalf("write id pdf: %v", err)
	}
	if err := os.WriteFile(regPath, pdfMinimal, 0o644); err != nil {
		t.Fatalf("write registry pdf: %v", err)
	}

	idRel := filepath.ToSlash(filepath.Join("uploads", "kyc", "merchant", shopID, "id_card.pdf"))
	regRel := filepath.ToSlash(filepath.Join("uploads", "kyc", "merchant", shopID, "registry.pdf"))

	kycData := map[string]interface{}{
		"documents": []map[string]interface{}{
			{
				"document_type":   "identity_card",
				"file_path":       idRel,
				"file_name":       "id_card.pdf",
				"file_size_bytes": len(pdfMinimal),
				"mime_type":       "application/pdf",
			},
			{
				"document_type":   "business_registry",
				"file_path":       regRel,
				"file_name":       "registry.pdf",
				"file_size_bytes": len(pdfMinimal),
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
		"amount_cents":       0,
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
