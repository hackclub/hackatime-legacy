package services

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
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
	dumps, err := srv.repository.GetByUser(userId)
	if err != nil {
		return nil, err
	}

	for _, dump := range dumps {
		srv.refreshDownloadURL(dump)
	}

	return dumps, nil
}

func (srv *DataDumpService) MarkStuckDumps() error {
	threshold := time.Now().Add(-dataDumpStuckThreshold)
	stuckDumps, err := srv.repository.GetStuckDumps(threshold)
	if err != nil {
		return err
	}
	for _, dump := range stuckDumps {
		slog.Warn("marking data dump as stuck", "dumpID", dump.ID)
		dump.IsStuck = true
		if _, err := srv.repository.Update(dump); err != nil {
			return err
		}
	}

	return nil
}

func (srv *DataDumpService) CleanupExpired() error {
	expired, err := srv.repository.GetExpiredDumps()
	if err != nil {
		return err
	}

	for _, dump := range expired {
		slog.Info("cleaning up expired data dump", "dumpID", dump.ID, "userID", dump.UserID)
		if err := srv.deleteObject(dump); err != nil {
			config.Log().Error("failed to delete expired data dump object", "dumpID", dump.ID, "error", err)
			continue
		}
		if err := srv.repository.Delete(dump.ID); err != nil {
			config.Log().Error("failed to delete expired data dump", "dumpID", dump.ID, "error", err)
		}
	}

	if len(expired) > 0 {
		slog.Info("cleaned up expired data dumps", "count", len(expired))
	}

	return nil
}

func (srv *DataDumpService) DeleteByUser(userID string) error {
	dumps, err := srv.repository.GetByUser(userID)
	if err != nil {
		return err
	}

	for _, dump := range dumps {
		if err := srv.deleteObject(dump); err != nil {
			return err
		}
	}

	return srv.repository.DeleteByUser(userID)
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

	if dump.Type == models.DataDumpTypeHeartbeats {
		if err := srv.uploadHeartbeatsDump(dump, user); err != nil {
			config.Log().Error("data dump processing failed", "dumpID", dump.ID, "error", err)
			dump.HasFailed = true
			dump.IsProcessing = false
			dump.Status = models.DataDumpStatusCompleted
			srv.repository.Update(dump)
			return
		}
	} else {
		if err := srv.uploadDailyDump(dump, user); err != nil {
			config.Log().Error("data dump processing failed", "dumpID", dump.ID, "error", err)
			dump.HasFailed = true
			dump.IsProcessing = false
			dump.Status = models.DataDumpStatusCompleted
			srv.repository.Update(dump)
			return
		}
	}

	dump.Status = models.DataDumpStatusCompleted
	dump.PercentComplete = 100
	dump.IsProcessing = false
	srv.refreshDownloadURL(dump)
	if _, err := srv.repository.Update(dump); err != nil {
		config.Log().Error("failed to upload data dump", "dumpID", dump.ID, "error", err)
	}

	slog.Info("data dump completed", "dumpID", dump.ID, "userID", user.ID)
}

func (srv *DataDumpService) uploadHeartbeatsDump(dump *models.DataDump, user *models.User) error {
	if srv.objectStorageService == nil {
		return errors.New("object storage not configured")
	}

	from := user.CreatedAt.T()
	to := time.Now()

	dump.PercentComplete = 50
	srv.repository.Update(dump)

	reader, writer := io.Pipe()
	defer reader.Close()
	go func() {
		err := srv.streamHeartbeatsExport(writer, from, to, user)
		if err != nil {
			_ = writer.CloseWithError(err)
			return
		}
		_ = writer.Close()
	}()

	dump.Status = models.DataDumpStatusUploading
	dump.PercentComplete = 90
	srv.repository.Update(dump)

	key := dataDumpObjectKey(user.ID, dump.ID)
	if err := srv.objectStorageService.Upload(key, reader, "application/json"); err != nil {
		return fmt.Errorf("failed to upload heartbeat dump: %w", err)
	}

	return nil
}

