package scheduler

import (
	"testing"

	"Goshop/domain/entity"

	"github.com/stretchr/testify/assert"
)

// ============================================================
// TESTS : validateEscrowReleaseTxn (PR-A)
// ============================================================
//
// Ces tests verrouillent le comportement de la validation stricte d'idempotence
// lors de la libération automatique d'escrow. Aucun PostgreSQL requis :
// la fonction testée est pure.
//
// Convention comptable testée :
//   - expectedAmountCents = montant BRUT dû au marchand (escrow.GetMerchantAmount())
//   - txn.AmountCents     = montant NET crédité au solde après debt sweep
//   - txn.GrossAmountCents = montant BRUT tracé (NULL pour les transactions legacy)
// ============================================================

const (
	idemTestShopID  = "11111111-1111-1111-1111-111111111111"
	idemTestRefType = "escrow_auto_release"
	idemTestProofID = "proof-abc-123"
)

// newIdemTestTxn construit une transaction de libération d'escrow complétée.
// Si gross est nil, la transaction simule une écriture legacy (avant PR-C).
func newIdemTestTxn(net int64, gross *int64) *entity.WalletTransaction {
	refType := idemTestRefType
	refID := idemTestProofID

	return &entity.WalletTransaction{
		ID:                "txn-existing-1",
		ShopID:            idemTestShopID,
		TransactionType:   entity.WalletTxSaleCredit,
		AmountCents:       net,
		GrossAmountCents:  gross,
		BalanceAfterCents: net,
		ReferenceType:     &refType,
		ReferenceID:       &refID,
		Status:            entity.WalletTxCompleted,
	}
}

func int64Ptr(v int64) *int64 { return &v }

// ------------------------------------------------------------
// CAS NOMINAUX
// ------------------------------------------------------------

func TestValidateEscrowReleaseTxn_ValidFullCredit(t *testing.T) {
	// Crédit sans dette : brut = net = 10 000
	txn := newIdemTestTxn(10000, int64Ptr(10000))

	err := validateEscrowReleaseTxn(txn, idemTestShopID, 10000, idemTestRefType, idemTestProofID)

	assert.NoError(t, err, "un crédit complet identique doit être accepté comme idempotent")
}

func TestValidateEscrowReleaseTxn_ValidPartialDebtSweep(t *testing.T) {
	// Brut 10 000, dette 3 000 absorbée, net crédité 7 000.
	// La validation doit comparer le BRUT, pas le NET.
	txn := newIdemTestTxn(7000, int64Ptr(10000))

	err := validateEscrowReleaseTxn(txn, idemTestShopID, 10000, idemTestRefType, idemTestProofID)

	assert.NoError(t, err, "un debt sweep partiel légitime ne doit pas être rejeté")
}

func TestValidateEscrowReleaseTxn_ValidFullDebtSweep(t *testing.T) {
	// Brut 10 000 intégralement absorbé par la dette, net = 0.
	txn := newIdemTestTxn(0, int64Ptr(10000))

	err := validateEscrowReleaseTxn(txn, idemTestShopID, 10000, idemTestRefType, idemTestProofID)

	assert.NoError(t, err, "un debt sweep total légitime ne doit pas être rejeté")
}

// ------------------------------------------------------------
// REJET : TRANSACTION ABSENTE OU NON TERMINÉE
// ------------------------------------------------------------

func TestValidateEscrowReleaseTxn_NilTransaction(t *testing.T) {
	err := validateEscrowReleaseTxn(nil, idemTestShopID, 10000, idemTestRefType, idemTestProofID)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "nil")
}

func TestValidateEscrowReleaseTxn_StatusPending_Rejected(t *testing.T) {
	txn := newIdemTestTxn(10000, int64Ptr(10000))
	txn.Status = entity.WalletTxPending

	err := validateEscrowReleaseTxn(txn, idemTestShopID, 10000, idemTestRefType, idemTestProofID)

	assert.Error(t, err, "une transaction pending ne prouve pas un crédit effectué")
	assert.Contains(t, err.Error(), "not completed")
}

func TestValidateEscrowReleaseTxn_StatusFailed_Rejected(t *testing.T) {
	txn := newIdemTestTxn(10000, int64Ptr(10000))
	txn.Status = entity.WalletTxFailed

	err := validateEscrowReleaseTxn(txn, idemTestShopID, 10000, idemTestRefType, idemTestProofID)

	assert.Error(t, err, "une transaction failed ne doit jamais masquer un crédit non effectué")
	assert.Contains(t, err.Error(), "not completed")
}

