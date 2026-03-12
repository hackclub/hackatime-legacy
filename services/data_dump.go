package services

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/gofrs/uuid/v5"
	"github.com/hackclub/hackatime/config"
	"github.com/hackclub/hackatime/models"
	wakatime "github.com/hackclub/hackatime/models/compat/wakatime/v1"
	"github.com/hackclub/hackatime/repositories"
	"github.com/muety/artifex/v2"
)

const dataDumpStuckThreshold = 30 * time.Minute

var (
	ErrInvalidDataDumpType       = errors.New("invalid dump type")
	ErrDataDumpAlreadyInProgress = errors.New("a data export is already in progress, please wait for it to complete")
)

type DataDumpService struct {
	config               *config.Config
	repository           repositories.IDataDumpRepository
	heartbeatService     IHeartbeatService
	summaryService       ISummaryService
	objectStorageService IObjectStorageService
	queue                *artifex.Dispatcher
}

func NewDataDumpService(
	dataDumpRepo repositories.IDataDumpRepository,
	heartbeatService IHeartbeatService,
	summaryService ISummaryService,
	objectStorageService IObjectStorageService,
) *DataDumpService {
	return &DataDumpService{
		config:               config.Get(),
		repository:           dataDumpRepo,
		heartbeatService:     heartbeatService,
		summaryService:       summaryService,
		objectStorageService: objectStorageService,
		queue:                config.GetQueue(config.QueueProcessing),
	}
}

func (srv *DataDumpService) GetByUser(userId string) ([]*models.DataDump, error) {
	srv.markStuckDumps()
	return srv.repository.GetByUser(userId)
}

func (srv *DataDumpService) markStuckDumps() {
	threshold := time.Now().Add(-dataDumpStuckThreshold)
	stuckDumps, err := srv.repository.GetStuckDumps(threshold)
	if err != nil {
		config.Log().Error("failed to check for stuck data dumps", "error", err)
		return
	}
	for _, dump := range stuckDumps {
		slog.Warn("marking data dump as stuck", "dumpID", dump.ID)
		dump.IsStuck = true
		srv.repository.Update(dump)
	}
}

func (srv *DataDumpService) CleanupExpired() error {
	expired, err := srv.repository.GetExpiredDumps()
	if err != nil {
		return err
	}

	for _, dump := range expired {
		slog.Info("cleaning up expired data dump", "dumpID", dump.ID, "userID", dump.UserID)
		if err := srv.repository.Delete(dump.ID); err != nil {
			config.Log().Error("failed to delete expired data dump", "dumpID", dump.ID, "error", err)
		}
	}

	if len(expired) > 0 {
		slog.Info("cleaned up expired data dumps", "count", len(expired))
	}

	return nil
}

func (srv *DataDumpService) Create(user *models.User, dumpType string) (*models.DataDump, error) {
	if dumpType != models.DataDumpTypeHeartbeats && dumpType != models.DataDumpTypeDaily {
		return nil, fmt.Errorf("%w: %s", ErrInvalidDataDumpType, dumpType)
	}

	existing, err := srv.repository.GetByUser(user.ID)
	if err != nil {
		return nil, err
	}
	for _, d := range existing {
		if d.IsProcessing && !d.IsStuck && !d.HasFailed {
			return nil, ErrDataDumpAlreadyInProgress
		}
	}

	id := uuid.Must(uuid.NewV4()).String()
	now := time.Now()
	expires := now.Add(24 * time.Hour)

	dump := &models.DataDump{
		ID:           id,
		UserID:       user.ID,
		Status:       models.DataDumpStatusPending,
		Type:         dumpType,
		IsProcessing: true,
		CreatedAt:    now,
		Expires:      &expires,
	}

	dump, err = srv.repository.Insert(dump)
	if err != nil {
		return nil, err
	}

	if err := srv.queue.Dispatch(func() {
		srv.process(dump, user)
	}); err != nil {
		config.Log().Error("failed to dispatch data dump job", "dumpID", dump.ID, "error", err)
		dump.Status = models.DataDumpStatusCompleted
		dump.HasFailed = true
		dump.IsProcessing = false
		srv.repository.Update(dump)
		return dump, err
	}

	return dump, nil
}

