package withdrawalusecase

import "context"

// ReversePayoutDebitForTest expose la méthode privée reversePayoutDebit
// uniquement pour le package de test _test, permettant de valider la logique de compensation.
func (uc *CreateWithdrawalUsecase) ReversePayoutDebitForTest(
	ctx context.Context,
	shopID string,
	amountCents int64,
	withdrawalID string,
	reason string,
) error {
	return uc.reversePayoutDebit(ctx, shopID, amountCents, withdrawalID, reason)
}