func TestValidateEscrowReleaseTxn_StatusCancelled_Rejected(t *testing.T) {
	txn := newIdemTestTxn(10000, int64Ptr(10000))
	txn.Status = entity.WalletTxCancelled

	err := validateEscrowReleaseTxn(txn, idemTestShopID, 10000, idemTestRefType, idemTestProofID)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not completed")
}

// ------------------------------------------------------------
// REJET : DIVERGENCE D'IDENTITÉ FINANCIÈRE
// ------------------------------------------------------------

func TestValidateEscrowReleaseTxn_ShopMismatch_Rejected(t *testing.T) {
	txn := newIdemTestTxn(10000, int64Ptr(10000))
	txn.ShopID = "22222222-2222-2222-2222-222222222222"

	err := validateEscrowReleaseTxn(txn, idemTestShopID, 10000, idemTestRefType, idemTestProofID)

	assert.Error(t, err, "une transaction d'un autre marchand ne doit pas être acceptée")
	assert.Contains(t, err.Error(), "shop_id mismatch")
}

func TestValidateEscrowReleaseTxn_GrossAmountMismatch_Rejected(t *testing.T) {
	// La transaction existante porte un brut de 20 000, on attend 10 000.
	txn := newIdemTestTxn(20000, int64Ptr(20000))

	err := validateEscrowReleaseTxn(txn, idemTestShopID, 10000, idemTestRefType, idemTestProofID)

	assert.Error(t, err, "deux montants bruts différents ne sont pas la même opération")
	assert.Contains(t, err.Error(), "amount mismatch")
}

func TestValidateEscrowReleaseTxn_GrossAmountLowerThanExpected_Rejected(t *testing.T) {
	// Régression PR-C : un brut inférieur ne doit PLUS être accepté silencieusement.
	txn := newIdemTestTxn(5000, int64Ptr(5000))

	err := validateEscrowReleaseTxn(txn, idemTestShopID, 10000, idemTestRefType, idemTestProofID)

	assert.Error(t, err, "un brut inférieur au montant attendu doit être rejeté")
	assert.Contains(t, err.Error(), "amount mismatch")
}

func TestValidateEscrowReleaseTxn_GrossMismatchDespiteMatchingNet_Rejected(t *testing.T) {
	// Piège : le NET correspond au montant attendu, mais le BRUT diffère.
	// Sans lecture du brut, cette divergence serait masquée.
	txn := newIdemTestTxn(10000, int64Ptr(15000))

	err := validateEscrowReleaseTxn(txn, idemTestShopID, 10000, idemTestRefType, idemTestProofID)

	assert.Error(t, err, "le brut doit primer sur le net pour établir l'identité")
	assert.Contains(t, err.Error(), "amount mismatch")
}

// ------------------------------------------------------------
// REJET : DIVERGENCE DE RÉFÉRENCE
// ------------------------------------------------------------

func TestValidateEscrowReleaseTxn_ReferenceTypeMismatch_Rejected(t *testing.T) {
	txn := newIdemTestTxn(10000, int64Ptr(10000))
	other := "dispute_resolution"
	txn.ReferenceType = &other

	err := validateEscrowReleaseTxn(txn, idemTestShopID, 10000, idemTestRefType, idemTestProofID)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "reference_type mismatch")
}

func TestValidateEscrowReleaseTxn_ReferenceIDMismatch_Rejected(t *testing.T) {
	txn := newIdemTestTxn(10000, int64Ptr(10000))
	other := "proof-OTHER-999"
	txn.ReferenceID = &other

	err := validateEscrowReleaseTxn(txn, idemTestShopID, 10000, idemTestRefType, idemTestProofID)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "reference_id mismatch")
}

func TestValidateEscrowReleaseTxn_NilReferenceType_Rejected(t *testing.T) {
	txn := newIdemTestTxn(10000, int64Ptr(10000))
	txn.ReferenceType = nil

	err := validateEscrowReleaseTxn(txn, idemTestShopID, 10000, idemTestRefType, idemTestProofID)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "reference_type mismatch")
}

