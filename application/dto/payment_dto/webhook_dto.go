package paymentdto

// WebhookResponse représente la réponse à un webhook
type WebhookResponse struct {
	Status  string `json:"status"`
	Message string `json:"message,omitempty"`
}
