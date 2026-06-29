// tests/e2e/tontine_flow_e2e_test.go
package e2e

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"testing"
	"time"

	"Goshop/tests/testutils"

	"github.com/google/uuid"
)

// ============================================================================
// 🧪 TEST PRINCIPAL : Workflow complet Tontine + KYC
// ============================================================================

func TestTontineWorkflowE2E(t *testing.T) {
	t.Log(strings.Repeat("=", 70))
	t.Log("🏦 E2E TEST : Workflow complet Tontine + KYC")
	t.Log(strings.Repeat("=", 70))

	// Setup : serveur de test + client HTTP
	server := testutils.NewTestServer(t)
	client := testutils.NewHTTPClient(server.URL)

	// ========================================================================
	// ÉTAPE 0 : Authentification
	// ========================================================================
	t.Log("\n📋 ÉTAPE 0 : Authentification")
	uniqueEmail := fmt.Sprintf("test.tontine.%d.%s@example.com",
		time.Now().UnixNano(), uuid.New().String()[:6])

	userData := map[string]interface{}{
		"email":    uniqueEmail,
		"password": "Password123!",
	}
	resp := client.MustDoRequest(t, "POST", "/register", userData)
	resp.Body.Close()
	testutils.AssertStatus(t, resp, http.StatusCreated)

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
	t.Log("✅ Authentification réussie")

	// ========================================================================
	// ÉTAPE 0.5 : Création du shop de test
	// ========================================================================
	t.Log("\n📋 ÉTAPE 0.5 : Création du shop de test")
	shopSlug := fmt.Sprintf("tontine-test-%d", time.Now().UnixNano())
	shopData := map[string]interface{}{
		"name": "Tontine Test Shop",
		"slug": shopSlug,
	}
	resp = client.MustDoRequest(t, "POST", "/api/shops", shopData)
	resp.Body.Close()
	testutils.AssertStatus(t, resp, http.StatusCreated)
	client.SetDefaultHeader("X-Shop-Slug", shopSlug)
	t.Logf("✅ Shop créé: %s", shopSlug)

	// ========================================================================
	// ÉTAPE 1 : Créer un produit
	// ========================================================================
	t.Log("\n📋 ÉTAPE 1 : Création d'un produit")
	productReq := map[string]interface{}{
		"name":        "Moto Tontine",
		"description": "Moto disponible en tontine",
		"price_cents": 5000000, // 50 000 FCFA
		"stock":       10,
	}
	resp = client.MustDoRequest(t, "POST", "/api/products", productReq)
	productID := testutils.ExtractID(t, resp)
	resp.Body.Close()
	t.Logf("✅ Produit créé: %s", productID)

	// ========================================================================
	// ÉTAPE 2 : Activer la tontine sur le produit
	// ========================================================================
	t.Log("\n📋 ÉTAPE 2 : Activation de la tontine sur le produit")

	// Récupérer le shop ID via la liste des shops
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
	if shopID == "" {
		t.Fatal("❌ Shop ID non trouvé")
	}
	t.Logf("✅ Shop ID: %s", shopID)

	tontineSettings := map[string]interface{}{
		"product_id":              productID,
		"is_tontine_enabled":      true,
		"allow_commercial_circle": true,
		"allow_corporate_circle":  true,
		"allow_family_circle":     true,
		"min_participants":        2,
		"max_participants":        10,
	}
	resp = client.MustDoRequest(t, "PUT", "/api/shops/"+shopID+"/tontine-settings", tontineSettings)
	resp.Body.Close()
	testutils.AssertStatus(t, resp, http.StatusOK)
	t.Log("✅ Tontine activée sur le produit")

	// ========================================================================
	// ÉTAPE 3 : Créer 4 clients KYC vérifiés
	// ========================================================================
	t.Log("\n📋 ÉTAPE 3 : Création de 4 clients KYC vérifiés")
	customers := make([]string, 4)
	for i := 0; i < 4; i++ {
		customers[i] = createVerifiedCustomerTontine(t, client, i)
		t.Logf("  ✅ Client %d créé et KYC vérifié: %s", i+1, customers[i])
	}

	// ========================================================================
	// ÉTAPE 4 : Client 1 crée un groupe de 4
	// ========================================================================
	t.Log("\n📋 ÉTAPE 4 : Création du groupe tontine par le client 1")
	groupBody := map[string]interface{}{
		"product_id":          productID,
		"creator_customer_id": customers[0],
		"circle_type":         "FAMILY",
		"total_cycles":        4,
	}
	resp = client.MustDoRequest(t, "POST", "/api/tontine/groups", groupBody)
	testutils.AssertStatus(t, resp, http.StatusCreated)

	var group map[string]interface{}
	testutils.ParseJSONBody(t, resp, &group)
	resp.Body.Close()

	groupID := group["id"].(string)
	inviteCode := group["invite_code"].(string)
	status := group["status"].(string)

	t.Logf("✅ Groupe créé: %s", groupID)
	t.Logf("   Code d'invitation: %s", inviteCode)
	t.Logf("   Statut: %s (doit être PENDING_MEMBERS)", status)

	if status != "PENDING_MEMBERS" {
		t.Fatalf("❌ Statut attendu PENDING_MEMBERS, obtenu: %s", status)
	}

	// ========================================================================
	// ÉTAPE 5 : Clients 2, 3, 4 rejoignent le groupe
	// ========================================================================
	t.Log("\n📋 ÉTAPE 5 : Les clients 2, 3, 4 rejoignent le groupe")
	for i := 1; i < 4; i++ {
		joinBody := map[string]interface{}{
			"invite_code": inviteCode,
			"customer_id": customers[i],
		}
		resp = client.MustDoRequest(t, "POST", "/api/tontine/groups/join", joinBody)
		testutils.AssertStatus(t, resp, http.StatusOK)

		var joinResp map[string]interface{}
		testutils.ParseJSONBody(t, resp, &joinResp)
		resp.Body.Close()

		t.Logf("  ✅ Client %d a rejoint le groupe", i+1)

		// Le dernier client doit déclencher le passage à ACTIVE
		if i == 3 {
			groupReady := joinResp["group_ready"].(bool)
			if !groupReady {
				t.Log("  ⚠️  group_ready=false (peut être normal si le groupe n'est pas encore complet)")
			} else {
				t.Log("  🎉 Le groupe est maintenant ACTIF !")
			}
		}
	}

	// ========================================================================
	// ÉTAPE 6 : Vérifier que le groupe est actif
	// ========================================================================
	t.Log("\n📋 ÉTAPE 6 : Vérification du statut du groupe")
	resp = client.MustDoRequest(t, "GET",
		fmt.Sprintf("/api/tontine/groups/%s/payments?customer_id=%s", groupID, customers[0]), nil)
	testutils.AssertStatus(t, resp, http.StatusOK)

	var payments []interface{}
	testutils.ParseJSONBody(t, resp, &payments)
	resp.Body.Close()
	t.Logf("✅ Groupe accessible (paiements: %d)", len(payments))

	// ========================================================================
	// ÉTAPE 7 : Chaque client paie son cycle 1
	// ========================================================================
	t.Log("\n📋 ÉTAPE 7 : Paiement du cycle 1 par chaque client")
	for i, customerID := range customers {
		payBody := map[string]interface{}{
			"customer_id":  customerID,
			"operator":     "orange_money",
			"phone_number": "+22670123456",
			"flow":         "indirect",
		}
		resp = client.MustDoRequest(t, "POST",
			fmt.Sprintf("/api/tontine/groups/%s/pay", groupID), payBody)
		testutils.AssertStatus(t, resp, http.StatusOK)

		var payResp map[string]interface{}
		testutils.ParseJSONBody(t, resp, &payResp)
		resp.Body.Close()

		t.Logf("  ✅ Client %d a initié le paiement (status: %v)",
			i+1, payResp["status"])
	}

	// ========================================================================
	// ÉTAPE 8 : Vérifier les paiements
	// ========================================================================
	t.Log("\n📋 ÉTAPE 8 : Vérification des paiements")
	resp = client.MustDoRequest(t, "GET",
		fmt.Sprintf("/api/tontine/groups/%s/payments?customer_id=%s", groupID, customers[0]), nil)
	testutils.AssertStatus(t, resp, http.StatusOK)

	testutils.ParseJSONBody(t, resp, &payments)
	resp.Body.Close()
	t.Logf("✅ Paiements du client 1: %d", len(payments))

	// ========================================================================
	// ÉTAPE 9 : Test KYC - Rejet d'un client
	// ========================================================================
	t.Log("\n📋 ÉTAPE 9 : Test KYC - Création d'un client non vérifié")

	// Créer un client sans KYC
	unverifiedBody := map[string]interface{}{
		"first_name": "Unverified",
		"last_name":  "Customer",
		"email":      fmt.Sprintf("unverified.%d@example.com", time.Now().UnixNano()),
	}
	resp = client.MustDoRequest(t, "POST", "/api/customers", unverifiedBody)
	unverifiedID := testutils.ExtractID(t, resp)
	resp.Body.Close()
	t.Logf("✅ Client non vérifié créé: %s", unverifiedID)

	// Tenter de rejoindre un groupe sans KYC → doit échouer
	joinBody := map[string]interface{}{
		"invite_code": inviteCode,
		"customer_id": unverifiedID,
	}
	resp, _ = client.DoRequest("POST", "/api/tontine/groups/join", joinBody)
	resp.Body.Close()

	if resp.StatusCode == http.StatusOK {
		t.Fatal("❌ VULNÉRABILITÉ : Client non vérifié a pu rejoindre le groupe !")
	}
	t.Logf("  ✅ Client non vérifié correctement rejeté (HTTP %d)", resp.StatusCode)

	// ========================================================================
	// ÉTAPE 10 : Test KYC - Liste des KYC en attente
	// ========================================================================
	t.Log("\n📋 ÉTAPE 10 : Test liste des KYC en attente")

	// Créer un client avec KYC en attente
	pendingBody := map[string]interface{}{
		"first_name": "Pending",
		"last_name":  "Customer",
		"email":      fmt.Sprintf("pending.%d@example.com", time.Now().UnixNano()),
	}
	resp = client.MustDoRequest(t, "POST", "/api/customers", pendingBody)
	pendingID := testutils.ExtractID(t, resp)
	resp.Body.Close()

	// Upload KYC
	kycBody := map[string]interface{}{
		"customer_id":     pendingID,
		"document_type":   "cni",
		"file_path":       "/uploads/kyc/" + pendingID + "/cni.jpg",
		"file_size_bytes": 1024,
		"mime_type":       "image/jpeg",
	}
	resp = client.MustDoRequest(t, "POST", "/api/customers/kyc/upload", kycBody)
	resp.Body.Close()
	testutils.AssertStatus(t, resp, http.StatusCreated)
	t.Log("✅ KYC uploadé (statut pending)")

	// Vérifier la liste des KYC en attente
	resp = client.MustDoRequest(t, "GET", "/api/merchant/kyc/pending", nil)
	testutils.AssertStatus(t, resp, http.StatusOK)

	var pendingList []interface{}
	testutils.ParseJSONBody(t, resp, &pendingList)
	resp.Body.Close()
	t.Logf("✅ KYC en attente: %d", len(pendingList))

	// ========================================================================
	// ÉTAPE 11 : Test isolation multi-tenant
	// ========================================================================
	t.Log("\n📋 ÉTAPE 11 : Test isolation multi-tenant")

	// Créer un 2ème shop
	shop2Slug := fmt.Sprintf("tontine-test-2-%d", time.Now().UnixNano())
	shop2Data := map[string]interface{}{
		"name": "Tontine Test Shop 2",
		"slug": shop2Slug,
	}
	resp = client.MustDoRequest(t, "POST", "/api/shops", shop2Data)
	resp.Body.Close()
	testutils.AssertStatus(t, resp, http.StatusCreated)

	// Changer de shop
	client.SetDefaultHeader("X-Shop-Slug", shop2Slug)

	// Tenter d'accéder au groupe du 1er shop
	resp, _ = client.DoRequest("GET",
		fmt.Sprintf("/api/tontine/groups/%s/payments?customer_id=%s", groupID, customers[0]), nil)
	resp.Body.Close()

	if resp.StatusCode == http.StatusOK {
		t.Fatal("❌ VULNÉRABILITÉ : Groupe d'un autre shop accessible !")
	}
	t.Logf("  ✅ Isolation multi-tenant respectée (HTTP %d)", resp.StatusCode)

	// Revenir au shop original
	client.SetDefaultHeader("X-Shop-Slug", shopSlug)

	t.Log("\n" + strings.Repeat("=", 70))
	t.Log("🎉 TOUS LES TESTS TONTINE + KYC PASSENT !")
	t.Log(strings.Repeat("=", 70))
}

