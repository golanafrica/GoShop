package freezejobusecase

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	appscheduler "Goshop/application/scheduler"
	"Goshop/domain/repository"
	"Goshop/interfaces/utils"
)

type FreezeJobConfig struct {
	Enabled   bool   `json:"enabled"`
	Policy    string `json:"policy"`
	MaxPerRun int    `json:"max_per_run"`
	BatchSize int    `json:"batch_size"`
}

type GraceExpiredWallet struct {
	ShopID      string `json:"shop_id"`
	FrozenAt    string `json:"frozen_at"`
	FrozenUntil string `json:"frozen_until"`
	DebtCents   int64  `json:"debt_cents"`
	DaysExpired int    `json:"days_expired"`
}

type FreezeJobUsecase struct {
	settingsRepo repository.PlatformSettingsRepository
	walletRepo   repository.MerchantWalletRepository
	scheduler    *appscheduler.WalletGraceExpiryScheduler
}

func NewFreezeJobUsecase(
	settingsRepo repository.PlatformSettingsRepository,
	walletRepo repository.MerchantWalletRepository,
	sched *appscheduler.WalletGraceExpiryScheduler,
) *FreezeJobUsecase {
	return &FreezeJobUsecase{
		settingsRepo: settingsRepo,
		walletRepo:   walletRepo,
		scheduler:    sched,
	}
}

func (uc *FreezeJobUsecase) GetConfig(ctx context.Context) (*FreezeJobConfig, error) {
	cfg := &FreezeJobConfig{Enabled: false, Policy: "alert_only", MaxPerRun: 500, BatchSize: 100}

	getVal := func(key string, target interface{}) error {
		val, err := uc.settingsRepo.Get(ctx, key)
		if err != nil {
			return fmt.Errorf("failed to read %s: %w", key, err)
		}
		if val != nil {
			if err := json.Unmarshal(val, target); err != nil {
				return fmt.Errorf("failed to unmarshal %s: %w", key, err)
			}
		}
		return nil
	}

	if err := getVal("wallet.freeze_job.enabled", &cfg.Enabled); err != nil {
		return nil, err
	}
	if err := getVal("wallet.freeze_job.policy", &cfg.Policy); err != nil {
		return nil, err
	}
	if err := getVal("wallet.freeze_job.max_per_run", &cfg.MaxPerRun); err != nil {
		return nil, err
	}
	if err := getVal("wallet.freeze_job.batch_size", &cfg.BatchSize); err != nil {
		return nil, err
	}

	return cfg, nil
}

func (uc *FreezeJobUsecase) UpdateConfig(ctx context.Context, cfg *FreezeJobConfig) error {
	if cfg == nil {
		return errors.New("config cannot be nil")
	}

	updatedBy, ok := utils.UserIDFromContext(ctx)
	if !ok || updatedBy == "" {
		return errors.New("unauthorized: valid user identity required to update config")
	}

	if cfg.Policy != "alert_only" && cfg.Policy != "auto_freeze" {
		return errors.New("invalid policy: must be 'alert_only' or 'auto_freeze'")
	}
	if cfg.MaxPerRun <= 0 {
		return errors.New("invalid max_per_run: must be > 0")
	}
	if cfg.BatchSize <= 0 {
		return errors.New("invalid batch_size: must be > 0")
	}

	settings := make(map[string]json.RawMessage)

	bEnabled, _ := json.Marshal(cfg.Enabled)
	settings["wallet.freeze_job.enabled"] = bEnabled

	bPolicy, _ := json.Marshal(cfg.Policy)
	settings["wallet.freeze_job.policy"] = bPolicy

	bMax, _ := json.Marshal(cfg.MaxPerRun)
	settings["wallet.freeze_job.max_per_run"] = bMax

	bBatch, _ := json.Marshal(cfg.BatchSize)
	settings["wallet.freeze_job.batch_size"] = bBatch

	if err := uc.settingsRepo.UpdateMany(ctx, settings, &updatedBy); err != nil {
		return fmt.Errorf("failed to update config atomically: %w", err)
	}

	return nil
}

func (uc *FreezeJobUsecase) RunJob(ctx context.Context, mode string, isSuperAdmin bool) error {
	if !isSuperAdmin {
		cfg, err := uc.GetConfig(ctx)
		if err != nil {
			return fmt.Errorf("failed to read freeze job config: %w", err)
		}
		if !cfg.Enabled {
			return errors.New("freeze job is disabled by super admin")
		}
	}

	if uc.scheduler == nil {
		return errors.New("freeze job scheduler is not configured")
	}

	go func() {
		uc.scheduler.RunManual(context.Background(), mode)
	}()

	return nil
}

func (uc *FreezeJobUsecase) ListGraceExpired(ctx context.Context) ([]*GraceExpiredWallet, error) {
	wallets, err := uc.walletRepo.FindGracePeriodExpired(ctx)
	if err != nil {
		return nil, err
	}

	result := make([]*GraceExpiredWallet, 0, len(wallets))
	for _, w := range wallets {
		daysExpired := 0
		frozenUntilStr := ""
		frozenAtStr := ""

		// 🛡️ P1 FIX : PROTECTION CONTRE LE PANIC sur FrozenUntil
		if w.FrozenUntil != nil {
			daysExpired = int(time.Since(*w.FrozenUntil).Hours() / 24)
			frozenUntilStr = w.FrozenUntil.Format(time.RFC3339)
		}

		// 🛡️ P1 FIX : PROTECTION CONTRE LE PANIC sur FrozenAt (peut être nil sur d'anciennes données)
		if w.FrozenAt != nil {
			frozenAtStr = w.FrozenAt.Format(time.RFC3339)
		}

		result = append(result, &GraceExpiredWallet{
			ShopID:      w.ShopID,
			FrozenAt:    frozenAtStr,
			FrozenUntil: frozenUntilStr,
			DebtCents:   w.DebtCents,
			DaysExpired: daysExpired,
		})
	}
	return result, nil
}
