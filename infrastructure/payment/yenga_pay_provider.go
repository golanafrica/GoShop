package payment

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"Goshop/domain/entity"

	"github.com/rs/zerolog"
)

// YengaPayProvider implémente l'API réelle de Yenga Pay
type YengaPayProvider struct {
	httpClient     *http.Client
	apiKey         string
	organizationID string
	projectID      string
	webhookSecret  string
	baseURL        string
	env            string
}

// YengaPayConfig configuration du provider Yenga Pay
type YengaPayConfig struct {
	APIKey         string
	OrganizationID string
	ProjectID      string
	WebhookSecret  string
	Env            string
}

// NewYengaPayProvider crée un nouveau provider Yenga Pay
func NewYengaPayProvider(config YengaPayConfig) (*YengaPayProvider, error) {
	if config.APIKey == "" {
		return nil, fmt.Errorf("Yenga Pay API key is required")
	}
	if config.OrganizationID == "" {
		return nil, fmt.Errorf("Yenga Pay organization ID is required")
	}
	if config.ProjectID == "" {
		return nil, fmt.Errorf("Yenga Pay project ID is required")
	}

	baseURL := "https://api.yengapay.com/api/v1"

	return &YengaPayProvider{
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
		apiKey:         config.APIKey,
		organizationID: config.OrganizationID,
		projectID:      config.ProjectID,
		webhookSecret:  config.WebhookSecret,
		baseURL:        baseURL,
		env:            config.Env,
	}, nil
}

// Code retourne le code du provider
func (p *YengaPayProvider) Code() entity.PaymentProvider {
	return entity.ProviderYengaPay
}

// InitiatePayment initie un paiement via Yenga Pay
func (p *YengaPayProvider) InitiatePayment(ctx context.Context, req *PaymentRequest) (*PaymentResponse, error) {
	logger := zerolog.Ctx(ctx)

	flow := "indirect"
	operator := ""
	customerMSISDN := req.PhoneNumber
	customerEmail := ""

	if req.Metadata != nil {
		if f, ok := req.Metadata["flow"].(string); ok {
			flow = f
		}
		if op, ok := req.Metadata["operator"].(string); ok {
			operator = op
		}
		if email, ok := req.Metadata["customer_email"].(string); ok {
			customerEmail = email
		}
	}

	amountFCFA := req.AmountCents / 100

	var providerRef, redirectURL, ussdCode, message string
	var status entity.PaymentStatus
	var metadata map[string]interface{}

	switch flow {
	case "indirect":
		resp, err := p.createPaymentIntent(ctx, amountFCFA, req)
		if err != nil {
			return nil, err
		}

		providerRef = resp.PaymentIntentID
		redirectURL = resp.CheckoutPageURL
		status = entity.PaymentStatusProcessing
		message = fmt.Sprintf("Redirigez le client vers %s pour compléter le paiement", redirectURL)
		metadata = map[string]interface{}{
			"payment_url":  redirectURL,
			"operator":     operator,
			"flow":         flow,
			"reference":    providerRef,
			"notification": message,
		}

	case "direct":
		if operator == "" {
			return nil, fmt.Errorf("operator is required for direct payment (ORANGE, MOOV, CORISM, SANKM, TELECEL)")
		}

		operatorCode := mapOperatorCode(operator)

		if operatorCode == "ORANGE" || operatorCode == "TELECEL" {
			otp, ok := req.Metadata["otp"].(string)
			if !ok || otp == "" {
				var ussdPattern string
				if operatorCode == "ORANGE" {
					ussdPattern = "*144*4*6*%d#"
				} else {
					ussdPattern = "*808*4*4*%d#"
				}
				ussdCode = fmt.Sprintf(ussdPattern, amountFCFA)
				return &PaymentResponse{
					ProviderRef: "",
					Status:      entity.PaymentStatusPending,
					USSDCode:    ussdCode,
					Metadata: map[string]interface{}{
						"flow":         "direct",
						"operator":     operatorCode,
						"flow_type":    "ONE_STEP",
						"notification": fmt.Sprintf("Composez %s pour générer le code OTP, puis relancez le paiement avec l'OTP", ussdCode),
					},
				}, nil
			}

			resp, err := p.directPaymentInitAndPay(ctx, amountFCFA, operatorCode, customerMSISDN, otp, customerEmail, req)
			if err != nil {
				return nil, err
			}

			switch resp.Status {
			case "DONE":
				providerRef = resp.TransactionID
				status = entity.PaymentStatusSuccess
				message = fmt.Sprintf("Paiement de %d FCFA réussi via %s", amountFCFA, operatorCode)
			case "OTP_SENT":
				providerRef = resp.PaymentIntentID
				status = entity.PaymentStatusProcessing
				message = fmt.Sprintf("OTP envoyé au %s. Veuillez finaliser le paiement", customerMSISDN)
			default:
				providerRef = resp.PaymentIntentID
				status = entity.PaymentStatusProcessing
				message = fmt.Sprintf("Paiement en cours via %s", operatorCode)
			}

			metadata = map[string]interface{}{
				"operator":     operatorCode,
				"flow":         flow,
				"reference":    providerRef,
				"notification": message,
			}
		} else {
			initResp, err := p.directPaymentInit(ctx, amountFCFA, operatorCode, customerEmail, req)
			if err != nil {
				return nil, err
			}

			providerRef = initResp.PaymentIntentID

			sendResp, err := p.directPaymentSendOTP(ctx, initResp.PaymentIntentID, operatorCode, customerMSISDN)
			if err != nil {
				return nil, err
			}

			status = entity.PaymentStatusProcessing
			message = fmt.Sprintf("%s. Code OTP reçu par SMS requis pour finaliser", sendResp.Message)

			metadata = map[string]interface{}{
				"operator":       operatorCode,
				"flow":           flow,
				"flow_type":      "TWO_STEP",
				"reference":      providerRef,
				"payment_intent": providerRef,
				"notification":   message,
				"next_step":      "Call POST /api/payments/{id}/complete with OTP",
			}
		}

	default:
		return nil, fmt.Errorf("invalid flow: %s (must be 'indirect' or 'direct')", flow)
	}

	logger.Info().
		Str("provider_ref", providerRef).
		Str("flow", flow).
		Str("operator", operator).
		Msg("Payment initiated via Yenga Pay")

	return &PaymentResponse{
		ProviderRef: providerRef,
		Status:      status,
		USSDCode:    ussdCode,
		RedirectURL: redirectURL,
		ExpiresAt:   time.Now().Add(30 * time.Minute).Unix(),
		Metadata:    metadata,
	}, nil
}