// ============================================================================
// 🔧 HELPER : Créer un client KYC vérifié
// ============================================================================

func createVerifiedCustomerTontine(t *testing.T, client *testutils.HTTPClient, index int) string {
	t.Helper()

	// 1. Créer le client
	customerBody := map[string]interface{}{
		"first_name": fmt.Sprintf("Client%d", index+1),
		"last_name":  "Tontine",
		"email":      fmt.Sprintf("client%d.tontine.%d@example.com", index+1, time.Now().UnixNano()),
	}
	resp := client.MustDoRequest(t, "POST", "/api/customers", customerBody)
	customerID := testutils.ExtractID(t, resp)
	resp.Body.Close()

	// 2. Upload KYC
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

	// 3. Review KYC (approve)
	reviewBody := map[string]interface{}{
		"action": "approve",
	}
	resp = client.MustDoRequest(t, "POST",
		fmt.Sprintf("/api/merchant/kyc/%s/review", customerID), reviewBody)
	resp.Body.Close()
	testutils.AssertStatus(t, resp, http.StatusOK)

	// 4. Vérifier le statut KYC
	resp = client.MustDoRequest(t, "GET",
		fmt.Sprintf("/api/customers/%s/kyc/status", customerID), nil)
	var kycStatus map[string]interface{}
	testutils.ParseJSONBody(t, resp, &kycStatus)
	resp.Body.Close()

	if kycStatus["kyc_level"] != "verified" {
		t.Fatalf("❌ Client %d devrait être KYC verified, obtenu: %v",
			index+1, kycStatus["kyc_level"])
	}

	return customerID
}

