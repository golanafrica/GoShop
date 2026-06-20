package mock

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"math/big"
	"sync"
	"time"

	"Goshop/domain/entity"
	"Goshop/infrastructure/payment"
)

// OrangeMoneyProvider est un mock du provider Orange Money
// Il simule parfaitement le comportement d'un vrai provider
// pour permettre le développement et les tests sans API réelle
type OrangeMoneyProvider struct {
	mu           sync.RWMutex
	payments     map[string]*mockPayment
	autoSuccess  bool
	successDelay time.Duration
	failureRate  float64
	available    bool
}

type mockPayment struct {
	Ref         string
	Amount      int64
	Phone       string
	Status      entity.PaymentStatus
	CreatedAt   time.Time
	CompletedAt *time.Time
	Metadata    map[string]interface{}
}

// OrangeMoneyConfig configuration du mock
type OrangeMoneyConfig struct {
	AutoSuccess  bool
	SuccessDelay time.Duration
	FailureRate  float64
	Available    bool
}

// DefaultOrangeMoneyConfig retourne une configuration par défaut
func DefaultOrangeMoneyConfig() OrangeMoneyConfig {
	return OrangeMoneyConfig{
		AutoSuccess:  true,
		SuccessDelay: 2 * time.Second,
		FailureRate:  0.05,
		Available:    true,
	}
}

// NewOrangeMoneyProvider crée un nouveau mock Orange Money
func NewOrangeMoneyProvider(config OrangeMoneyConfig) *OrangeMoneyProvider {
	return &OrangeMoneyProvider{
		payments:     make(map[string]*mockPayment),
		autoSuccess:  config.AutoSuccess,
		successDelay: config.SuccessDelay,
		failureRate:  config.FailureRate,
		available:    config.Available,
	}
}

// Code retourne le code du provider
func (p *OrangeMoneyProvider) Code() entity.PaymentProvider {
	return entity.ProviderOrangeMoney
}

// InitiatePayment simule l'initiation d'un paiement
func (p *OrangeMoneyProvider) InitiatePayment(ctx context.Context, req *payment.PaymentRequest) (*payment.PaymentResponse, error) {
	if !p.available {
		return nil, payment.ErrProviderUnavailable
	}

	if !isValidBurkinaPhone(req.PhoneNumber) {
		return nil, payment.ErrInvalidPhoneNumber
	}

	ref := generateMockRef("OM")

	p.mu.Lock()
	p.payments[ref] = &mockPayment{
		Ref:       ref,
		Amount:    req.AmountCents,
		Phone:     req.PhoneNumber,
		Status:    entity.PaymentStatusProcessing,
		CreatedAt: time.Now(),
		Metadata:  req.Metadata,
	}
	p.mu.Unlock()

	if p.autoSuccess {
		go p.simulateCompletion(ref, req.CallbackURL)
	}

	expiresAt := time.Now().Add(30 * time.Minute).Unix()

	return &payment.PaymentResponse{
		ProviderRef: ref,
		Status:      entity.PaymentStatusProcessing,
		USSDCode:    "#144*111#" + ref + "#",
		ExpiresAt:   expiresAt,
		Metadata: map[string]interface{}{
			"merchant_code": "OM_MOCK_MERCHANT_001",
			"notification":  fmt.Sprintf("Composez %s pour valider le paiement de %d FCFA", "#144*111*"+ref+"#", req.AmountCents),
		},
	}, nil
}

// simulateCompletion simule la complétion d'un paiement
// simulateCompletion simule la complétion d'un paiement
// callbackURL est utilisé pour simuler la notification au marchand
func (p *OrangeMoneyProvider) simulateCompletion(ref, callbackURL string) {
	time.Sleep(p.successDelay)

	p.mu.Lock()
	defer p.mu.Unlock()

	mp, exists := p.payments[ref]
	if !exists {
		return
	}

	// Simulation d'échec aléatoire selon failureRate
	if p.failureRate > 0 && randFloat() < p.failureRate {
		mp.Status = entity.PaymentStatusFailed
		now := time.Now()
		mp.CompletedAt = &now
		mp.Metadata["failure_reason"] = "insufficient_funds"
		mp.Metadata["callback_url"] = callbackURL // ✅ Utilisé pour debug
		return
	}

	// Succès
	mp.Status = entity.PaymentStatusSuccess
	now := time.Now()
	mp.CompletedAt = &now
	mp.Metadata["callback_url"] = callbackURL // ✅ Utilisé pour debug
}

