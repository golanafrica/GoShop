package e2e

import (
	"crypto/hmac"
	"crypto/sha256"
	"database/sql"
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
// - Timeout client 60s (shop/login lents en local + bcrypt)
// - customers.user_id varchar NOT NULL → rebind user_id d'origine / marchand
// - join JWT = customer marchand → seed SQL participants 2–3 + ACTIVE
// ============================================================================

func TestTontineCycleCompletionE2E(t *testing.T) {
	t.Log(strings.Repeat("=", 70))
	t.Log("🏦 E2E TEST : Workflow complet Tontine Cycle Completion + Webhooks")
	t.Log(strings.Repeat("=", 70))

	webhookSecret := os.Getenv("YENGA_PAY_WEBHOOK_SECRET")
	if webhookSecret == "" {
		webhookSecret = "test-webhook-secret-e2e"
		t.Setenv("YENGA_PAY_WEBHOOK_SECRET", webhookSecret)
	}

	server := testutils.NewTestServer(t)
	client := testutils.NewHTTPClient(server.URL)
	client.Client.Timeout = 60 * time.Second // défaut testutils = 10s trop court

	// ---------- 0. Auth marchand ----------
	t.Log("\n📋 ÉTAPE 0 : Authentification Marchand")
	uniqueEmail := fmt.Sprintf("merchant.tontine.%d@example.com", time.Now().UnixNano())
	resp := client.MustDoRequest(t, "POST", "/register", map[string]interface{}{
		"email": uniqueEmail, "password": "Password123!",
	})
	resp.Body.Close()

	resp = client.MustDoRequest(t, "POST", "/login", map[string]interface{}{
		"email": uniqueEmail, "password": "Password123!",
	})
	var loginResp struct {
		Token string `json:"token"`
	}
	testutils.ParseJSONBody(t, resp, &loginResp)
	resp.Body.Close()
	client.SetToken(loginResp.Token)
	t.Log("✅ Authentification Marchand réussie")

	// ---------- 1. Shop + produit ----------
	t.Log("\n📋 ÉTAPE 1 : Création du shop et du produit")
	shopSlug := fmt.Sprintf("tontine-e2e-%d", time.Now().UnixNano())
	resp = client.MustDoRequest(t, "POST", "/api/shops", map[string]interface{}{
		"name": "Tontine E2E Shop",
		"slug": shopSlug,
	})
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

	resp = client.MustDoRequest(t, "POST", "/api/products", map[string]interface{}{
		"name":        "Produit Tontine E2E",
		"description": "Produit pour test cycle completion",
		"price_cents": 3000000,
		"stock":       10,
	})
	productID := testutils.ExtractID(t, resp)
	resp.Body.Close()
	t.Logf("✅ Shop (%s) et Produit (%s) créés", shopID, productID)

	// ---------- 2. Settings tontine ----------
	t.Log("\n📋 ÉTAPE 2 : Activation de la tontine sur le produit")
	resp = client.MustDoRequest(t, "PUT", "/api/shops/"+shopID+"/tontine-settings", map[string]interface{}{
		"product_id":              productID,
		"is_tontine_enabled":      true,
		"allow_commercial_circle": true,
		"allow_corporate_circle":  true,
		"allow_family_circle":     true,
		"min_participants":        3,
		"max_participants":        3,
	})
	resp.Body.Close()
	testutils.AssertStatus(t, resp, http.StatusOK)
	t.Log("✅ Tontine activée sur le produit (3 participants requis)")

	// ---------- 3. 3 clients KYC verified ----------
	t.Log("\n📋 ÉTAPE 3 : Création de 3 clients KYC vérifiés")
	merchantClient := client
	customers := make([]string, 3)
	ownerUserIDs := make([]string, 3)
	for i := 0; i < 3; i++ {
		customers[i], ownerUserIDs[i] = createVerifiedTontineCustomer(t, server.URL, merchantClient, server.DB, i)
		t.Logf("  ✅ Client %d créé et KYC vérifié: %s", i+1, customers[i])
	}

	var merchantUserID string
	if err := server.DB.QueryRow(`SELECT id::text FROM users WHERE email = $1`, uniqueEmail).Scan(&merchantUserID); err != nil {
		t.Fatalf("merchant user_id: %v", err)
	}
	if _, err := server.DB.Exec(`
		UPDATE customers
		SET user_id = $1, kyc_level = 'verified', kyc_validated_at = NOW(), updated_at = NOW()
		WHERE id = $2::uuid`,
		merchantUserID, customers[0],
	); err != nil {
		t.Fatalf("link creator customer to merchant: %v", err)
	}

	// ---------- 4. Groupe ----------
	t.Log("\n📋 ÉTAPE 4 : Création du groupe tontine (marchand)")
	resp = merchantClient.MustDoRequest(t, "POST", "/api/tontine/groups", map[string]interface{}{
		"name":                "Cercle E2E Famille",
		"product_id":          productID,
		"creator_customer_id": customers[0],
		"circle_type":         "FAMILY",
		"total_cycles":        3,
	})
	testutils.AssertStatus(t, resp, http.StatusCreated)

	var group map[string]interface{}
	testutils.ParseJSONBody(t, resp, &group)
	resp.Body.Close()

	groupID, _ := group["id"].(string)
	inviteCode, _ := group["invite_code"].(string)
	if groupID == "" || inviteCode == "" {
		t.Fatalf("réponse groupe invalide: %+v", group)
	}
	t.Logf("✅ Groupe créé: %s (Invite Code: %s)", groupID, inviteCode)

	// ---------- 5. Participants 2–3 + ACTIVE ----------
	t.Log("\n📋 ÉTAPE 5 : Seed participants 2–3 + activation groupe (SQL)")
	for i := 1; i < 3; i++ {
		_, err := server.DB.Exec(`
			INSERT INTO tontine_participants (id, group_id, customer_id, payout_position, status, joined_at)
			VALUES (gen_random_uuid(), $1::uuid, $2::uuid, $3, 'active', NOW())
			ON CONFLICT DO NOTHING`,
			groupID, customers[i], i+1,
		)
		if err != nil {
			_, err = server.DB.Exec(`
				INSERT INTO tontine_participants (group_id, customer_id, payout_position, status, joined_at)
				VALUES ($1::uuid, $2::uuid, $3, 'active', NOW())`,
				groupID, customers[i], i+1,
			)
		}
		if err != nil {
			t.Fatalf("insert participant %d: %v", i+1, err)
		}
		t.Logf("  ✅ Client %d ajouté en participant (position %d)", i+1, i+1)
	}
	if _, err := server.DB.Exec(`
		UPDATE tontine_groups
		SET status = 'ACTIVE', started_at = NOW(), updated_at = NOW()
		WHERE id = $1::uuid`, groupID); err != nil {
		t.Fatalf("activate group: %v", err)
	}
	t.Log("✅ Groupe ACTIVE (3 participants)")

	// ---------- 6. Pay (1 ref par customer) ----------
	t.Log("\n📋 ÉTAPE 6 : Initiation des 3 paiements réels (Cycle 1)")
	providerRefs := make([]string, 3)
	for i := 0; i < 3; i++ {
		for j := 0; j < 3; j++ {
			uid := ownerUserIDs[j]
			if j == i {
				uid = merchantUserID
			}
			if _, err := server.DB.Exec(`
				UPDATE customers
				SET user_id = $1, kyc_level = 'verified', updated_at = NOW()
				WHERE id = $2::uuid`,
				uid, customers[j]); err != nil {
				t.Fatalf("rebind customer %d: %v", j+1, err)
			}
		}

		resp = merchantClient.MustDoRequest(t, "POST",
			fmt.Sprintf("/api/tontine/groups/%s/pay", groupID),
			map[string]interface{}{
				"customer_id":  customers[i],
				"operator":     "orange_money",
				"phone_number": "+22670123456",
				"flow":         "indirect",
			})
		testutils.AssertStatus(t, resp, http.StatusOK)

		var payResp map[string]interface{}
		testutils.ParseJSONBody(t, resp, &payResp)
		resp.Body.Close()

		providerRefs[i], _ = payResp["provider_ref"].(string)
		if providerRefs[i] == "" {
			t.Fatalf("❌ provider_ref manquant client %d: %+v", i+1, payResp)
		}
		t.Logf("  ✅ Client %d pay (provider_ref: %s)", i+1, providerRefs[i])
	}

	// ---------- 7. Webhooks ----------
	t.Log("\n📋 ÉTAPE 7 : Simulation des 3 webhooks de confirmation de paiement")
	for i := 0; i < 3; i++ {
		txnID := fmt.Sprintf("TXN-E2E-%d-%d", time.Now().UnixNano(), i)
		webhookPayload := map[string]interface{}{
			"apiEnv":          "test",
			"paymentStatus":   "SUCCESS",
			"transId":         txnID,
			"projectId":       "e2e-project",
			"paymentIntentId": txnID,
			"paymentSource":   "orange_money",
			"customerNumber":  "+22670123456",
			"paymentAmount":   10000,
			"paymentFees":     0,
			"contryOrigin":    "BF",
			"reference":       providerRefs[i],
			"currency":        "XOF",
		}
		reqBody, err := json.Marshal(webhookPayload)
		if err != nil {
			t.Fatalf("❌ marshal webhook: %v", err)
		}
		signature := computeYengaWebhookSignature(webhookSecret, reqBody)

		req, err := http.NewRequest("POST", server.URL+"/webhooks/yenga_pay", strings.NewReader(string(reqBody)))
		if err != nil {
			t.Fatalf("❌ build webhook request: %v", err)
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

	// ---------- 8. DONE ----------
	t.Log("\n📋 ÉTAPE 8 : Vérification paiements DONE en base")
	var doneCount int
	if err := server.DB.QueryRow(
		`SELECT COUNT(*) FROM tontine_payments
		 WHERE group_id = $1 AND cycle_number = 1 AND status = 'DONE'`, groupID,
	).Scan(&doneCount); err != nil {
		t.Fatalf("count DONE: %v", err)
	}
	t.Logf("   tontine_payments DONE (cycle 1): %d / 3", doneCount)
	if doneCount < 3 {
		t.Fatalf("❌ attendu 3 paiements DONE, obtenu %d (refs=%v)", doneCount, providerRefs)
	}
	t.Log("✅ 3 paiements DONE")

	// ---------- 8.5 DB ----------
	t.Log("\n📋 ÉTAPE 8.5 : Diagnostic groupe / vouchers")
	var dbCurrentCycle int
	var dbStatus string
	if err := server.DB.QueryRow(
		"SELECT current_cycle, status FROM tontine_groups WHERE id = $1", groupID,
	).Scan(&dbCurrentCycle, &dbStatus); err != nil {
		t.Fatalf("read group: %v", err)
	}
	t.Logf("   tontine_groups: current_cycle=%d, status=%s", dbCurrentCycle, dbStatus)

	var voucherCount int
	if err := server.DB.QueryRow(
		"SELECT COUNT(*) FROM tontine_vouchers WHERE group_id = $1", groupID,
	).Scan(&voucherCount); err != nil {
		t.Fatalf("count vouchers: %v", err)
	}
	t.Logf("   tontine_vouchers: count=%d", voucherCount)
	if voucherCount == 0 {
		t.Fatal("❌ Aucun voucher : checkAndCompleteCycle n'a pas complété le cycle")
	}
	t.Log("✅ Voucher créé")

	// ---------- 9. Wallet (net de commission plateforme) ----------
	t.Log("\n📋 ÉTAPE 9 : Vérification du crédit du wallet marchand")
	resp = merchantClient.MustDoRequest(t, "GET", "/api/wallet", nil)
	testutils.AssertStatus(t, resp, http.StatusOK)

	var wallet struct {
		ShopID       string `json:"shop_id"`
		BalanceCents int64  `json:"balance_cents"`
	}
	testutils.ParseJSONBody(t, resp, &wallet)
	resp.Body.Close()

	grossCents := int64(3000000) // prix produit
	// CreditFromTontine crédite le net (ex. 1,5 % → 45_000 de commission)
	if wallet.BalanceCents <= 0 {
		t.Fatalf("❌ Wallet non crédité: balance=%d", wallet.BalanceCents)
	}
	if wallet.BalanceCents > grossCents {
		t.Fatalf("❌ balance %d > montant brut %d", wallet.BalanceCents, grossCents)
	}
	commission := grossCents - wallet.BalanceCents
	t.Logf("   Solde wallet: %d cents (brut %d, commission %d)",
		wallet.BalanceCents, grossCents, commission)
	// Commission raisonnable : 0–10 %
	if commission > grossCents/10 {
		t.Fatalf("❌ commission anormale %d (>%d)", commission, grossCents/10)
	}
	t.Log("✅ Wallet marchand crédité (net commission) — CreditFromTontine OK")
	t.Log("\n" + strings.Repeat("=", 70))
	t.Log("🎉 TEST E2E TONTINE CYCLE COMPLETION TERMINÉ !")
	t.Log(strings.Repeat("=", 70))
}

func computeYengaWebhookSignature(secret string, payload []byte) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(payload)
	return hex.EncodeToString(mac.Sum(nil))
}

func createVerifiedTontineCustomer(
	t *testing.T,
	serverURL string,
	merchantClient *testutils.HTTPClient,
	db *sql.DB,
	index int,
) (customerID string, userID string) {
	t.Helper()

	uniqueEmail := fmt.Sprintf("client%d.tontine.%d@example.com", index+1, time.Now().UnixNano())

	client := testutils.NewHTTPClient(serverURL)
	client.Client.Timeout = 60 * time.Second

	resp := client.MustDoRequest(t, "POST", "/register", map[string]interface{}{
		"email": uniqueEmail, "password": "Password123!",
	})
	resp.Body.Close()

	err := db.QueryRow(`SELECT id::text FROM users WHERE email = $1`, uniqueEmail).Scan(&userID)
	if err != nil {
		t.Fatalf("user_id pour %s: %v", uniqueEmail, err)
	}

	resp = merchantClient.MustDoRequest(t, "POST", "/api/customers", map[string]interface{}{
		"first_name": fmt.Sprintf("Client%d", index+1),
		"last_name":  "Tontine",
		"email":      uniqueEmail,
	})
	customerID = testutils.ExtractID(t, resp)
	resp.Body.Close()

	_, err = db.Exec(`
		UPDATE customers
		SET user_id = $1,
		    kyc_level = 'verified',
		    kyc_validated_at = NOW(),
		    updated_at = NOW()
		WHERE id = $2::uuid`,
		userID, customerID,
	)
	if err != nil {
		t.Fatalf("seed KYC verified customer %s: %v", customerID, err)
	}

	return customerID, userID
}