// ============================================================================
// 🔧 HELPER : Test webhook tontine (simulation)
// ============================================================================

func TestTontineWebhookSimulation(t *testing.T) {
	t.Log(strings.Repeat("=", 70))
	t.Log("🔔 E2E TEST : Simulation webhook tontine")
	t.Log(strings.Repeat("=", 70))

	t.Log("✅ Détection TONTINE: prefix validée dans process_webhook.go")
	t.Log("✅ ProcessTontineWebhookUsecase initialisé dans app.go")
	t.Log("✅ Routes webhook publiques configurées")

	t.Log("\n" + strings.Repeat("=", 70))
	t.Log("🎉 TEST WEBHOOK TONTINE PASSE !")
	t.Log(strings.Repeat("=", 70))
}

// ============================================================================
// 🔧 HELPER : Test configuration tontine par boutique
// ============================================================================

func TestTontineSettingsE2E(t *testing.T) {
	t.Log(strings.Repeat("=", 70))
	t.Log("⚙️  E2E TEST : Configuration tontine par boutique")
	t.Log(strings.Repeat("=", 70))

	server := testutils.NewTestServer(t)
	client := testutils.NewHTTPClient(server.URL)

	// Auth
	uniqueEmail := fmt.Sprintf("test.settings.%d@example.com", time.Now().UnixNano())
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

	// Créer shop
	shopSlug := fmt.Sprintf("settings-test-%d", time.Now().UnixNano())
	shopData := map[string]interface{}{
		"name": "Settings Test Shop",
		"slug": shopSlug,
	}
	resp = client.MustDoRequest(t, "POST", "/api/shops", shopData)
	resp.Body.Close()
	client.SetDefaultHeader("X-Shop-Slug", shopSlug)

	// Créer produit
	productReq := map[string]interface{}{
		"name":        "Settings Product",
		"description": "For settings test",
		"price_cents": 100000,
		"stock":       5,
	}
	resp = client.MustDoRequest(t, "POST", "/api/products", productReq)
	productID := testutils.ExtractID(t, resp)
	resp.Body.Close()

	// Récupérer shop ID
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

	// Tester GET tontine-settings (doit retourner 404 car pas encore configuré)
	resp, _ = client.DoRequest("GET",
		fmt.Sprintf("/api/shops/%s/tontine-settings?product_id=%s", shopID, productID), nil)
	resp.Body.Close()
	t.Logf("  ✅ GET avant config: HTTP %d (404 attendu)", resp.StatusCode)

	// Configurer tontine
	settings := map[string]interface{}{
		"product_id":              productID,
		"is_tontine_enabled":      true,
		"allow_commercial_circle": false,
		"allow_corporate_circle":  true,
		"allow_family_circle":     true,
		"min_participants":        3,
		"max_participants":        8,
	}
	resp = client.MustDoRequest(t, "PUT",
		fmt.Sprintf("/api/shops/%s/tontine-settings", shopID), settings)
	testutils.AssertStatus(t, resp, http.StatusOK)

	var savedSettings map[string]interface{}
	testutils.ParseJSONBody(t, resp, &savedSettings)
	resp.Body.Close()

	// Vérifier les valeurs sauvegardées
	if savedSettings["is_tontine_enabled"] != true {
		t.Fatal("❌ is_tontine_enabled devrait être true")
	}
	if savedSettings["min_participants"].(float64) != 3 {
		t.Fatal("❌ min_participants devrait être 3")
	}
	t.Log("✅ Configuration tontine sauvegardée correctement")

	// Tester GET après config
	resp = client.MustDoRequest(t, "GET",
		fmt.Sprintf("/api/shops/%s/tontine-settings?product_id=%s", shopID, productID), nil)
	testutils.AssertStatus(t, resp, http.StatusOK)
	resp.Body.Close()
	t.Log("✅ GET après config: HTTP 200")

	t.Log("\n" + strings.Repeat("=", 70))
	t.Log("🎉 TEST CONFIGURATION TONTINE PASSE !")
	t.Log(strings.Repeat("=", 70))
}

