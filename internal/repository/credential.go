package repository

import (
	credentialprovider "PandoraHelper/internal/provider/credential"
	"PandoraHelper/internal/model"
	"context"
	"errors"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

func NewCredentialRepository(repository *Repository) credentialprovider.Store {
	return &credentialRepository{Repository: repository}
}

type credentialRepository struct {
	*Repository
}

func (r *credentialRepository) Get(ctx context.Context, accountID uint) (*credentialprovider.Record, error) {
	var row model.AccountCredential
	if err := r.DB(ctx).Where("account_id = ?", accountID).First(&row).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, credentialprovider.ErrCredentialNotFound
		}
		return nil, err
	}
	return &credentialprovider.Record{
		AccountID:  row.AccountID,
		Kind:       row.Kind,
		Ciphertext: row.Ciphertext,
		State:      credentialprovider.State(row.State),
		Message:    row.Message,
		CheckedAt:  row.CheckedAt,
	}, nil
}

func (r *credentialRepository) Upsert(ctx context.Context, record *credentialprovider.Record) error {
	row := model.AccountCredential{
		AccountID:  record.AccountID,
		Kind:       record.Kind,
		Ciphertext: record.Ciphertext,
		State:      string(record.State),
		Message:    record.Message,
		CheckedAt:  record.CheckedAt,
	}
	return r.DB(ctx).Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "account_id"}},
		DoUpdates: clause.Assignments(map[string]interface{}{
			"kind":       row.Kind,
			"ciphertext": row.Ciphertext,
			"state":      row.State,
			"message":    row.Message,
			"checked_at": row.CheckedAt,
		}),
	}).Create(&row).Error
}

func (r *credentialRepository) Delete(ctx context.Context, accountID uint) error {
	return r.DB(ctx).Where("account_id = ?", accountID).Delete(&model.AccountCredential{}).Error
}

func (r *credentialRepository) UpdateHealth(ctx context.Context, accountID uint, health credentialprovider.Health) error {
	result := r.DB(ctx).Model(&model.AccountCredential{}).
		Where("account_id = ?", accountID).
		Updates(map[string]interface{}{
			"state":      string(health.State),
			"message":    health.Message,
			"checked_at": health.CheckedAt,
		})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return credentialprovider.ErrCredentialNotFound
	}
	return nil
}
