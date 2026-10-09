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

	if val, _ := uc.settingsRepo.Get(ctx, "wallet.freeze_job.enabled"); val != nil {
		json.Unmarshal(val, &cfg.Enabled)
	}
	if val, _ := uc.settingsRepo.Get(ctx, "wallet.freeze_job.policy"); val != nil {
		json.Unmarshal(val, &cfg.Policy)
	}
	if val, _ := uc.settingsRepo.Get(ctx, "wallet.freeze_job.max_per_run"); val != nil {
		json.Unmarshal(val, &cfg.MaxPerRun)
	}
	if val, _ := uc.settingsRepo.Get(ctx, "wallet.freeze_job.batch_size"); val != nil {
		json.Unmarshal(val, &cfg.BatchSize)
	}
	return cfg, nil
}

func (uc *FreezeJobUsecase) UpdateConfig(ctx context.Context, cfg *FreezeJobConfig) error {
	setJSON := func(key string, val interface{}) error {
		b, _ := json.Marshal(val)
		updatedBy, _ := utils.UserIDFromContext(ctx)
		if updatedBy == "" {
			updatedBy = "super_admin"
		}
		return uc.settingsRepo.Set(ctx, key, b, &updatedBy)
	}

	if err := setJSON("wallet.freeze_job.enabled", cfg.Enabled); err != nil {
		return fmt.Errorf("failed to update enabled: %w", err)
	}
	if err := setJSON("wallet.freeze_job.policy", cfg.Policy); err != nil {
		return fmt.Errorf("failed to update policy: %w", err)
	}
	if err := setJSON("wallet.freeze_job.max_per_run", cfg.MaxPerRun); err != nil {
		return fmt.Errorf("failed to update max_per_run: %w", err)
	}
	if err := setJSON("wallet.freeze_job.batch_size", cfg.BatchSize); err != nil {
		return fmt.Errorf("failed to update batch_size: %w", err)
	}
	return nil
}

// RunJob déclenche le scheduler manuellement (en arrière-plan)
// 🛡️ CORRECTION P1 : Vérifie les droits RBAC avant de lancer le job
func (uc *FreezeJobUsecase) RunJob(ctx context.Context, mode string, isSuperAdmin bool) error {
	if !isSuperAdmin {
		// Un admin délégué ne peut pas forcer l'exécution si le job est désactivé
		cfg, err := uc.GetConfig(ctx)
		if err != nil {
			return fmt.Errorf("failed to read freeze job config: %w", err)
		}
		if !cfg.Enabled {
			return errors.New("freeze job is disabled by super admin")
		}
	}

	// Lancer en goroutine pour ne pas bloquer la requête HTTP
	// On utilise context.Background() pour que la goroutine survive à la fin de la requête HTTP
	go uc.scheduler.RunManual(context.Background(), mode)

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
		if w.FrozenUntil != nil {
			daysExpired = int(time.Since(*w.FrozenUntil).Hours() / 24)
		}
		result = append(result, &GraceExpiredWallet{
			ShopID:      w.ShopID,
			FrozenAt:    w.FrozenAt.Format(time.RFC3339),
			FrozenUntil: w.FrozenUntil.Format(time.RFC3339),
			DebtCents:   w.DebtCents,
			DaysExpired: daysExpired,
		})
	}
	return result, nil
}