func (srv *DataDumpService) process(dump *models.DataDump, user *models.User) {
	slog.Info("starting data dump processing", "dumpID", dump.ID, "userID", user.ID, "type", dump.Type)

	dump.Status = models.DataDumpStatusProcessing
	dump.PercentComplete = 0
	srv.repository.Update(dump)

	var data []byte
	var err error

	if dump.Type == models.DataDumpTypeHeartbeats {
		data, err = srv.exportHeartbeats(dump, user)
	} else {
		data, err = srv.exportDaily(dump, user)
	}

	if err != nil {
		config.Log().Error("data dump processing failed", "dumpID", dump.ID, "error", err)
		dump.HasFailed = true
		dump.IsProcessing = false
		dump.Status = models.DataDumpStatusCompleted
		srv.repository.Update(dump)
		return
	}

	dump.Status = models.DataDumpStatusUploading
	dump.PercentComplete = 90
	srv.repository.Update(dump)

	if srv.objectStorageService == nil {
		config.Log().Error("object storage not configured, cannot upload data dump", "dumpID", dump.ID)
		dump.HasFailed = true
		dump.IsProcessing = false
		dump.Status = models.DataDumpStatusCompleted
		srv.repository.Update(dump)
		return
	}

	key := fmt.Sprintf("data_dumps/%s/%s.json", user.ID, dump.ID)
	downloadUrl, err := srv.objectStorageService.Upload(key, bytes.NewReader(data), "application/json")
	if err != nil {
		config.Log().Error("failed to upload data dump", "dumpID", dump.ID, "error", err)
		dump.HasFailed = true
		dump.IsProcessing = false
		dump.Status = models.DataDumpStatusCompleted
		srv.repository.Update(dump)
		return
	}

	dump.Status = models.DataDumpStatusCompleted
	dump.PercentComplete = 100
	dump.DownloadUrl = downloadUrl
	dump.IsProcessing = false
	srv.repository.Update(dump)

	slog.Info("data dump completed", "dumpID", dump.ID, "userID", user.ID)
}

func (srv *DataDumpService) exportHeartbeats(dump *models.DataDump, user *models.User) ([]byte, error) {
	from := user.CreatedAt.T()
	to := time.Now()

	heartbeats, err := srv.heartbeatService.GetAllWithin(from, to, user)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch heartbeats: %w", err)
	}

	dump.PercentComplete = 50
	srv.repository.Update(dump)

	compatEntries := wakatime.HeartbeatsToCompat(heartbeats)

	dayMap := make(map[string][]*wakatime.HeartbeatEntry)
	for _, entry := range compatEntries {
		day := time.Unix(int64(entry.Time), 0).Format("2006-01-02")
		dayMap[day] = append(dayMap[day], entry)
	}

	days := make([]*wakatime.JsonExportDay, 0, len(dayMap))
	for date, hbs := range dayMap {
		days = append(days, &wakatime.JsonExportDay{
			Date:       date,
			Heartbeats: hbs,
		})
	}

	export := &wakatime.JsonExportViewModel{
		Range: &wakatime.JsonExportRange{
			Start: from.Unix(),
			End:   to.Unix(),
		},
		Days: days,
	}

	dump.PercentComplete = 80
	srv.repository.Update(dump)

	return json.Marshal(export)
}

func (srv *DataDumpService) exportDaily(dump *models.DataDump, user *models.User) ([]byte, error) {
	from := user.CreatedAt.T()
	to := time.Now()

	summary, err := srv.summaryService.Retrieve(from, to, user, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve summary: %w", err)
	}

	dump.PercentComplete = 80
	srv.repository.Update(dump)

	return json.Marshal(summary)
}
