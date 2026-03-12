package services

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/glebarez/sqlite"
	"github.com/hackclub/hackatime/config"
	"github.com/hackclub/hackatime/models"
	wakatime "github.com/hackclub/hackatime/models/compat/wakatime/v1"
	"github.com/hackclub/hackatime/repositories"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

const (
	garageS3Port = "3900"
	garageRegion = "garage"
	garageBucket = "hackatime-test"
)

func init() {
	_, filename, _, _ := runtime.Caller(0)
	dir := path.Join(path.Dir(filename), "..")
	os.Chdir(dir)
}

func setupTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	dbPath := fmt.Sprintf("/tmp/hackatime_test_%d.db", time.Now().UnixNano())
	t.Cleanup(func() { os.Remove(dbPath) })

	db, err := gorm.Open(sqlite.Open(dbPath), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	require.NoError(t, err)

	err = db.AutoMigrate(
		&models.User{},
		&models.KeyStringValue{},
		&models.Alias{},
		&models.Heartbeat{},
		&models.Summary{},
		&models.SummaryItem{},
		&models.LanguageMapping{},
		&models.ProjectLabel{},
		&models.Diagnostics{},
		&models.LeaderboardItem{},
		&models.DataDump{},
	)
	require.NoError(t, err)

	return db
}

func setupConfig(t *testing.T) {
	t.Helper()
	cfg := config.Empty()
	cfg.App.CustomLanguages = map[string]string{}
	config.Set(cfg)
}

func createTestUser(t *testing.T, db *gorm.DB, id string) *models.User {
	t.Helper()
	user := &models.User{
		ID:        id,
		ApiKey:    id + "-api-key",
		CreatedAt: models.CustomTime(time.Now().Add(-24 * time.Hour)),
	}
	require.NoError(t, db.Create(user).Error)
	return user
}

func createTestHeartbeats(t *testing.T, db *gorm.DB, user *models.User, count int) {
	t.Helper()
	for i := 0; i < count; i++ {
		hb := &models.Heartbeat{
			UserID:   user.ID,
			Entity:   fmt.Sprintf("/path/to/file_%d.go", i),
			Type:     "file",
			Category: "coding",
			Project:  "test-project",
			Language: "Go",
			Time:     models.CustomTime(time.Now().Add(-time.Duration(i) * time.Hour)),
		}
		hb = hb.Hashed()
		require.NoError(t, db.Create(hb).Error)
	}
}

type objectStorageServiceStub struct {
	mu          sync.Mutex
	uploads     map[string][]byte
	deletedKeys []string
}