// ============ CASH-OUT (RETRAIT) ============

// CashOutRequest représente une demande de retrait
type CashOutRequest struct {
	AmountCents       int64
	PaymentMethod     string // "ORANGE_MONEY", "MOOV_MONEY", etc.
	DestinationNumber string
	DestinationName   string
	DestinationEmail  string
	Description       string
}

// CashOutResponse représente la réponse d'un retrait
type CashOutResponse struct {
	ProviderRef  string
	Status       string
	Amount       int64
	Fees         int64
	TotalDebited int64
	CreatedAt    string
}

// CashOut effectue un retrait vers Mobile Money via Yenga Pay
// Endpoint: POST /api/v1/groups/{organization_id}/cash-out
func (p *YengaPayProvider) CashOut(ctx context.Context, req *CashOutRequest) (*CashOutResponse, error) {
	logger := zerolog.Ctx(ctx)

	yengaReq := map[string]interface{}{
		"cashoutMethod": req.PaymentMethod,
		"amount":        req.AmountCents / 100, // Yenga Pay attend des FCFA
		"destNumber":    req.DestinationNumber,
		"groupId":       p.organizationID,
		"projectId":     p.projectID,
	}

	if req.Description != "" {
		yengaReq["description"] = req.Description
	}

	reqBody, err := json.Marshal(yengaReq)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	url := fmt.Sprintf("%s/groups/%s/cash-out", p.baseURL, p.organizationID)

	httpReq, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(reqBody))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("x-api-key", p.apiKey)

	logger.Info().
		Str("url", url).
		Int64("amount_cents", req.AmountCents).
		Str("payment_method", req.PaymentMethod).
		Msg("Calling Yenga Pay cash-out API")

	resp, err := p.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	switch resp.StatusCode {
	case http.StatusOK, http.StatusCreated:
		// OK
	default:
		return nil, fmt.Errorf("Yenga Pay cash-out error (status %d): %s", resp.StatusCode, string(respBody))
	}

	var yengaResp struct {
		ID              string  `json:"id"`
		Amount          int64   `json:"amount"`
		Fees            int64   `json:"fees"`
		TotalDebited    int64   `json:"totalDebited"`
		Status          string  `json:"status"`
		CreatedAt       string  `json:"createdAt"`
		DestNumber      string  `json:"destNumber"`
		CashoutMethod   string  `json:"cashoutMethod"`
		ErrorMessage    *string `json:"errorMessage"`
		OperatorTransID *string `json:"operatorTransId"`
	}

	if err := json.Unmarshal(respBody, &yengaResp); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	logger.Info().
		Str("provider_ref", yengaResp.ID).
		Str("status", yengaResp.Status).
		Int64("fees", yengaResp.Fees).
		Msg("Cash-out successful")

	return &CashOutResponse{
		ProviderRef:  yengaResp.ID,
		Status:       yengaResp.Status,
		Amount:       yengaResp.Amount,
		Fees:         yengaResp.Fees,
		TotalDebited: yengaResp.TotalDebited,
		CreatedAt:    yengaResp.CreatedAt,
	}, nil
}

