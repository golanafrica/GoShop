package e2e

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"testing"
	"time"

	"Goshop/tests/testutils"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCreditLifecycleE2E(t *testing.T) {
	t.Log("======================================================================")
	t.Log("💳 E2E TEST : Cycle de vie complet du Crédit (Configuration -> Demande -> Approbation -> Apport -> Échéance)")
	t.Log("======================================================================")

	server := testutils.NewTestServer(t)
	client := testutils.NewHTTPClient(server.URL)

	// =========================================================================
	// ÉTAPE 0 : Authentification du marchand et création de la boutique
	// =========================================================================
	t.Log("\n📋 ÉTAPE 0 : Authentification et création de la boutique")
	merchantEmail := fmt.Sprintf("merchant_credit_%d@example.com", time.Now().UnixNano())

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

	resp = client.MustDoRequest(t, "GET", "/auth/me", nil)
	testutils.AssertStatus(t, resp, http.StatusOK)
	var meResp map[string]interface{}
	testutils.ParseJSONBody(t, resp, &meResp)
	merchantUserID := meResp["id"].(string)

	shopSlug := fmt.Sprintf("credit-shop-%d", time.Now().UnixNano())
	resp = client.MustDoRequest(t, "POST", "/api/shops", map[string]interface{}{
		"name": "Boutique Crédit Test",
		"slug": shopSlug,
	})
	testutils.AssertStatus(t, resp, http.StatusCreated)

	var shopResp map[string]interface{}
	testutils.ParseJSONBody(t, resp, &shopResp)
	t.Logf("✅ Boutique créée: %s", shopResp["id"].(string))

	client.SetDefaultHeader("X-Shop-Slug", shopSlug)

	// =========================================================================
	// ÉTAPE 1 : Création d'un produit éligible au crédit
	// =========================================================================
	t.Log("\n📋 ÉTAPE 1 : Création d'un produit")
	productData := map[string]interface{}{
		"name":        "Smartphone Crédit",
		"description": "Smartphone éligible au paiement par tempérament",
		"price_cents": 100000,
		"stock":       10,
	}
	resp = client.MustDoRequest(t, "POST", "/api/products", productData)
	testutils.AssertStatus(t, resp, http.StatusCreated)

	var productResp map[string]interface{}
	testutils.ParseJSONBody(t, resp, &productResp)
	productID := productResp["id"].(string)
	t.Logf("✅ Produit créé: %s", productID)

	// =========================================================================
	// ÉTAPE 2 : Configuration du plan de crédit par le marchand
	// =========================================================================
	t.Log("\n📋 ÉTAPE 2 : Configuration du plan de crédit")
	creditPlanData := map[string]interface{}{
		"product_id":               productID,
		"is_enabled":               true,
		"min_down_payment_percent": 20,
		"max_duration_months":      12,
		"interest_rate_bps":        500,
		"penalty_rate_bps":         500,
		"min_credit_score":         300,
	}
	resp = client.MustDoRequest(t, "POST", "/api/credit/plans", creditPlanData)
	testutils.AssertStatus(t, resp, http.StatusOK)
	t.Log("✅ Plan de crédit configuré avec succès")

	// =========================================================================
	// ÉTAPE 3 : Le client s'inscrit et crée son profil
	// =========================================================================
	t.Log("\n📋 ÉTAPE 3 : Inscription et création du profil client")
	customerEmail := fmt.Sprintf("customer_credit_%d@example.com", time.Now().UnixNano())

	resp = client.MustDoRequest(t, "POST", "/register", map[string]interface{}{
		"email":    customerEmail,
		"password": "Password123!",
	})
	testutils.AssertStatus(t, resp, http.StatusCreated)

	resp = client.MustDoRequest(t, "POST", "/login", map[string]interface{}{
		"email":    customerEmail,
		"password": "Password123!",
	})
	testutils.AssertStatus(t, resp, http.StatusOK)
	var customerLoginResp map[string]interface{}
	testutils.ParseJSONBody(t, resp, &customerLoginResp)

	customerClient := testutils.NewHTTPClient(server.URL)
	customerClient.SetToken(customerLoginResp["token"].(string))
	customerClient.SetDefaultHeader("X-Shop-Slug", shopSlug)

	customerData := map[string]interface{}{
		"first_name":   "Jean",
		"last_name":    "Dupont",
		"email":        customerEmail,
		"phone_number": "70000000",
	}
	resp = customerClient.MustDoRequest(t, "POST", "/api/customers", customerData)
	testutils.AssertStatus(t, resp, http.StatusCreated)
	var custResp map[string]interface{}
	testutils.ParseJSONBody(t, resp, &custResp)
	customerID := custResp["id"].(string)
	t.Logf("✅ Profil client créé: %s", customerID)

	// =========================================================================
	// ÉTAPE 4 : Le client demande un crédit
	// =========================================================================
	t.Log("\n📋 ÉTAPE 4 : Demande de crédit par le client")
	creditApplyData := map[string]interface{}{
		"customer_id":               customerID,
		"product_id":                productID,
		"requested_duration_months": 6,
	}
	resp = customerClient.MustDoRequest(t, "POST", "/api/credit/apply", creditApplyData)
	testutils.AssertStatus(t, resp, http.StatusCreated)

	var applyResp map[string]interface{}
	testutils.ParseJSONBody(t, resp, &applyResp)
	application := applyResp["application"].(map[string]interface{})
	applicationID := application["application_id"].(string)
	t.Logf("✅ Demande de crédit créée: %s", applicationID)

	// =========================================================================
	// ÉTAPE 5 : Le marchand approuve la demande
	// =========================================================================
	t.Log("\n📋 ÉTAPE 5 : Approbation de la demande par le marchand")
	approveData := map[string]interface{}{
		"application_id": applicationID,
		"reviewed_by":    merchantUserID,
	}
	resp = client.MustDoRequest(t, "POST", "/api/credit/approve", approveData)
	testutils.AssertStatus(t, resp, http.StatusOK)

	var approveResp map[string]interface{}
	testutils.ParseJSONBody(t, resp, &approveResp)
	approval := approveResp["approval"].(map[string]interface{})
	contractID := approval["contract_id"].(string)
	t.Logf("✅ Demande approuvée, Contrat créé: %s", contractID)

	// =========================================================================
	// ÉTAPE 6 : Le client paie l'apport initial (NOUVEAU FLUX SÉCURISÉ)
	// =========================================================================
	t.Log("\n📋 ÉTAPE 6 : Initiation du paiement de l'apport initial par le client")
	payDownData := map[string]interface{}{
		"contract_id":  contractID,
		"phone_number": "70000000",
		"operator":     "orange_money",
	}
	resp = customerClient.MustDoRequest(t, "POST", "/api/credit/down-payment", payDownData)
	testutils.AssertStatus(t, resp, http.StatusCreated)

	var payDownResp map[string]interface{}
	testutils.ParseJSONBody(t, resp, &payDownResp)
	data := payDownResp["data"].(map[string]interface{})
	t.Logf("✅ Paiement de l'apport initial initié avec succès. ProviderRef: %v", data["provider_ref"])

	// =========================================================================
	// 🆕 ÉTAPE 7 : Le client paie la première échéance (Simulation Webhook)
	// =========================================================================
	t.Log("\n📋 ÉTAPE 7 : Paiement de la première échéance par le client")

	resp = client.MustDoRequest(t, "GET", fmt.Sprintf("/api/credit/contracts/%s", contractID), nil)
	testutils.AssertStatus(t, resp, http.StatusOK)

	var contractDetailsResp map[string]interface{}
	testutils.ParseJSONBody(t, resp, &contractDetailsResp)

	installments := contractDetailsResp["installments"].([]interface{})
	firstInstallment := installments[0].(map[string]interface{})
	installmentID := firstInstallment["id"].(string)
	installmentAmount := int64(firstInstallment["amount_cents"].(float64))
	t.Logf("✅ Première échéance récupérée: %s (Montant: %d cents)", installmentID, installmentAmount)

	payInstallmentData := map[string]interface{}{
		"phone_number": "70000000",
		"operator":     "orange_money",
	}
	resp = customerClient.MustDoRequest(t, "POST", fmt.Sprintf("/api/credit/installments/%s/pay", installmentID), payInstallmentData)
	testutils.AssertStatus(t, resp, http.StatusCreated)

	var payInstallmentResp map[string]interface{}
	testutils.ParseJSONBody(t, resp, &payInstallmentResp)

	instData := payInstallmentResp["data"].(map[string]interface{})
	providerRef := instData["provider_ref"].(string)
	t.Logf("✅ Paiement de l'échéance initié. ProviderRef: %s", providerRef)

	// ✅ CORRECTION FINALE BASÉE SUR LE CODE SOURCE DU MOCK :
	// Le mock orange_money_provider.go attend EXACTEMENT la clé "provider_ref" (voir ligne ~165)
	webhookPayload := map[string]interface{}{
		"provider_ref": providerRef, // ✅ Clé exacte attendue par json:"provider_ref" dans le mock
		"status":       "success",   // ✅ Doit matcher entity.PaymentStatusSuccess
	}
	payloadBytes, _ := json.Marshal(webhookPayload)

	// Calculer la signature HMAC-SHA256 avec le secret exact du mock Orange Money
	mac := hmac.New(sha256.New, []byte("orange_money_webhook_secret_dev_only_change_in_prod"))
	mac.Write(payloadBytes)
	signature := hex.EncodeToString(mac.Sum(nil))

	// Envoyer le webhook manuellement
	req, _ := http.NewRequest("POST", server.URL+"/webhooks/orange_money", bytes.NewBuffer(payloadBytes))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Signature", signature)

	webhookClient := &http.Client{Timeout: 10 * time.Second}
	webhookResp, err := webhookClient.Do(req)
	require.NoError(t, err, "Failed to send webhook")
	defer webhookResp.Body.Close()
	testutils.AssertStatus(t, webhookResp, http.StatusOK)
	t.Log("✅ Webhook de succès simulé et traité avec succès par le backend")

	// 7.4 Vérifier que l'échéance est bien passée au statut "paid" en base de données
	resp = client.MustDoRequest(t, "GET", fmt.Sprintf("/api/credit/contracts/%s", contractID), nil)
	testutils.AssertStatus(t, resp, http.StatusOK)

	var updatedContractResp map[string]interface{}
	testutils.ParseJSONBody(t, resp, &updatedContractResp)

	updatedInstallments := updatedContractResp["installments"].([]interface{})
	updatedFirstInstallment := updatedInstallments[0].(map[string]interface{})
	installmentStatus := updatedFirstInstallment["status"].(string)

	assert.Equal(t, "paid", installmentStatus, "L'échéance devrait être marquée comme 'paid' après le webhook")
	t.Logf("✅ Statut de l'échéance mis à jour avec succès en base de données: %s", installmentStatus)

	t.Log("\n======================================================================")
	t.Log("🎉 TOUS LES TESTS DE CYCLE DE VIE DU CRÉDIT PASSENT (y compris le paiement d'échéance) !")
	t.Log("======================================================================")
}