func (s *objectStorageServiceStub) Upload(key string, data io.Reader, contentType string) error {
	body, err := io.ReadAll(data)
	if err != nil {
		return err
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	if s.uploads == nil {
		s.uploads = make(map[string][]byte)
	}
	s.uploads[key] = body
	return nil
}

func (s *objectStorageServiceStub) GetDownloadURL(key string, expiresAt time.Time) (string, error) {
	return fmt.Sprintf("https://signed.example/%s?expires=%d", key, expiresAt.Unix()), nil
}

func (s *objectStorageServiceStub) Delete(key string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.deletedKeys = append(s.deletedKeys, key)
	delete(s.uploads, key)
	return nil
}

func TestDataDumpRepository_Integration_CRUD(t *testing.T) {
	setupConfig(t)
	db := setupTestDB(t)
	repo := repositories.NewDataDumpRepository(db)
	createTestUser(t, db, "repo-test-user")

	now := time.Now()
	expires := now.Add(24 * time.Hour)

	dump := &models.DataDump{
		ID:           "test-dump-1",
		UserID:       "repo-test-user",
		Status:       models.DataDumpStatusPending,
		Type:         models.DataDumpTypeHeartbeats,
		IsProcessing: true,
		CreatedAt:    now,
		Expires:      &expires,
	}
	inserted, err := repo.Insert(dump)
	require.NoError(t, err)
	assert.Equal(t, "test-dump-1", inserted.ID)

	fetched, err := repo.GetById("test-dump-1")
	require.NoError(t, err)
	assert.Equal(t, "repo-test-user", fetched.UserID)
	assert.Equal(t, models.DataDumpStatusPending, fetched.Status)
	assert.True(t, fetched.IsProcessing)

	dumps, err := repo.GetByUser("repo-test-user")
	require.NoError(t, err)
	assert.Len(t, dumps, 1)

	dump.Status = models.DataDumpStatusCompleted
	dump.PercentComplete = 100
	dump.IsProcessing = false
	dump.DownloadUrl = "https://example.com/dump.json"
	_, err = repo.Update(dump)
	require.NoError(t, err)

	fetched, err = repo.GetById("test-dump-1")
	require.NoError(t, err)
	assert.Equal(t, models.DataDumpStatusCompleted, fetched.Status)
	assert.Equal(t, float32(100), fetched.PercentComplete)
	assert.Equal(t, "https://example.com/dump.json", fetched.DownloadUrl)
	assert.False(t, fetched.IsProcessing)

	err = repo.Delete("test-dump-1")
	require.NoError(t, err)
	dumps, err = repo.GetByUser("repo-test-user")
	require.NoError(t, err)
	assert.Len(t, dumps, 0)
}

func TestDataDumpRepository_Integration_StuckDumps(t *testing.T) {
	setupConfig(t)
	db := setupTestDB(t)
	repo := repositories.NewDataDumpRepository(db)
	createTestUser(t, db, "stuck-test-user")

	now := time.Now()

	oldExpires := now.Add(24 * time.Hour)
	oldDump := &models.DataDump{
		ID:           "old-dump",
		UserID:       "stuck-test-user",
		Status:       models.DataDumpStatusProcessing,
		Type:         models.DataDumpTypeHeartbeats,
		IsProcessing: true,
		CreatedAt:    now.Add(-1 * time.Hour),
		Expires:      &oldExpires,
	}
	_, err := repo.Insert(oldDump)
	require.NoError(t, err)

	recentExpires := now.Add(24 * time.Hour)
	recentDump := &models.DataDump{
		ID:           "recent-dump",
		UserID:       "stuck-test-user",
		Status:       models.DataDumpStatusProcessing,
		Type:         models.DataDumpTypeHeartbeats,
		IsProcessing: true,
		CreatedAt:    now,
		Expires:      &recentExpires,
	}
	_, err = repo.Insert(recentDump)
	require.NoError(t, err)

	threshold := now.Add(-30 * time.Minute)
	stuckDumps, err := repo.GetStuckDumps(threshold)
	require.NoError(t, err)
	assert.Len(t, stuckDumps, 1)
	assert.Equal(t, "old-dump", stuckDumps[0].ID)
}

func TestDataDumpRepository_Integration_ExpiredDumps(t *testing.T) {
	setupConfig(t)
	db := setupTestDB(t)
	repo := repositories.NewDataDumpRepository(db)
	createTestUser(t, db, "expired-test-user")

	now := time.Now()

	pastExpires := now.Add(-1 * time.Hour)
	expiredDump := &models.DataDump{
		ID:        "expired-dump",
		UserID:    "expired-test-user",
		Status:    models.DataDumpStatusCompleted,
		Type:      models.DataDumpTypeHeartbeats,
		CreatedAt: now.Add(-25 * time.Hour),
		Expires:   &pastExpires,
	}
	_, err := repo.Insert(expiredDump)
	require.NoError(t, err)

	futureExpires := now.Add(23 * time.Hour)
	activeDump := &models.DataDump{
		ID:        "active-dump",
		UserID:    "expired-test-user",
		Status:    models.DataDumpStatusCompleted,
		Type:      models.DataDumpTypeHeartbeats,
		CreatedAt: now,
		Expires:   &futureExpires,
	}
	_, err = repo.Insert(activeDump)
	require.NoError(t, err)

	expired, err := repo.GetExpiredDumps()
	require.NoError(t, err)
	assert.Len(t, expired, 1)
	assert.Equal(t, "expired-dump", expired[0].ID)
}

func TestDataDumpService_Integration_MarkStuckDumps(t *testing.T) {
	setupConfig(t)
	db := setupTestDB(t)
	repo := repositories.NewDataDumpRepository(db)
	user := createTestUser(t, db, "stuck-svc-user")

	srv := NewDataDumpService(repo, nil, nil, nil)

	now := time.Now()
	expires := now.Add(24 * time.Hour)
	oldDump := &models.DataDump{
		ID:           "stuck-svc-dump",
		UserID:       user.ID,
		Status:       models.DataDumpStatusProcessing,
		Type:         models.DataDumpTypeHeartbeats,
		IsProcessing: true,
		CreatedAt:    now.Add(-1 * time.Hour),
		Expires:      &expires,
	}
	_, err := repo.Insert(oldDump)
	require.NoError(t, err)

	err = srv.MarkStuckDumps()
	require.NoError(t, err)

	dumps, err := srv.GetByUser(user.ID)
	require.NoError(t, err)
	assert.Len(t, dumps, 1)
	assert.True(t, dumps[0].IsStuck, "dump should be marked as stuck")
}

func TestDataDumpService_Integration_RateLimiting(t *testing.T) {
	setupConfig(t)
	db := setupTestDB(t)
	repo := repositories.NewDataDumpRepository(db)
	user := createTestUser(t, db, "rate-limit-user")

	srv := NewDataDumpService(repo, nil, nil, nil)

	now := time.Now()
	expires := now.Add(24 * time.Hour)
	activeDump := &models.DataDump{
		ID:           "active-processing",
		UserID:       user.ID,
		Status:       models.DataDumpStatusProcessing,
		Type:         models.DataDumpTypeHeartbeats,
		IsProcessing: true,
		CreatedAt:    now,
		Expires:      &expires,
	}
	_, err := repo.Insert(activeDump)
	require.NoError(t, err)

	_, err = srv.Create(user, models.DataDumpTypeHeartbeats)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "already in progress")
	assert.ErrorIs(t, err, ErrDataDumpAlreadyInProgress)
}