// ============ AUTRES MÉTHODES (inchangées) ============

func (p *YengaPayProvider) createPaymentIntent(ctx context.Context, amountFCFA int64, req *PaymentRequest) (*yengaPaymentIntentResponse, error) {
	articles := []map[string]interface{}{
		{
			"title":       req.Description,
			"description": req.Description,
			"price":       amountFCFA,
		},
	}

	yengaReq := map[string]interface{}{
		"paymentAmount": amountFCFA,
		"reference":     req.PaymentID,
		"articles":      articles,
	}

	reqBody, err := json.Marshal(yengaReq)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	url := fmt.Sprintf("%s/groups/%s/payment-intent/%s", p.baseURL, p.organizationID, p.projectID)

	httpReq, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(reqBody))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("x-api-key", p.apiKey)

	resp, err := p.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	switch resp.StatusCode {
	case http.StatusOK, http.StatusCreated:
	default:
		return nil, fmt.Errorf("Yenga Pay API error (status %d): %s", resp.StatusCode, string(respBody))
	}

	var yengaResp yengaPaymentIntentResponse
	if err := json.Unmarshal(respBody, &yengaResp); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	yengaResp.PaymentIntentID = yengaResp.ID
	yengaResp.CheckoutPageURL = yengaResp.CheckoutPageURLWithPaymentToken

	return &yengaResp, nil
}

func (p *YengaPayProvider) directPaymentInit(ctx context.Context, amountFCFA int64, _ string, customerEmail string, req *PaymentRequest) (*yengaDirectInitResponse, error) {
	articles := []map[string]interface{}{
		{
			"title":       req.Description,
			"description": req.Description,
			"price":       amountFCFA,
		},
	}

	yengaReq := map[string]interface{}{
		"amount":    amountFCFA,
		"articles":  articles,
		"reference": req.PaymentID,
	}

	if customerEmail != "" {
		yengaReq["customerEmailToNotify"] = customerEmail
	}

	reqBody, err := json.Marshal(yengaReq)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	url := fmt.Sprintf("%s/groups/%s/projects/%s/direct-payment/init", p.baseURL, p.organizationID, p.projectID)

	httpReq, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(reqBody))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("x-api-key", p.apiKey)

	resp, err := p.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	switch resp.StatusCode {
	case http.StatusOK, http.StatusCreated:
	default:
		return nil, fmt.Errorf("Yenga Pay API error (status %d): %s", resp.StatusCode, string(respBody))
	}

	var yengaResp yengaDirectInitResponse
	if err := json.Unmarshal(respBody, &yengaResp); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &yengaResp, nil
}

func (p *YengaPayProvider) directPaymentSendOTP(ctx context.Context, paymentIntentID, operatorCode, customerMSISDN string) (*yengaSendOTPResponse, error) {
	phone := customerMSISDN
	if len(phone) > 0 && phone[0] == '+' {
		phone = phone[1:]
	}

	yengaReq := map[string]interface{}{
		"paymentIntentId": paymentIntentID,
		"operatorCode":    operatorCode,
		"countryCode":     "BF",
		"customerMSISDN":  phone,
	}

	reqBody, err := json.Marshal(yengaReq)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	url := fmt.Sprintf("%s/groups/%s/projects/%s/direct-payment/send-otp", p.baseURL, p.organizationID, p.projectID)

	httpReq, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(reqBody))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("x-api-key", p.apiKey)

	resp, err := p.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	switch resp.StatusCode {
	case http.StatusOK, http.StatusCreated:
	default:
		return nil, fmt.Errorf("Yenga Pay API error (status %d): %s", resp.StatusCode, string(respBody))
	}

	var yengaResp yengaSendOTPResponse
	if err := json.Unmarshal(respBody, &yengaResp); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &yengaResp, nil
}

