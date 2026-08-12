package e2e

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"

	"Goshop/tests/testutils"
)

// ============================================================================
// 🧪 TEST E2E : Workflow complet Tontine Cycle Completion + Webhooks
// ============================================================================

func TestTontineCycleCompletionE2E(t *testing.T) {
	t.Log(strings.Repeat("=", 70))
	t.Log("🏦 E2E TEST : Workflow complet Tontine Cycle Completion + Webhooks")
	t.Log(strings.Repeat("=", 70))

	// ⚠️ Nécessaire pour que ValidateWebhook accepte la signature qu'on va calculer.
	// Doit être défini AVANT le démarrage du TestServer (app.go lit os.Getenv une seule fois à l'init).
	webhookSecret := os.Getenv("YENGA_PAY_WEBHOOK_SECRET")
	if webhookSecret == "" {
		webhookSecret = "test-webhook-secret-e2e"
		t.Setenv("YENGA_PAY_WEBHOOK_SECRET", webhookSecret)
	}

	server := testutils.NewTestServer(t)
	client := testutils.NewHTTPClient(server.URL)

	// ========================================================================
	// ÉTAPE 0 : Authentification Marchand
	// ========================================================================
	t.Log("\n📋 ÉTAPE 0 : Authentification Marchand")
	uniqueEmail := fmt.Sprintf("merchant.tontine.%d@example.com", time.Now().UnixNano())
	userData := map[string]interface{}{
		"email":    uniqueEmail,
		"password": "Password123!",
	}
	resp := client.MustDoRequest(t, "POST", "/register", userData)
	resp.Body.Close()

	loginData := map[string]interface{}{
		"email":    uniqueEmail,
		"password": "Password123!",
	}
	resp = client.MustDoRequest(t, "POST", "/login", loginData)
	var loginResp struct {
		Token string `json:"token"`
	}
	testutils.ParseJSONBody(t, resp, &loginResp)
	resp.Body.Close()
	client.SetToken(loginResp.Token)
	t.Log("✅ Authentification Marchand réussie")

	// ========================================================================
	// ÉTAPE 1 : Création du shop et du produit
	// ========================================================================
	t.Log("\n📋 ÉTAPE 1 : Création du shop et du produit")
	shopSlug := fmt.Sprintf("tontine-e2e-%d", time.Now().UnixNano())
	shopData := map[string]interface{}{
		"name": "Tontine E2E Shop",
		"slug": shopSlug,
	}
	resp = client.MustDoRequest(t, "POST", "/api/shops", shopData)
	resp.Body.Close()

	client.SetDefaultHeader("X-Shop-Slug", shopSlug)

	resp = client.MustDoRequest(t, "GET", "/api/shops", nil)
	var shops []map[string]interface{}
	testutils.ParseJSONBody(t, resp, &shops)
	resp.Body.Close()

	var shopID string
	for _, s := range shops {
		if s["slug"] == shopSlug {
			shopID = s["id"].(string)
			break
		}
	}

	productReq := map[string]interface{}{
		"name":        "Produit Tontine E2E",
		"description": "Produit pour test cycle completion",
		"price_cents": 3000000, // 30 000 FCFA
		"stock":       10,
	}
	resp = client.MustDoRequest(t, "POST", "/api/products", productReq)
	productID := testutils.ExtractID(t, resp)
	resp.Body.Close()
	t.Logf("✅ Shop (%s) et Produit (%s) créés", shopID, productID)

	// ========================================================================
	// ÉTAPE 2 : Activer la tontine sur le produit
	// ========================================================================
	t.Log("\n📋 ÉTAPE 2 : Activation de la tontine sur le produit")
	tontineSettings := map[string]interface{}{
		"product_id":              productID,
		"is_tontine_enabled":      true,
		"allow_commercial_circle": true,
		"allow_corporate_circle":  true,
		"allow_family_circle":     true,
		"min_participants":        3,
		"max_participants":        3,
	}
	resp = client.MustDoRequest(t, "PUT", "/api/shops/"+shopID+"/tontine-settings", tontineSettings)
	resp.Body.Close()
	testutils.AssertStatus(t, resp, http.StatusOK)
	t.Log("✅ Tontine activée sur le produit (3 participants requis)")

	// ========================================================================
	// ÉTAPE 3 : Créer 3 clients KYC vérifiés
	// ========================================================================
	t.Log("\n📋 ÉTAPE 3 : Création de 3 clients KYC vérifiés")

	merchantClient := client
	customers := make([]string, 3)
	clientTokens := make([]string, 3)

	for i := 0; i < 3; i++ {
		customers[i], clientTokens[i] = createVerifiedTontineCustomer(t, server.URL, shopSlug, merchantClient, i)
		t.Logf("  ✅ Client %d créé et KYC vérifié: %s", i+1, customers[i])
	}

	// ========================================================================
	// ÉTAPE 4 : Client 1 crée un groupe de 3
	// ========================================================================
	t.Log("\n📋 ÉTAPE 4 : Création du groupe tontine par le client 1")

	client1 := testutils.NewHTTPClient(server.URL)
	client1.SetToken(clientTokens[0])
	client1.SetDefaultHeader("X-Shop-Slug", shopSlug)

	groupBody := map[string]interface{}{
		"product_id":          productID,
		"creator_customer_id": customers[0],
		"circle_type":         "FAMILY",
		"total_cycles":        3,
	}
	resp = client1.MustDoRequest(t, "POST", "/api/tontine/groups", groupBody)
	testutils.AssertStatus(t, resp, http.StatusCreated)

	var group map[string]interface{}
	testutils.ParseJSONBody(t, resp, &group)
	resp.Body.Close()

	groupID := group["id"].(string)
	inviteCode := group["invite_code"].(string)
	t.Logf("✅ Groupe créé: %s (Invite Code: %s)", groupID, inviteCode)

	// ========================================================================
	// ÉTAPE 5 : Clients 2 et 3 rejoignent le groupe
	// ========================================================================
	t.Log("\n📋 ÉTAPE 5 : Les clients 2 et 3 rejoignent le groupe")
	clients := make([]*testutils.HTTPClient, 3)
	clients[0] = client1

	for i := 1; i < 3; i++ {
		clientN := testutils.NewHTTPClient(server.URL)
		clientN.SetToken(clientTokens[i])
		clientN.SetDefaultHeader("X-Shop-Slug", shopSlug)
		clients[i] = clientN

		joinBody := map[string]interface{}{
			"invite_code": inviteCode,
			"customer_id": customers[i],
		}
		resp = clientN.MustDoRequest(t, "POST", "/api/tontine/groups/join", joinBody)
		testutils.AssertStatus(t, resp, http.StatusOK)
		resp.Body.Close()
		t.Logf("  ✅ Client %d a rejoint le groupe", i+1)
	}

	// ========================================================================
	// ÉTAPE 6 : Chaque client initie son paiement réel (cycle 1)
	// ========================================================================
	// ⚠️ On DOIT passer par le vrai endpoint /pay : c'est lui qui crée
	// l'enregistrement TontinePayment et génère la référence YengaPay
	// (TONTINE:{groupID[:8]}:{cycle}:{participantID[:8]}) que le webhook
	// devra ensuite retrouver. Un webhook simulé pour une référence
	// jamais créée ne peut jamais aboutir.
	t.Log("\n📋 ÉTAPE 6 : Initiation des 3 paiements réels (Cycle 1)")

	providerRefs := make([]string, 3)
	for i, c := range clients {
		payBody := map[string]interface{}{
			"customer_id":  customers[i],
			"operator":     "orange_money",
			"phone_number": "+22670123456",
			"flow":         "indirect",
		}
		resp = c.MustDoRequest(t, "POST", fmt.Sprintf("/api/tontine/groups/%s/pay", groupID), payBody)
		testutils.AssertStatus(t, resp, http.StatusOK)

		var payResp map[string]interface{}
		testutils.ParseJSONBody(t, resp, &payResp)
		resp.Body.Close()

		providerRefs[i], _ = payResp["provider_ref"].(string)
		if providerRefs[i] == "" {
			t.Fatalf("❌ provider_ref manquant dans la réponse /pay pour le client %d: %+v", i+1, payResp)
		}
		t.Logf("  ✅ Client %d a initié son paiement (provider_ref: %s)", i+1, providerRefs[i])
	}

	// ========================================================================
	// ÉTAPE 7 : Simulation des 3 webhooks YengaPay (paiement confirmé)
	// ========================================================================
	t.Log("\n📋 ÉTAPE 7 : Simulation des 3 webhooks de confirmation de paiement")

	for i := 0; i < 3; i++ {
		txnID := fmt.Sprintf("TXN-E2E-%d-%d", time.Now().UnixNano(), i)

		// Format réel attendu par YengaPayProvider.ValidateWebhook
		webhookPayload := map[string]interface{}{
			"apiEnv":          "test",
			"paymentStatus":   "SUCCESS",
			"transId":         txnID,
			"projectId":       "e2e-project",
			"paymentIntentId": txnID,
			"paymentSource":   "orange_money",
			"customerNumber":  "+22670123456",
			"paymentAmount":   10000, // en unité "principale" (FCFA), le provider multiplie *100
			"paymentFees":     0,
			"contryOrigin":    "BF",
			"reference":       providerRefs[i],
			"currency":        "XOF",
		}

		reqBody, err := json.Marshal(webhookPayload)
		if err != nil {
			t.Fatalf("❌ Failed to marshal webhook payload: %v", err)
		}

		signature := computeYengaWebhookSignature(webhookSecret, reqBody)

		req, err := http.NewRequest("POST", server.URL+"/webhooks/yenga_pay", strings.NewReader(string(reqBody)))
		if err != nil {
			t.Fatalf("❌ Failed to build webhook request: %v", err)
		}
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("x-webhook-hash", signature)

		httpResp, err := client.DoRequestRaw(req)
		if err != nil {
			t.Fatalf("❌ Webhook %d error: %v", i+1, err)
		}
		body := testutils.ReadBody(t, httpResp)
		httpResp.Body.Close()

		if httpResp.StatusCode != http.StatusOK {
			t.Fatalf("❌ Webhook %d a échoué (HTTP %d): %s", i+1, httpResp.StatusCode, body)
		}
		t.Logf("  ✅ Webhook %d accepté (HTTP %d)", i+1, httpResp.StatusCode)
	}

	// ========================================================================
	// ÉTAPE 8 : Vérification finale (via API)
	// ========================================================================
	t.Log("\n📋 ÉTAPE 8 : Vérification que les paiements sont bien marqués DONE")

	resp = client1.MustDoRequest(t, "GET", fmt.Sprintf("/api/tontine/groups/%s/payments?customer_id=%s", groupID, customers[0]), nil)
	testutils.AssertStatus(t, resp, http.StatusOK)

	var payments []map[string]interface{}
	testutils.ParseJSONBody(t, resp, &payments)
	resp.Body.Close()

	t.Logf("✅ Paiements enregistrés pour le client 1: %d", len(payments))
	if len(payments) == 0 {
		t.Fatal("❌ Aucun paiement enregistré : le webhook n'a pas été traité correctement")
	}

	allDone := true
	for _, p := range payments {
		if p["status"] != "DONE" {
			allDone = false
		}
	}
	if !allDone {
		t.Fatal("❌ Au moins un paiement n'est pas au statut DONE après le webhook")
	}
	t.Log("✅ Tous les paiements du client 1 sont DONE")

	// ========================================================================
	// ÉTAPE 8.5 : Diagnostic direct en base (contourne le niveau de log "warn"
	// qui masque tous les logger.Info() de checkAndCompleteCycle)
	// ========================================================================
	t.Log("\n📋 ÉTAPE 8.5 : Diagnostic direct en base de données")

	var dbCurrentCycle int
	var dbStatus string
	dbErr := server.DB.QueryRow(
		"SELECT current_cycle, status FROM tontine_groups WHERE id = $1", groupID,
	).Scan(&dbCurrentCycle, &dbStatus)
	if dbErr != nil {
		t.Fatalf("❌ Impossible de lire le groupe en base: %v", dbErr)
	}
	t.Logf("   tontine_groups: current_cycle=%d, status=%s", dbCurrentCycle, dbStatus)

	var voucherCount int
	dbErr = server.DB.QueryRow(
		"SELECT COUNT(*) FROM tontine_vouchers WHERE group_id = $1", groupID,
	).Scan(&voucherCount)
	if dbErr != nil {
		t.Fatalf("❌ Impossible de compter les vouchers en base: %v", dbErr)
	}
	t.Logf("   tontine_vouchers: count=%d", voucherCount)

	var doneCount int
	dbErr = server.DB.QueryRow(
		"SELECT COUNT(*) FROM tontine_payments WHERE group_id = $1 AND cycle_number = 1 AND status = 'DONE'", groupID,
	).Scan(&doneCount)
	if dbErr != nil {
		t.Fatalf("❌ Impossible de compter les paiements DONE en base: %v", dbErr)
	}
	t.Logf("   tontine_payments DONE (cycle 1): %d / 3", doneCount)

	if voucherCount == 0 {
		t.Fatal("❌ Aucun voucher créé : checkAndCompleteCycle n'a jamais complété le cycle. " +
			"Le problème n'est PAS le crédit wallet, il est en amont (doneCount < TotalCycles, " +
			"ou group.IsActive() == false, ou une erreur dans FindByPosition/NewTontineVoucher).")
	}
	t.Log("✅ Voucher créé — checkAndCompleteCycle a bien atteint l'étape de génération du voucher")

	// ========================================================================
	// ÉTAPE 9 : Vérification que le wallet marchand a été crédité
	// ========================================================================
	// C'est LA vérification qui manquait : ÉTAPE 8 ne prouve que la
	// transition de statut du paiement, pas que checkAndCompleteCycle a
	// réellement exécuté CreditFromTontine (qui est best-effort et n'aurait
	// pas fait échouer le webhook même en cas d'erreur).
	t.Log("\n📋 ÉTAPE 9 : Vérification du crédit du wallet marchand")

	resp = client.MustDoRequest(t, "GET", "/api/wallet", nil)
	testutils.AssertStatus(t, resp, http.StatusOK)

	var wallet struct {
		ShopID       string `json:"shop_id"`
		BalanceCents int64  `json:"balance_cents"`
	}
	testutils.ParseJSONBody(t, resp, &wallet)
	resp.Body.Close()

	// price_cents(3 000 000) / total_cycles(3) = amount_per_cycle(1 000 000)
	// group.TotalAmountCents() = amount_per_cycle * total_cycles = 3 000 000
	expectedBalanceCents := int64(3000000)
	t.Logf("   Solde wallet: %d cents (attendu: %d cents)", wallet.BalanceCents, expectedBalanceCents)
	if wallet.BalanceCents != expectedBalanceCents {
		t.Fatalf("❌ Wallet non crédité correctement : attendu %d cents, obtenu %d cents. "+
			"checkAndCompleteCycle n'a probablement pas appelé CreditFromTontine avec succès.",
			expectedBalanceCents, wallet.BalanceCents)
	}
	t.Log("✅ Wallet marchand crédité du bon montant — CreditFromTontine a bien été exécuté")

	t.Log("\n" + strings.Repeat("=", 70))
	t.Log("🎉 TEST E2E TONTINE CYCLE COMPLETION TERMINÉ !")
	t.Log(strings.Repeat("=", 70))
}