func TestDataDumpService_Integration_ValidateDumpType(t *testing.T) {
	setupConfig(t)
	db := setupTestDB(t)
	repo := repositories.NewDataDumpRepository(db)
	user := createTestUser(t, db, "invalid-type-user")

	srv := NewDataDumpService(repo, nil, nil, nil)

	_, err := srv.Create(user, "custom")
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrInvalidDataDumpType)
	assert.True(t, errors.Is(err, ErrInvalidDataDumpType))
	assert.Equal(t, "invalid dump type: custom", err.Error())
}

func TestDataDumpService_Integration_CleanupExpired(t *testing.T) {
	setupConfig(t)
	db := setupTestDB(t)
	repo := repositories.NewDataDumpRepository(db)
	createTestUser(t, db, "cleanup-user")
	objectStorage := &objectStorageServiceStub{}

	srv := NewDataDumpService(repo, nil, nil, objectStorage)

	now := time.Now()
	pastExpires := now.Add(-1 * time.Hour)
	futureExpires := now.Add(23 * time.Hour)

	_, err := repo.Insert(&models.DataDump{
		ID: "expired-1", UserID: "cleanup-user", Status: models.DataDumpStatusCompleted,
		Type: models.DataDumpTypeHeartbeats, CreatedAt: now.Add(-25 * time.Hour), Expires: &pastExpires,
	})
	require.NoError(t, err)

	_, err = repo.Insert(&models.DataDump{
		ID: "active-1", UserID: "cleanup-user", Status: models.DataDumpStatusCompleted,
		Type: models.DataDumpTypeHeartbeats, CreatedAt: now, Expires: &futureExpires,
	})
	require.NoError(t, err)

	err = srv.CleanupExpired()
	require.NoError(t, err)

	dumps, err := repo.GetByUser("cleanup-user")
	require.NoError(t, err)
	assert.Len(t, dumps, 1)
	assert.Equal(t, "active-1", dumps[0].ID)
	assert.Equal(t, []string{dataDumpObjectKey("cleanup-user", "expired-1")}, objectStorage.deletedKeys)
}