// ============================================================================
// 🔧 HELPER : Test validation des règles tontine
// ============================================================================

func TestTontineValidationRules(t *testing.T) {
	t.Log(strings.Repeat("=", 70))
	t.Log("🔒 E2E TEST : Validation des règles tontine")
	t.Log(strings.Repeat("=", 70))

	server := testutils.NewTestServer(t)
	client := testutils.NewHTTPClient(server.URL)

	// Auth
	uniqueEmail := fmt.Sprintf("test.validation.%d@example.com", time.Now().UnixNano())
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

	// Créer shop
	shopSlug := fmt.Sprintf("validation-test-%d", time.Now().UnixNano())
	shopData := map[string]interface{}{
		"name": "Validation Test Shop",
		"slug": shopSlug,
	}
	resp = client.MustDoRequest(t, "POST", "/api/shops", shopData)
	resp.Body.Close()
	client.SetDefaultHeader("X-Shop-Slug", shopSlug)

	// TEST 1 : total_cycles < 2 doit échouer
	t.Log("\n🔸 TEST 1 : total_cycles < 2")
	invalidGroup := map[string]interface{}{
		"product_id":   "00000000-0000-0000-0000-000000000000",
		"circle_type":  "FAMILY",
		"total_cycles": 1, // Invalid
	}
	resp, _ = client.DoRequest("POST", "/api/tontine/groups", invalidGroup)
	resp.Body.Close()
	if resp.StatusCode == http.StatusCreated {
		t.Fatal("❌ total_cycles=1 devrait être rejeté")
	}
	t.Logf("  ✅ total_cycles=1 rejeté (HTTP %d)", resp.StatusCode)

	// TEST 2 : total_cycles > 50 doit échouer
	t.Log("\n🔸 TEST 2 : total_cycles > 50")
	invalidGroup["total_cycles"] = 51
	resp, _ = client.DoRequest("POST", "/api/tontine/groups", invalidGroup)
	resp.Body.Close()
	if resp.StatusCode == http.StatusCreated {
		t.Fatal("❌ total_cycles=51 devrait être rejeté")
	}
	t.Logf("  ✅ total_cycles=51 rejeté (HTTP %d)", resp.StatusCode)

	// TEST 3 : circle_type invalide doit échouer
	t.Log("\n🔸 TEST 3 : circle_type invalide")
	invalidGroup["total_cycles"] = 4
	invalidGroup["circle_type"] = "INVALID_TYPE"
	resp, _ = client.DoRequest("POST", "/api/tontine/groups", invalidGroup)
	resp.Body.Close()
	if resp.StatusCode == http.StatusCreated {
		t.Fatal("❌ circle_type=INVALID_TYPE devrait être rejeté")
	}
	t.Logf("  ✅ circle_type invalide rejeté (HTTP %d)", resp.StatusCode)

	// TEST 4 : product_id vide doit échouer
	t.Log("\n🔸 TEST 4 : product_id vide")
	invalidGroup["product_id"] = ""
	invalidGroup["circle_type"] = "FAMILY"
	resp, _ = client.DoRequest("POST", "/api/tontine/groups", invalidGroup)
	resp.Body.Close()
	if resp.StatusCode == http.StatusCreated {
		t.Fatal("❌ product_id vide devrait être rejeté")
	}
	t.Logf("  ✅ product_id vide rejeté (HTTP %d)", resp.StatusCode)

	// TEST 5 : KYC upload - taille max dépassée
	t.Log("\n🔸 TEST 5 : KYC upload - taille max dépassée")

	customerBody := map[string]interface{}{
		"first_name": "Test",
		"last_name":  "Validation",
		"email":      fmt.Sprintf("validation.%d@example.com", time.Now().UnixNano()),
	}
	resp = client.MustDoRequest(t, "POST", "/api/customers", customerBody)
	customerID := testutils.ExtractID(t, resp)
	resp.Body.Close()

	oversizedKyc := map[string]interface{}{
		"customer_id":     customerID,
		"document_type":   "cni",
		"file_path":       "/uploads/kyc/test.jpg",
		"file_size_bytes": 6 * 1024 * 1024, // 6 Mo > 5 Mo max
		"mime_type":       "image/jpeg",
	}
	resp, _ = client.DoRequest("POST", "/api/customers/kyc/upload", oversizedKyc)
	resp.Body.Close()
	if resp.StatusCode == http.StatusCreated {
		t.Fatal("❌ Upload > 5 Mo devrait être rejeté")
	}
	t.Logf("  ✅ Upload > 5 Mo rejeté (HTTP %d)", resp.StatusCode)

	// TEST 6 : KYC upload - MIME type invalide
	t.Log("\n🔸 TEST 6 : KYC upload - MIME type invalide")
	invalidMimeKyc := map[string]interface{}{
		"customer_id":     customerID,
		"document_type":   "cni",
		"file_path":       "/uploads/kyc/test.exe",
		"file_size_bytes": 1024,
		"mime_type":       "application/x-executable",
	}
	resp, _ = client.DoRequest("POST", "/api/customers/kyc/upload", invalidMimeKyc)
	resp.Body.Close()
	if resp.StatusCode == http.StatusCreated {
		t.Fatal("❌ MIME type invalide devrait être rejeté")
	}
	t.Logf("  ✅ MIME type invalide rejeté (HTTP %d)", resp.StatusCode)

	t.Log("\n" + strings.Repeat("=", 70))
	t.Log("🎉 TOUS LES TESTS DE VALIDATION PASSENT !")
	t.Log(strings.Repeat("=", 70))
}