// computeYengaWebhookSignature calcule la signature HMAC-SHA256 attendue par
// YengaPayProvider.ValidateWebhook (infrastructure/payment/yenga_pay_provider.go).
func computeYengaWebhookSignature(secret string, payload []byte) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(payload)
	return hex.EncodeToString(mac.Sum(nil))
}

// ============================================================================
// 🔧 HELPER : Créer un client KYC vérifié (retourne customerID et Token)
// ============================================================================

func createVerifiedTontineCustomer(t *testing.T, serverURL, shopSlug string, merchantClient *testutils.HTTPClient, index int) (customerID string, token string) {
	t.Helper()

	client := testutils.NewHTTPClient(serverURL)
	client.SetDefaultHeader("X-Shop-Slug", shopSlug)

	uniqueEmail := fmt.Sprintf("client%d.tontine.%d@example.com", index+1, time.Now().UnixNano())

	userData := map[string]interface{}{
		"email":    uniqueEmail,
		"password": "Password123!",
	}
	resp := client.MustDoRequest(t, "POST", "/register", userData)
	resp.Body.Close()

	loginData := map[string]interface{}{
		"email":    uniqueEmail,
		"password": "Password123!",
	}
	resp = client.MustDoRequest(t, "POST", "/login", loginData)
	var loginResp struct {
		Token string `json:"token"`
	}
	testutils.ParseJSONBody(t, resp, &loginResp)
	resp.Body.Close()
	token = loginResp.Token
	client.SetToken(token)

	customerBody := map[string]interface{}{
		"first_name": fmt.Sprintf("Client%d", index+1),
		"last_name":  "Tontine",
		"email":      uniqueEmail,
	}
	resp = client.MustDoRequest(t, "POST", "/api/customers", customerBody)
	customerID = testutils.ExtractID(t, resp)
	resp.Body.Close()

	kycBody := map[string]interface{}{
		"customer_id":     customerID,
		"document_type":   "cni",
		"file_path":       fmt.Sprintf("/uploads/kyc/%s/cni.jpg", customerID),
		"file_size_bytes": 1024,
		"mime_type":       "image/jpeg",
	}
	resp = client.MustDoRequest(t, "POST", "/api/customers/kyc/upload", kycBody)
	resp.Body.Close()
	testutils.AssertStatus(t, resp, http.StatusCreated)

	reviewBody := map[string]interface{}{
		"action": "approve",
	}
	resp = merchantClient.MustDoRequest(t, "POST", fmt.Sprintf("/api/merchant/kyc/%s/review", customerID), reviewBody)
	resp.Body.Close()
	testutils.AssertStatus(t, resp, http.StatusOK)

	return customerID, token
}
