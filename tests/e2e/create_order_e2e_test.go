// tests/e2e/order_e2e_test.go
package e2e

import (
	"Goshop/tests/testutils"
	"fmt"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestCreateOrderE2E(t *testing.T) {
	t.Log(strings.Repeat("=", 60))
	t.Log("?? E2E TEST : POST /api/orders (sc�nario complet)")
	t.Log(strings.Repeat("=", 60))

	// Utiliser le serveur de test UNIFI�
	server := testutils.NewTestServer(t)
	client := testutils.NewHTTPClient(server.URL)

	// ?? �TAPE 0 : S'authentifier avant toute op�ration
	uniqueEmail := fmt.Sprintf("test.order.%d.%s@example.com", time.Now().UnixNano(), uuid.New().String()[:6])

	// Inscription
	userData := map[string]interface{}{
		"email":    uniqueEmail,
		"password": "Password123!",
	}
	resp := client.MustDoRequest(t, "POST", "/register", userData)
	resp.Body.Close()

	// Connexion
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
	client.SetToken(loginResp.Token) // ?? Active l'authentification

	// ======================================
	// �TAPE 1 : Cr�er un customer
	// ======================================
	customerReq := testutils.CustomerFixture()
	t.Logf("?? Customer: %s %s", customerReq["first_name"], customerReq["last_name"])

	resp = client.MustDoRequest(t, "POST", "/api/customers", customerReq)
	defer resp.Body.Close()
	testutils.AssertStatus(t, resp, http.StatusCreated)

	customerID := testutils.ExtractID(t, resp)
	t.Logf("? Customer cr��: %s", customerID)

	// ======================================
	// �TAPE 2 : Cr�er deux produits
	// ======================================
	product1Req := testutils.ProductFixture()
	product1Req["name"] = "Laptop Pro"
	product1Req["description"] = "High-end laptop"
	product1Req["price_cents"] = 150000
	product1Req["stock"] = 10

	resp = client.MustDoRequest(t, "POST", "/api/products", product1Req)
	defer resp.Body.Close()
	testutils.AssertStatus(t, resp, http.StatusCreated)
	product1ID := testutils.ExtractID(t, resp)
	t.Logf("? Produit 1 cr��: %s", product1ID)

	product2Req := testutils.ProductFixture()
	product2Req["name"] = "Mouse Wireless"
	product2Req["description"] = "Ergonomic wireless mouse"
	product2Req["price_cents"] = 2500
	product2Req["stock"] = 50

	resp = client.MustDoRequest(t, "POST", "/api/products", product2Req)
	defer resp.Body.Close()
	testutils.AssertStatus(t, resp, http.StatusCreated)
	product2ID := testutils.ExtractID(t, resp)
	t.Logf("? Produit 2 cr��: %s", product2ID)

	// ======================================
	// �TAPE 3 : Cr�er une commande
	// ======================================
	orderReq := map[string]interface{}{
		"customer_id": customerID,
		"items": []map[string]interface{}{
			{"product_id": product1ID, "quantity": 1},
			{"product_id": product2ID, "quantity": 2},
		},
	}

	t.Logf("?? Cr�ation commande pour customer %s", customerID)
	resp = client.MustDoRequest(t, "POST", "/api/orders", orderReq)
	defer resp.Body.Close()
	testutils.AssertStatus(t, resp, http.StatusCreated)

	orderID := testutils.ExtractID(t, resp)
	t.Logf("? Commande cr��e: %s", orderID)

	// ======================================
	// �TAPE 4 : V�rifier la commande
	// ======================================
	resp = client.MustDoRequest(t, "GET", "/api/orders/"+orderID, nil)
	defer resp.Body.Close()
	testutils.AssertStatus(t, resp, http.StatusOK)

	var order map[string]interface{}
	testutils.ParseJSONBody(t, resp, &order)

	// V�rifier le customer
	if order["customer_id"] != customerID {
		t.Errorf("? Mauvais customer_id. Attendu: %s, obtenu: %v", customerID, order["customer_id"])
	}

	// V�rifier les items
	items, ok := order["items"].([]interface{})
	if !ok || len(items) != 2 {
		t.Fatalf("? Nombre d'items incorrect: %v", items)
	}

	t.Log("? Commande r�cup�r�e et valid�e")
}
