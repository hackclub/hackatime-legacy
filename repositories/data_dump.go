package repositories

import (
	"time"

	"github.com/hackclub/hackatime/models"
	"gorm.io/gorm"
)

type DataDumpRepository struct {
	db *gorm.DB
}

func NewDataDumpRepository(db *gorm.DB) *DataDumpRepository {
	return &DataDumpRepository{db: db}
}

func (r *DataDumpRepository) Insert(dump *models.DataDump) (*models.DataDump, error) {
	return dump, r.db.Create(dump).Error
}

func (r *DataDumpRepository) Update(dump *models.DataDump) (*models.DataDump, error) {
	return dump, r.db.Save(dump).Error
}

func (r *DataDumpRepository) GetByUser(userId string) ([]*models.DataDump, error) {
	var dumps []*models.DataDump
	if err := r.db.Where("user_id = ?", userId).Order("created_at DESC").Find(&dumps).Error; err != nil {
		return nil, err
	}
	return dumps, nil
}

func (r *DataDumpRepository) GetById(id string) (*models.DataDump, error) {
	var dump models.DataDump
	if err := r.db.Where("id = ?", id).First(&dump).Error; err != nil {
		return nil, err
	}
	return &dump, nil
}

func (r *DataDumpRepository) GetStuckDumps(threshold time.Time) ([]*models.DataDump, error) {
	var dumps []*models.DataDump
	if err := r.db.Where("is_processing = ? AND created_at < ? AND is_stuck = ?", true, threshold, false).Find(&dumps).Error; err != nil {
		return nil, err
	}
	return dumps, nil
}

func (r *DataDumpRepository) GetExpiredDumps() ([]*models.DataDump, error) {
	var dumps []*models.DataDump
	if err := r.db.Where("expires IS NOT NULL AND expires < ?", time.Now()).Find(&dumps).Error; err != nil {
		return nil, err
	}
	return dumps, nil
}

func (r *DataDumpRepository) Delete(id string) error {
	return r.db.Where("id = ?", id).Delete(&models.DataDump{}).Error
}

func (r *DataDumpRepository) DeleteByUser(userId string) error {
	return r.db.Where("user_id = ?", userId).Delete(&models.DataDump{}).Error
}
