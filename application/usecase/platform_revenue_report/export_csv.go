package reportusecase

import (
	"bytes"
	"encoding/csv"
	"fmt"
	"strconv"
	"time"

	reportingdto "Goshop/application/dto/reporting_dto"
)

func ExportPlatformCommissionsCSV(items []reportingdto.PlatformRevenueTransactionItem) ([]byte, error) {
	var buf bytes.Buffer
	// BOM Excel FR
	buf.Write([]byte{0xEF, 0xBB, 0xBF})
	w := csv.NewWriter(&buf)
	_ = w.Write([]string{
		"id", "transaction_type", "amount_cents", "amount_xof",
		"reference_type", "reference_id", "description", "created_at",
	})
	for _, t := range items {
		_ = w.Write([]string{
			t.ID,
			t.TransactionType,
			strconv.FormatInt(t.AmountCents, 10),
			fmt.Sprintf("%.2f", float64(t.AmountCents)/100.0),
			t.ReferenceType,
			t.ReferenceID,
			t.Description,
			t.CreatedAt.UTC().Format(time.RFC3339),
		})
	}
	w.Flush()
	return buf.Bytes(), w.Error()
}

func ExportWalletTransactionsCSV(items []reportingdto.WalletTransactionItem) ([]byte, error) {
	var buf bytes.Buffer
	buf.Write([]byte{0xEF, 0xBB, 0xBF})
	w := csv.NewWriter(&buf)
	_ = w.Write([]string{
		"id", "shop_id", "transaction_type", "amount_cents", "amount_xof",
		"balance_after_cents", "status", "reference_type", "reference_id",
		"description", "created_at",
	})
	for _, t := range items {
		refType, refID, desc := "", "", ""
		if t.ReferenceType != nil {
			refType = *t.ReferenceType
		}
		if t.ReferenceID != nil {
			refID = *t.ReferenceID
		}
		if t.Description != nil {
			desc = *t.Description
		}
		_ = w.Write([]string{
			t.ID,
			t.ShopID,
			t.TransactionType,
			strconv.FormatInt(t.AmountCents, 10),
			fmt.Sprintf("%.2f", float64(t.AmountCents)/100.0),
			strconv.FormatInt(t.BalanceAfterCents, 10),
			t.Status,
			refType,
			refID,
			desc,
			t.CreatedAt.UTC().Format(time.RFC3339),
		})
	}
	w.Flush()
	return buf.Bytes(), w.Error()
}