func (p *YengaPayProvider) directPaymentInitAndPay(ctx context.Context, amountFCFA int64, operatorCode, customerMSISDN, otp, customerEmail string, req *PaymentRequest) (*yengaDirectPayResponse, error) {
	articles := []map[string]interface{}{
		{
			"title":       req.Description,
			"description": req.Description,
			"price":       amountFCFA,
		},
	}

	phone := customerMSISDN
	if len(phone) > 0 && phone[0] == '+' {
		phone = phone[1:]
	}

	yengaReq := map[string]interface{}{
		"amount":         amountFCFA,
		"articles":       articles,
		"reference":      req.PaymentID,
		"operatorCode":   operatorCode,
		"countryCode":    "BF",
		"customerMSISDN": phone,
		"otp":            otp,
	}

	if customerEmail != "" {
		yengaReq["customerEmailToNotify"] = customerEmail
	}

	reqBody, err := json.Marshal(yengaReq)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	url := fmt.Sprintf("%s/groups/%s/projects/%s/direct-payment/init-and-pay", p.baseURL, p.organizationID, p.projectID)

	httpReq, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(reqBody))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("x-api-key", p.apiKey)

	resp, err := p.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	switch resp.StatusCode {
	case http.StatusOK, http.StatusCreated:
	default:
		return nil, fmt.Errorf("Yenga Pay API error (status %d): %s", resp.StatusCode, string(respBody))
	}

	var yengaResp yengaDirectPayResponse
	if err := json.Unmarshal(respBody, &yengaResp); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &yengaResp, nil
}

func (p *YengaPayProvider) CompletePayment(ctx context.Context, paymentIntentID, operatorCode, customerMSISDN, otp string) (*CompletePaymentResponse, error) {
	phone := customerMSISDN
	if len(phone) > 0 && phone[0] == '+' {
		phone = phone[1:]
	}

	yengaReq := map[string]interface{}{
		"paymentIntentId": paymentIntentID,
		"operatorCode":    operatorCode,
		"countryCode":     "BF",
		"customerMSISDN":  phone,
		"otp":             otp,
	}

	reqBody, err := json.Marshal(yengaReq)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	url := fmt.Sprintf("%s/groups/%s/projects/%s/direct-payment/pay", p.baseURL, p.organizationID, p.projectID)

	httpReq, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(reqBody))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("x-api-key", p.apiKey)

	resp, err := p.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	switch resp.StatusCode {
	case http.StatusOK, http.StatusCreated:
	default:
		return nil, fmt.Errorf("Yenga Pay API error (status %d): %s", resp.StatusCode, string(respBody))
	}

	var yengaResp yengaDirectPayResponse
	if err := json.Unmarshal(respBody, &yengaResp); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &CompletePaymentResponse{
		Status:        yengaResp.Status,
		TransactionID: yengaResp.TransactionID,
		Amount:        yengaResp.Amount,
		Fees:          yengaResp.Fees,
		TotalAmount:   yengaResp.TotalAmount,
	}, nil
}