// CheckStatus vérifie le statut d'un paiement
func (p *OrangeMoneyProvider) CheckStatus(ctx context.Context, providerRef string) (*payment.PaymentStatus, error) {
	if !p.available {
		return nil, payment.ErrProviderUnavailable
	}

	p.mu.RLock()
	defer p.mu.RUnlock()

	mp, exists := p.payments[providerRef]
	if !exists {
		return nil, fmt.Errorf("payment %s not found", providerRef)
	}

	var completedAt int64
	if mp.CompletedAt != nil {
		completedAt = mp.CompletedAt.Unix()
	}

	// ✅ FIX : Utiliser le type complet payment.PaymentStatus
	// La variable locale s'appelle maintenant "mp" (mock payment)
	// donc "payment.PaymentStatus" fait bien référence au package
	return &payment.PaymentStatus{
		ProviderRef:   providerRef,
		Status:        mp.Status,
		AmountCents:   mp.Amount,
		CompletedAt:   completedAt,
		FailureReason: getStringFromMap(mp.Metadata, "failure_reason"),
		Metadata:      mp.Metadata,
	}, nil
}

// ValidateWebhook valide un webhook (simulé)
func (p *OrangeMoneyProvider) ValidateWebhook(ctx context.Context, payload []byte, signature string) (*payment.WebhookEvent, error) {
	if !p.available {
		return nil, payment.ErrProviderUnavailable
	}

	return &payment.WebhookEvent{
		Provider:    entity.ProviderOrangeMoney,
		EventType:   "payment.success",
		ProviderRef: "MOCK-REF-" + generateMockRef(""),
		ExternalID:  "WH-" + generateMockRef(""),
		Payload:     payload,
		Signature:   signature,
		Status:      entity.PaymentStatusSuccess,
		Metadata: map[string]interface{}{
			"mock": true,
		},
	}, nil
}

// Refund simule un remboursement
func (p *OrangeMoneyProvider) Refund(ctx context.Context, providerRef string, amountCents int64) error {
	if !p.available {
		return payment.ErrProviderUnavailable
	}

	p.mu.Lock()
	defer p.mu.Unlock()

	mp, exists := p.payments[providerRef]
	if !exists {
		return fmt.Errorf("payment %s not found", providerRef)
	}

	if mp.Status != entity.PaymentStatusSuccess {
		return fmt.Errorf("can only refund successful payments")
	}

	if amountCents > mp.Amount {
		return fmt.Errorf("refund amount exceeds original payment")
	}

	mp.Status = entity.PaymentStatusRefunded
	now := time.Now()
	mp.CompletedAt = &now
	mp.Metadata["refund_amount"] = amountCents

	return nil
}

// IsAvailable vérifie si le provider est disponible
func (p *OrangeMoneyProvider) IsAvailable(ctx context.Context) bool {
	return p.available
}

// SetAvailable permet de toggler la disponibilité (pour tests)
func (p *OrangeMoneyProvider) SetAvailable(available bool) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.available = available
}

// GetPayment retourne un paiement mock (pour tests)
func (p *OrangeMoneyProvider) GetPayment(ref string) (*mockPayment, bool) {
	p.mu.RLock()
	defer p.mu.RUnlock()
	mp, exists := p.payments[ref]
	return mp, exists
}

// ============ Helpers ============

func generateMockRef(prefix string) string {
	bytes := make([]byte, 8)
	rand.Read(bytes)
	return prefix + "-" + hex.EncodeToString(bytes)
}

func isValidBurkinaPhone(phone string) bool {
	if len(phone) < 8 {
		return false
	}
	return true
}

func getStringFromMap(m map[string]interface{}, key string) string {
	if v, ok := m[key]; ok {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return ""
}

// randFloat retourne un float64 entre 0 et 1
// ✅ FIX : Utilisation de crypto/rand pour générer un entier puis conversion
func randFloat() float64 {
	n, err := rand.Int(rand.Reader, big.NewInt(1000000))
	if err != nil {
		return 0.5
	}
	return float64(n.Int64()) / 1000000.0
}
