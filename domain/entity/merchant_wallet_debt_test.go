package entity

import "testing"

func TestCreditWithDebtSweep(t *testing.T) {
	w := &MerchantWallet{BalanceCents: 0, DebtCents: 40000, HeldCents: 0}
	net, swept, err := w.CreditWithDebtSweep(95000)
	if err != nil {
		t.Fatal(err)
	}
	if swept != 40000 || net != 55000 {
		t.Fatalf("swept=%d net=%d", swept, net)
	}
	if w.BalanceCents != 55000 || w.DebtCents != 0 {
		t.Fatalf("balance=%d debt=%d", w.BalanceCents, w.DebtCents)
	}
}

func TestApplyClawbackToDebt(t *testing.T) {
	w := &MerchantWallet{BalanceCents: 30000, DebtCents: 0}
	from, debt, err := w.ApplyClawbackToDebt(95000)
	if err != nil {
		t.Fatal(err)
	}
	if from != 30000 || debt != 65000 {
		t.Fatalf("from=%d toDebt=%d", from, debt)
	}
	if w.BalanceCents != 0 || w.DebtCents != 65000 {
		t.Fatalf("balance=%d debt=%d", w.BalanceCents, w.DebtCents)
	}
}

func TestAvailableCentsWithDebt(t *testing.T) {
	w := &MerchantWallet{BalanceCents: 100000, HeldCents: 10000, DebtCents: 40000}
	if w.AvailableCents() != 50000 {
		t.Fatalf("available=%d", w.AvailableCents())
	}
}