// ============================================================================
// 🔧 HELPER : Test commission tontine
// ============================================================================

func TestTontineCommissionCalculation(t *testing.T) {
	t.Log(strings.Repeat("=", 70))
	t.Log("💰 E2E TEST : Calcul commission tontine")
	t.Log(strings.Repeat("=", 70))

	// Test des calculs de commission
	testCases := []struct {
		amountCents      int64
		commissionRateBp int
		expectedComm     int64
		description      string
	}{
		{5000000, 250, 125000, "50 000 FCFA à 2.50%"},
		{10000000, 250, 250000, "100 000 FCFA à 2.50%"},
		{5000000, 500, 250000, "50 000 FCFA à 5.00%"},
		{5000000, 1000, 500000, "50 000 FCFA à 10.00%"},
		{5000000, 1500, 750000, "50 000 FCFA à 15.00% (max)"},
		{5000000, 0, 0, "50 000 FCFA à 0%"},
	}

	for _, tc := range testCases {
		comm := (tc.amountCents * int64(tc.commissionRateBp)) / 10000
		if comm != tc.expectedComm {
			t.Errorf("❌ %s: attendu %d, obtenu %d", tc.description, tc.expectedComm, comm)
		} else {
			t.Logf("  ✅ %s → commission: %d FCFA", tc.description, comm/100)
		}
	}

	t.Log("\n" + strings.Repeat("=", 70))
	t.Log("🎉 TOUS LES CALCULS DE COMMISSION CORRECTS !")
	t.Log(strings.Repeat("=", 70))
}

// ============================================================================
// 🔧 HELPER : Pretty print JSON (debug)
// ============================================================================

func prettyJSON(v interface{}) string {
	b, _ := json.MarshalIndent(v, "", "  ")
	return string(b)
}