func TestValidateEscrowReleaseTxn_NilReferenceID_Rejected(t *testing.T) {
	txn := newIdemTestTxn(10000, int64Ptr(10000))
	txn.ReferenceID = nil

	err := validateEscrowReleaseTxn(txn, idemTestShopID, 10000, idemTestRefType, idemTestProofID)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "reference_id mismatch")
}

// ------------------------------------------------------------
// TRANSACTIONS LEGACY (GrossAmountCents NULL, avant PR-C)
// ------------------------------------------------------------

func TestValidateEscrowReleaseTxn_LegacyWithoutSweep_Accepted(t *testing.T) {
	// Écriture legacy sans dette : amount_cents correspond au brut réel.
	txn := newIdemTestTxn(10000, nil)

	err := validateEscrowReleaseTxn(txn, idemTestShopID, 10000, idemTestRefType, idemTestProofID)

	assert.NoError(t, err, "une legacy sans debt sweep reste vérifiable via amount_cents")
}

func TestValidateEscrowReleaseTxn_LegacyWithSweep_FailsClosed(t *testing.T) {
	// Écriture legacy ayant subi un debt sweep : amount_cents = net (7 000),
	// brut historique inconnu. On ne peut pas prouver l'identité → fail-closed.
	txn := newIdemTestTxn(7000, nil)

	err := validateEscrowReleaseTxn(txn, idemTestShopID, 10000, idemTestRefType, idemTestProofID)

	assert.Error(t, err, "une legacy ambiguë doit être bloquée, pas devinée")
	assert.Contains(t, err.Error(), "amount mismatch")
}

func TestValidateEscrowReleaseTxn_LegacyFullSweep_FailsClosed(t *testing.T) {
	// Legacy avec net = 0 : impossible de distinguer un full sweep d'une absence de crédit.
	txn := newIdemTestTxn(0, nil)

	err := validateEscrowReleaseTxn(txn, idemTestShopID, 10000, idemTestRefType, idemTestProofID)

	assert.Error(t, err, "une legacy à net nul doit être bloquée pour réconciliation manuelle")
	assert.Contains(t, err.Error(), "amount mismatch")
}

// ------------------------------------------------------------
// TABLE RÉCAPITULATIVE : SÉMANTIQUE DU MONTANT
// ------------------------------------------------------------

func TestValidateEscrowReleaseTxn_AmountSemanticsMatrix(t *testing.T) {
	tests := []struct {
		name        string
		net         int64
		gross       *int64
		expected    int64
		wantErr     bool
		description string
	}{
		{
			name:        "no_sweep_gross_tracked",
			net:         10000,
			gross:       int64Ptr(10000),
			expected:    10000,
			wantErr:     false,
			description: "brut = net, brut tracé",
		},
		{
			name:        "partial_sweep_gross_tracked",
			net:         7000,
			gross:       int64Ptr(10000),
			expected:    10000,
			wantErr:     false,
			description: "dette partielle, brut tracé → OK",
		},
		{
			name:        "full_sweep_gross_tracked",
			net:         0,
			gross:       int64Ptr(10000),
			expected:    10000,
			wantErr:     false,
			description: "dette totale, brut tracé → OK",
		},
		{
			name:        "gross_higher_than_expected",
			net:         10000,
			gross:       int64Ptr(20000),
			expected:    10000,
			wantErr:     true,
			description: "brut supérieur → rejet",
		},
		{
			name:        "gross_lower_than_expected",
			net:         5000,
			gross:       int64Ptr(5000),
			expected:    10000,
			wantErr:     true,
			description: "brut inférieur → rejet (régression PR-C)",
		},
		{
			name:        "legacy_no_sweep",
			net:         10000,
			gross:       nil,
			expected:    10000,
			wantErr:     false,
			description: "legacy sans dette → OK",
		},
		{
			name:        "legacy_partial_sweep",
			net:         7000,
			gross:       nil,
			expected:    10000,
			wantErr:     true,
			description: "legacy avec dette → fail-closed",
		},
		{
			name:        "legacy_full_sweep",
			net:         0,
			gross:       nil,
			expected:    10000,
			wantErr:     true,
			description: "legacy dette totale → fail-closed",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			txn := newIdemTestTxn(tt.net, tt.gross)

			err := validateEscrowReleaseTxn(txn, idemTestShopID, tt.expected, idemTestRefType, idemTestProofID)

			if tt.wantErr {
				assert.Error(t, err, tt.description)
			} else {
				assert.NoError(t, err, tt.description)
			}
		})
	}
}