func (p *YengaPayProvider) CheckStatus(ctx context.Context, providerRef string) (*PaymentStatus, error) {
	logger := zerolog.Ctx(ctx)

	url := fmt.Sprintf("%s/groups/%s/payment-intent/project/%s/intent/%s", p.baseURL, p.organizationID, p.projectID, providerRef)

	httpReq, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	httpReq.Header.Set("x-api-key", p.apiKey)

	resp, err := p.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	switch resp.StatusCode {
	case http.StatusOK:
	case http.StatusNotFound:
		url = fmt.Sprintf("%s/groups/%s/merchant-payment/project/%s/payment/%s", p.baseURL, p.organizationID, p.projectID, providerRef)

		httpReq, err = http.NewRequestWithContext(ctx, "GET", url, nil)
		if err != nil {
			return nil, fmt.Errorf("failed to create request: %w", err)
		}

		httpReq.Header.Set("x-api-key", p.apiKey)

		resp, err = p.httpClient.Do(httpReq)
		if err != nil {
			return nil, fmt.Errorf("failed to send request: %w", err)
		}
		defer resp.Body.Close()

		respBody, err = io.ReadAll(resp.Body)
		if err != nil {
			return nil, fmt.Errorf("failed to read response: %w", err)
		}

		if resp.StatusCode != http.StatusOK {
			logger.Error().
				Int("status_code", resp.StatusCode).
				Str("response", string(respBody)).
				Msg("Yenga Pay API error")
			return nil, fmt.Errorf("Yenga Pay API error: %s", string(respBody))
		}
	default:
		logger.Error().
			Int("status_code", resp.StatusCode).
			Str("response", string(respBody)).
			Msg("Yenga Pay API error")
		return nil, fmt.Errorf("Yenga Pay API error (status %d): %s", resp.StatusCode, string(respBody))
	}

	var yengaResp struct {
		ID                string `json:"id"`
		PaymentIntentID   string `json:"paymentIntentId"`
		TransactionID     string `json:"transactionId"`
		TransactionStatus string `json:"transactionStatus"`
		Status            string `json:"status"`
		PaymentStatus     string `json:"paymentStatus"`
		PaymentAmount     int64  `json:"paymentAmount"`
		Amount            int64  `json:"amount"`
		Currency          string `json:"currency"`
	}

	if err := json.Unmarshal(respBody, &yengaResp); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	statusStr := yengaResp.TransactionStatus
	if statusStr == "" {
		statusStr = yengaResp.Status
	}
	if statusStr == "" {
		statusStr = yengaResp.PaymentStatus
	}

	status := mapYengaStatus(statusStr)

	amount := yengaResp.PaymentAmount
	if amount == 0 {
		amount = yengaResp.Amount
	}

	logger.Debug().
		Str("reference", providerRef).
		Str("yenga_status", statusStr).
		Str("mapped_status", string(status)).
		Msg("Payment status checked")

	return &PaymentStatus{
		ProviderRef: providerRef,
		Status:      status,
		AmountCents: amount * 100,
		Metadata: map[string]interface{}{
			"currency": yengaResp.Currency,
		},
	}, nil
}

func (p *YengaPayProvider) ValidateWebhook(ctx context.Context, payload []byte, signature string) (*WebhookEvent, error) {
	logger := zerolog.Ctx(ctx)

	if p.webhookSecret == "" {
		return nil, fmt.Errorf("webhook secret not configured")
	}

	if signature == "" {
		return nil, fmt.Errorf("missing webhook signature")
	}

	mac := hmac.New(sha256.New, []byte(p.webhookSecret))
	mac.Write(payload)
	expectedMAC := hex.EncodeToString(mac.Sum(nil))

	if !hmac.Equal([]byte(signature), []byte(expectedMAC)) {
		logger.Warn().
			Str("expected", expectedMAC).
			Str("received", signature).
			Msg("Invalid webhook signature")
		return nil, fmt.Errorf("invalid webhook signature")
	}

	var webhookData struct {
		// Pay-in
		APIEnv          string `json:"apiEnv"`
		PaymentStatus   string `json:"paymentStatus"`
		TransID         string `json:"transId"`
		ProjectID       string `json:"projectId"`
		PaymentIntentID string `json:"paymentIntentId"`
		PaymentSource   string `json:"paymentSource"`
		CustomerNumber  string `json:"customerNumber"`
		PaymentAmount   int64  `json:"paymentAmount"`
		PaymentFees     int64  `json:"paymentFees"`
		CountryOrigin   string `json:"contryOrigin"`
		Reference       string `json:"reference"`
		Currency        string `json:"currency"`

		// Payout (cash-out)
		ID              string `json:"id"`
		Status          string `json:"status"`
		OperatorTransID string `json:"operatorTransId"`
		Amount          int64  `json:"amount"`
		Fees            int64  `json:"fees"`
	}

	if err := json.Unmarshal(payload, &webhookData); err != nil {
		return nil, fmt.Errorf("invalid webhook payload: %w", err)
	}

	// Ref: pay-in (transId / paymentIntentId) ou payout (id)
	providerRef := webhookData.TransID
	if providerRef == "" {
		providerRef = webhookData.PaymentIntentID
	}
	if providerRef == "" {
		providerRef = webhookData.ID
	}
	if providerRef == "" {
		return nil, fmt.Errorf("missing transaction ID in webhook")
	}

	// Statut: paymentStatus (pay-in) ou status (payout)
	statusRaw := webhookData.PaymentStatus
	if statusRaw == "" {
		statusRaw = webhookData.Status
	}
	status := mapYengaStatus(statusRaw)

	amountCents := webhookData.PaymentAmount * 100
	if amountCents == 0 && webhookData.Amount > 0 {
		amountCents = webhookData.Amount * 100
	}

	logger.Info().
		Str("transaction_id", webhookData.TransID).
		Str("payment_intent_id", webhookData.PaymentIntentID).
		Str("payout_id", webhookData.ID).
		Str("operator_trans_id", webhookData.OperatorTransID).
		Str("status", string(status)).
		Msg("Webhook validated successfully")

	return &WebhookEvent{
		Provider:    entity.ProviderYengaPay,
		EventType:   "payment.completed",
		ProviderRef: providerRef,
		ExternalID:  firstNonEmpty(webhookData.TransID, webhookData.ID),
		Payload:     payload,
		Signature:   signature,
		Status:      status,
		AmountCents: amountCents,
		Metadata: map[string]interface{}{
			"payment_source":  webhookData.PaymentSource,
			"customer_number": webhookData.CustomerNumber,
			"payment_fees":    webhookData.PaymentFees,
			"country_origin":  webhookData.CountryOrigin,
			"reference":       webhookData.Reference,
			"currency":        webhookData.Currency,
			"api_env":         webhookData.APIEnv,
			"id":              webhookData.ID,
			"operatorTransId": webhookData.OperatorTransID,
			"status":          webhookData.Status,
			"amount":          webhookData.Amount,
			"fees":            webhookData.Fees,
		},
	}, nil
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if v != "" {
			return v
		}
	}
	return ""
}