func TestDataDumpService_Integration_FullExportWithGarage(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test with Garage in short mode")
	}

	if _, err := exec.LookPath("docker"); err != nil {
		t.Skip("docker not found, skipping Garage integration test")
	}

	setupConfig(t)
	db := setupTestDB(t)

	accessKeyID, secretKey := startGarage(t)

	cfg := config.Get()
	cfg.ObjectStorage.Enabled = true
	cfg.ObjectStorage.Endpoint = fmt.Sprintf("http://127.0.0.1:%s", garageS3Port)
	cfg.ObjectStorage.Region = garageRegion
	cfg.ObjectStorage.Bucket = garageBucket
	cfg.ObjectStorage.AccessKeyId = accessKeyID
	cfg.ObjectStorage.SecretAccessKey = secretKey
	config.Set(cfg)

	objStorage := NewObjectStorageService()
	heartbeatRepo := repositories.NewHeartbeatRepository(db)
	languageMappingRepo := repositories.NewLanguageMappingRepository(db)
	languageMappingSvc := NewLanguageMappingService(languageMappingRepo)
	heartbeatSvc := NewHeartbeatService(heartbeatRepo, languageMappingSvc)
	dataDumpRepo := repositories.NewDataDumpRepository(db)
	srv := NewDataDumpService(dataDumpRepo, heartbeatSvc, nil, objStorage)

	user := createTestUser(t, db, "e2e-user")
	createTestHeartbeats(t, db, user, 5)

	dump, err := srv.Create(user, models.DataDumpTypeHeartbeats)
	require.NoError(t, err)
	assert.Equal(t, models.DataDumpStatusPending, dump.Status)
	assert.True(t, dump.IsProcessing)

	var finalDump *models.DataDump
	require.Eventually(t, func() bool {
		d, err := dataDumpRepo.GetById(dump.ID)
		if err != nil {
			return false
		}
		finalDump = d
		return !d.IsProcessing
	}, 30*time.Second, 500*time.Millisecond, "dump should finish processing within 30s")

	require.NotNil(t, finalDump)
	assert.Equal(t, models.DataDumpStatusCompleted, finalDump.Status)
	assert.Equal(t, float32(100), finalDump.PercentComplete)
	assert.False(t, finalDump.HasFailed, "dump should not have failed")
	assert.False(t, finalDump.IsStuck)
	assert.NotEmpty(t, finalDump.DownloadUrl)

	downloadURL, err := url.Parse(finalDump.DownloadUrl)
	require.NoError(t, err)
	assert.NotEmpty(t, downloadURL.Query().Get("X-Amz-Algorithm"))
	assert.NotEmpty(t, downloadURL.Query().Get("X-Amz-Signature"))

	res, err := (&http.Client{Timeout: 10 * time.Second}).Get(finalDump.DownloadUrl)
	require.NoError(t, err)
	defer res.Body.Close()
	assert.Equal(t, http.StatusOK, res.StatusCode)

	s3Client := s3.New(s3.Options{
		BaseEndpoint: aws.String(fmt.Sprintf("http://127.0.0.1:%s", garageS3Port)),
		Region:       garageRegion,
		Credentials:  credentials.NewStaticCredentialsProvider(accessKeyID, secretKey, ""),
	})

	key := fmt.Sprintf("data_dumps/%s/%s.json", user.ID, dump.ID)
	getOut, err := s3Client.GetObject(context.Background(), &s3.GetObjectInput{
		Bucket: aws.String(garageBucket),
		Key:    aws.String(key),
	})
	require.NoError(t, err)
	defer getOut.Body.Close()

	body, err := io.ReadAll(getOut.Body)
	require.NoError(t, err)
	assert.True(t, len(body) > 0, "downloaded file should not be empty")

	var export wakatime.JsonExportViewModel
	err = json.Unmarshal(body, &export)
	require.NoError(t, err)
	assert.NotNil(t, export.Range)
	assert.NotEmpty(t, export.Days)

	totalHeartbeats := 0
	for _, day := range export.Days {
		totalHeartbeats += len(day.Heartbeats)
	}
	assert.Equal(t, 5, totalHeartbeats, "export should contain all 5 heartbeats")

	dumps, err := srv.GetByUser(user.ID)
	require.NoError(t, err)
	assert.Len(t, dumps, 1)
	assert.Equal(t, dump.ID, dumps[0].ID)
}

func startGarage(t *testing.T) (accessKeyID, secretKey string) {
	t.Helper()

	composeFile := path.Join("testing", "compose.yml")

	cmd := exec.Command("docker", "compose", "-f", composeFile, "up", "-d", "garage")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("failed to start Garage via compose: %s: %s", err, string(out))
	}

	t.Cleanup(func() {
		exec.Command("docker", "compose", "-f", composeFile, "down", "garage").Run()
	})

	setupScript := path.Join("testing", "setup_garage.sh")
	cmd = exec.Command("bash", setupScript)
	out, err = cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("garage setup script failed: %s: %s", err, string(out))
	}

	for _, line := range strings.Split(string(out), "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "KEY_ID=") {
			accessKeyID = strings.TrimPrefix(line, "KEY_ID=")
		}
		if strings.HasPrefix(line, "SECRET_KEY=") {
			secretKey = strings.TrimPrefix(line, "SECRET_KEY=")
		}
	}

	if accessKeyID == "" || secretKey == "" {
		t.Fatalf("failed to extract Garage credentials from setup script output:\n%s", string(out))
	}

	s3Client := s3.New(s3.Options{
		BaseEndpoint: aws.String(fmt.Sprintf("http://127.0.0.1:%s", garageS3Port)),
		Region:       garageRegion,
		Credentials:  credentials.NewStaticCredentialsProvider(accessKeyID, secretKey, ""),
	})

	require.Eventually(t, func() bool {
		_, err := s3Client.ListObjects(context.Background(), &s3.ListObjectsInput{
			Bucket: aws.String(garageBucket),
		})
		return err == nil
	}, 10*time.Second, 1*time.Second, "S3 API should be accessible")

	return accessKeyID, secretKey
}