func (srv *DataDumpService) uploadDailyDump(dump *models.DataDump, user *models.User) error {
	if srv.objectStorageService == nil {
		return errors.New("object storage not configured")
	}

	from := user.CreatedAt.T()
	to := time.Now()

	summary, err := srv.summaryService.Retrieve(from, to, user, nil)
	if err != nil {
		return fmt.Errorf("failed to retrieve summary: %w", err)
	}

	dump.PercentComplete = 80
	srv.repository.Update(dump)

	dump.Status = models.DataDumpStatusUploading
	dump.PercentComplete = 90
	srv.repository.Update(dump)

	key := dataDumpObjectKey(user.ID, dump.ID)
	reader := jsonReader(summary)
	defer reader.Close()
	if err := srv.objectStorageService.Upload(key, reader, "application/json"); err != nil {
		return fmt.Errorf("failed to upload daily dump: %w", err)
	}

	return nil
}

func (srv *DataDumpService) streamHeartbeatsExport(w io.Writer, from, to time.Time, user *models.User) error {
	rangeJSON, err := json.Marshal(&wakatime.JsonExportRange{
		Start: from.Unix(),
		End:   to.Unix(),
	})
	if err != nil {
		return err
	}

	if _, err := io.WriteString(w, `{"range":`); err != nil {
		return err
	}
	if _, err := w.Write(rangeJSON); err != nil {
		return err
	}
	if _, err := io.WriteString(w, `,"days":[`); err != nil {
		return err
	}

	currentDay := ""
	firstDay := true
	firstHeartbeat := true

	closeDay := func() error {
		if currentDay == "" {
			return nil
		}
		_, err := io.WriteString(w, "]}")
		return err
	}

	err = srv.heartbeatService.StreamAllWithin(from, to, user, 1000, func(heartbeats []*models.Heartbeat) error {
		for _, hb := range heartbeats {
			entry := wakatime.HeartbeatToCompat(hb)
			day := time.Unix(int64(entry.Time), 0).Format("2006-01-02")
			if day != currentDay {
				if err := closeDay(); err != nil {
					return err
				}
				if !firstDay {
					if _, err := io.WriteString(w, ","); err != nil {
						return err
					}
				}
				dateJSON, err := json.Marshal(day)
				if err != nil {
					return err
				}
				if _, err := io.WriteString(w, `{"date":`); err != nil {
					return err
				}
				if _, err := w.Write(dateJSON); err != nil {
					return err
				}
				if _, err := io.WriteString(w, `,"heartbeats":[`); err != nil {
					return err
				}
				currentDay = day
				firstDay = false
				firstHeartbeat = true
			}

			if !firstHeartbeat {
				if _, err := io.WriteString(w, ","); err != nil {
					return err
				}
			}
			entryJSON, err := json.Marshal(entry)
			if err != nil {
				return err
			}
			if _, err := w.Write(entryJSON); err != nil {
				return err
			}
			firstHeartbeat = false
		}
		return nil
	})
	if err != nil {
		return fmt.Errorf("failed to stream heartbeats: %w", err)
	}

	if err := closeDay(); err != nil {
		return err
	}

	_, err = io.WriteString(w, "]}")
	return err
}

func (srv *DataDumpService) refreshDownloadURL(dump *models.DataDump) {
	if srv.objectStorageService == nil || dump == nil || dump.Expires == nil || dump.HasFailed || dump.IsProcessing {
		return
	}
	if dump.Expires.Before(time.Now()) {
		return
	}

	downloadURL, err := srv.objectStorageService.GetDownloadURL(dataDumpObjectKey(dump.UserID, dump.ID), *dump.Expires)
	if err != nil {
		config.Log().Error("failed to generate data dump download url", "dumpID", dump.ID, "error", err)
		return
	}
	dump.DownloadUrl = downloadURL
}

func (srv *DataDumpService) deleteObject(dump *models.DataDump) error {
	if srv.objectStorageService == nil || dump == nil {
		return nil
	}
	return srv.objectStorageService.Delete(dataDumpObjectKey(dump.UserID, dump.ID))
}

func dataDumpObjectKey(userID, dumpID string) string {
	return fmt.Sprintf("data_dumps/%s/%s.json", userID, dumpID)
}

func jsonReader(v interface{}) *io.PipeReader {
	reader, writer := io.Pipe()
	go func() {
		err := json.NewEncoder(writer).Encode(v)
		if err != nil {
			_ = writer.CloseWithError(err)
			return
		}
		_ = writer.Close()
	}()
	return reader
}
