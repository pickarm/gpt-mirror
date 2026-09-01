package server

import (
	"PandoraHelper/internal/service"
	"PandoraHelper/pkg/log"
	"context"
	"github.com/go-co-op/gocron"
	"github.com/spf13/viper"
	"go.uber.org/zap"
	"time"
)

type Task struct {
	conf           *viper.Viper
	log            *log.Logger
	scheduler      *gocron.Scheduler
	accountService service.AccountService
	shareService   service.ShareService
}

func NewTask(conf *viper.Viper, log *log.Logger, accountService service.AccountService, shareService service.ShareService) *Task {
	return &Task{
		conf:           conf,
		log:            log,
		accountService: accountService,
		shareService:   shareService,
	}
}

// RefreshAllAccountEveryday keeps the legacy task name for scheduler/config
// compatibility. It now asks the credential provider to validate each account
// instead of inspecting or refreshing raw tokens in the account list response.
func (t *Task) RefreshAllAccountEveryday(ctx context.Context) error {
	accounts, err := t.accountService.SearchAccount(ctx, "chatgpt", "")
	if err != nil {
		return err
	}
	for _, account := range accounts {
		if !account.HasCredential {
			t.log.Warn("Account credential is not configured, skipped", zap.Int64("id", int64(account.ID)))
			continue
		}
		if err := t.accountService.RefreshAccount(ctx, int64(account.ID)); err != nil {
			t.log.Error("Account credential validation failed", zap.Int64("id", int64(account.ID)), zap.Error(err))
		}
	}
	t.log.Info("Account credential validation finished")
	return nil
}

func (t *Task) RefreshShareLimitEveryday(ctx context.Context) error {
	shares, err := t.shareService.SearchShare(ctx, "chatgpt", "", "")
	if err != nil {
		return err
	}
	for _, share := range shares {
		if !share.RefreshEveryday {
			continue
		}
		_, err = t.shareService.RefreshShareToken(ctx, share, "", true)
		if err != nil {
			t.log.Error(share.UniqueName+" Refresh Share Limit Error", zap.Error(err))
		}
	}
	t.log.Info("Refresh Share Limit Finish")
	return nil
}

func (t *Task) Start(ctx context.Context) error {
	gocron.SetPanicHandler(func(jobName string, recoverData interface{}) {
		t.log.Error("Task Panic", zap.String("job", jobName), zap.Any("recover", recoverData))
	})

	t.scheduler = gocron.NewScheduler(time.UTC)

	if t.conf.GetBool("account.refresh.enabled") {
		refreshCron := t.conf.GetString("account.refresh.cron")
		t.log.Info("automatic account credential validation enabled", zap.String("cron", refreshCron))
		var err error
		if refreshCron != "" {
			_, err = t.scheduler.CronWithSeconds(refreshCron).Do(t.RefreshAllAccountEveryday, ctx)
		} else {
			_, err = t.scheduler.Every(1).Day().At("00:00").Do(t.RefreshAllAccountEveryday, ctx)
		}
		if err != nil {
			return err
		}
	} else {
		t.log.Info("automatic account credential validation disabled")
	}

	if t.conf.GetBool("share.refresh.enabled") {
		_, err := t.scheduler.Every(1).Day().At("00:05").Do(t.RefreshShareLimitEveryday, ctx)
		if err != nil {
			return err
		}
		t.log.Info("automatic share refresh enabled")
	} else {
		t.log.Info("automatic share refresh disabled until a provider is configured")
	}

	t.scheduler.StartBlocking()
	return nil
}

func (t *Task) Stop(ctx context.Context) error {
	if t.scheduler != nil {
		t.scheduler.Stop()
	}
	t.log.Info("Task stop...")
	return nil
}