// Refund initie un remboursement via l'API de Yenga Pay
// Basé sur la structure de l'API YengaPay (voir méthode CheckStatus fallback)
// Refund initie un remboursement vers le client via l'API Cash-Out de Yenga Pay
// Refund initie un remboursement vers le client via l'API Cash-Out de Yenga Pay
// Refund initie un remboursement vers le client via l'API Cash-Out de Yenga Pay
func (p *YengaPayProvider) Refund(ctx context.Context, providerRef string, amountCents int64, customerPhone string, operator string) error {
	logger := zerolog.Ctx(ctx)

	amountFCFA := float64(amountCents) / 100.0

	// ✅ FORMATAGE DU NUMÉRO : YengaPay Cash-Out exige le format international avec '+'
	destNumber := customerPhone
	if len(destNumber) > 0 && destNumber[0] != '+' {
		destNumber = "+" + destNumber
	}

	// ✅ DÉTERMINATION DYNAMIQUE DE L'OPÉRATEUR POUR LE CASH-OUT
	cashoutMethod := "ORANGE_MONEY" // Valeur par défaut de secours
	if operator != "" {
		switch strings.ToUpper(operator) {
		case "ORANGE", "ORANGE_MONEY":
			cashoutMethod = "ORANGE_MONEY"
		case "MOOV", "MOOV_MONEY":
			cashoutMethod = "MOOV_MONEY"
		case "TELECEL":
			cashoutMethod = "TELECEL"
		case "CORIS", "CORIS_MONEY":
			cashoutMethod = "CORIS_MONEY"
		case "SANK", "SANK_MONEY":
			cashoutMethod = "SANK_MONEY"
		default:
			cashoutMethod = strings.ToUpper(operator)
		}
	}

	yengaReq := map[string]interface{}{
		"cashoutMethod": cashoutMethod,
		"amount":        amountFCFA,
		"destNumber":    destNumber,
		"groupId":       p.organizationID,
		"projectId":     p.projectID,
		"description":   fmt.Sprintf("Refund for payment %s", providerRef),
	}

	reqBody, err := json.Marshal(yengaReq)
	if err != nil {
		return fmt.Errorf("failed to marshal refund request: %w", err)
	}

	url := fmt.Sprintf("%s/groups/%s/cash-out", p.baseURL, p.organizationID)

	httpReq, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(reqBody))
	if err != nil {
		return fmt.Errorf("failed to create refund request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("x-api-key", p.apiKey)

	logger.Info().
		Str("url", url).
		Str("provider_ref", providerRef).
		Int64("amount_cents", amountCents).
		Str("dest_number", destNumber).
		Str("cashout_method", cashoutMethod).
		Msg("Calling Yenga Pay cash-out for refund")

	resp, err := p.httpClient.Do(httpReq)
	if err != nil {
		return fmt.Errorf("failed to send refund request: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read refund response: %w", err)
	}

	switch resp.StatusCode {
	case http.StatusOK, http.StatusCreated, http.StatusAccepted:
		// Succès
	default:
		logger.Error().
			Int("status_code", resp.StatusCode).
			Str("response", string(respBody)).
			Msg("Yenga Pay refund API error")
		return fmt.Errorf("Yenga Pay refund error (status %d): %s", resp.StatusCode, string(respBody))
	}

	logger.Info().
		Str("provider_ref", providerRef).
		Msg("Refund (cash-out) initiated successfully via Yenga Pay")

	return nil
}

func (p *YengaPayProvider) IsAvailable(ctx context.Context) bool {
	return p.apiKey != "" && p.organizationID != "" && p.projectID != ""
}

// ============ Helpers ============

func mapOperatorCode(operator string) string {
	switch operator {
	case "orange_money", "ORANGE":
		return "ORANGE"
	case "moov_money", "MOOV":
		return "MOOV"
	case "telecel", "TELECEL":
		return "TELECEL"
	case "coris_money", "CORISM":
		return "CORISM"
	case "sank_money", "SANKM":
		return "SANKM"
	case "mtn", "MTN":
		return "MTN"
	default:
		return operator
	}
}

func mapYengaStatus(yengaStatus string) entity.PaymentStatus {
	switch yengaStatus {
	case "PENDING", "pending", "initiated":
		return entity.PaymentStatusPending
	case "PROCESSING", "processing", "in_progress", "OTP_SENT":
		return entity.PaymentStatusProcessing
	case "DONE", "SUCCESS", "success", "completed", "paid":
		return entity.PaymentStatusSuccess
	case "FAILED", "failed", "error", "REJECTED":
		return entity.PaymentStatusFailed
	case "REFUNDED", "refunded":
		return entity.PaymentStatusRefunded
	case "CANCELLED", "cancelled", "canceled":
		return entity.PaymentStatusCancelled
	case "EXPIRED", "expired", "timeout":
		return entity.PaymentStatusExpired
	default:
		return entity.PaymentStatusPending
	}
}

// ============ Types de réponse Yenga Pay ============

type yengaPaymentIntentResponse struct {
	ID                              string `json:"id"`
	PaymentIntentID                 string
	CheckoutPageURLWithPaymentToken string `json:"checkoutPageUrlWithPaymentToken"`
	CheckoutPageURL                 string
	Token                           string `json:"token"`
	ProjectID                       string `json:"projectId"`
	PaymentAmount                   int64  `json:"paymentAmount"`
	PaymentFees                     int64  `json:"paymentFees"`
	Currency                        string `json:"currency"`
	TransactionStatus               string `json:"transactionStatus"`
	Reference                       string `json:"reference"`
}

type yengaDirectInitResponse struct {
	PaymentIntentID string `json:"paymentIntentId"`
	ExpiresAt       string `json:"expiresAt"`
}

type yengaSendOTPResponse struct {
	Status  string `json:"status"`
	Message string `json:"message"`
}

type yengaDirectPayResponse struct {
	Status          string `json:"status"`
	TransactionID   string `json:"transactionId"`
	PaymentIntentID string `json:"paymentIntentId"`
	Amount          int64  `json:"amount"`
	Fees            int64  `json:"fees"`
	TotalAmount     int64  `json:"totalAmount"`
	Flow            string `json:"flow"`
	Message         string `json:"message"`
}

func (p *YengaPayProvider) UpdateConfig(config YengaPayConfig) {
	if config.APIKey != "" {
		p.apiKey = config.APIKey
	}
	if config.OrganizationID != "" {
		p.organizationID = config.OrganizationID
	}
	if config.ProjectID != "" {
		p.projectID = config.ProjectID
	}
	if config.WebhookSecret != "" {
		p.webhookSecret = config.WebhookSecret
	}
	if config.Env != "" {
		p.env = config.Env
	}
}

func (p *YengaPayProvider) GetConfig() YengaPayConfig {
	return YengaPayConfig{
		APIKey:         p.apiKey,
		OrganizationID: p.organizationID,
		ProjectID:      p.projectID,
		WebhookSecret:  p.webhookSecret,
		Env:            p.env,
	}
}
