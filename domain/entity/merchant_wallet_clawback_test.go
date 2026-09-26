package entity

import "testing"

func TestCreditReducesNegativeBalance(t *testing.T) {
	w := NewMerchantWallet("shop-1")
	w.BalanceCents = -50_000
	w.MaxNegativeBalanceCents = -5_000_000

	if err := w.Credit(95_000); err != nil {
		t.Fatalf("Credit: %v", err)
	}
	if w.BalanceCents != 45_000 {
		t.Fatalf("balance=%d want 45000", w.BalanceCents)
	}
	if w.GetDebt() != 0 {
		t.Fatalf("debt=%d want 0", w.GetDebt())
	}
}

func TestCreditAllowedWhenFrozen(t *testing.T) {
	w := NewMerchantWallet("shop-1")
	w.BalanceCents = -10_000
	_ = w.Freeze(FreezeReasonNegativeBalance, "test")

	if err := w.Credit(20_000); err != nil {
		t.Fatalf("Credit while frozen should work for recovery: %v", err)
	}
	if w.BalanceCents != 10_000 {
		t.Fatalf("balance=%d", w.BalanceCents)
	}
}

func TestApplyClawbackGoesNegative(t *testing.T) {
	w := NewMerchantWallet("shop-1")
	w.BalanceCents = 95_000
	w.MaxNegativeBalanceCents = -5_000_000

	if err := w.ApplyClawback(97_500); err != nil {
		t.Fatalf("ApplyClawback: %v", err)
	}
	if w.BalanceCents != -2_500 {
		t.Fatalf("balance=%d want -2500", w.BalanceCents)
	}
	if w.GetDebt() != 2_500 {
		t.Fatalf("debt=%d", w.GetDebt())
	}
	if w.IsFrozen {
		t.Fatal("clawback must not freeze")
	}
}

func TestApplyClawbackRespectsMaxNegative(t *testing.T) {
	w := NewMerchantWallet("shop-1")
	w.BalanceCents = 0
	w.MaxNegativeBalanceCents = -1_000

	err := w.ApplyClawback(5_000)
	if err == nil {
		t.Fatal("expected error when exceeding max negative")
	}
}

func TestPayoutStillBlockedWhenFrozen(t *testing.T) {
	w := NewMerchantWallet("shop-1")
	w.BalanceCents = 50_000
	_ = w.Freeze(FreezeReasonAdminDecision, "test")

	if err := w.RequestPayout(10_000); err == nil {
		t.Fatal("payout must stay blocked when frozen")
	}
}
