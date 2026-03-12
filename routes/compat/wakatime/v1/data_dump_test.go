package v1

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/hackclub/hackatime/config"
	"github.com/hackclub/hackatime/middlewares"
	"github.com/hackclub/hackatime/mocks"
	"github.com/hackclub/hackatime/models"
	compat "github.com/hackclub/hackatime/models/compat/wakatime/v1"
	"github.com/hackclub/hackatime/services"
)

type dataDumpServiceStub struct {
	getByUser func(userID string) error
	create    func(dumpType string) error
}

func (s *dataDumpServiceStub) GetByUser(userID string) ([]*models.DataDump, error) {
	if s.getByUser != nil {
		return nil, s.getByUser(userID)
	}
	return nil, nil
}

func (s *dataDumpServiceStub) Create(user *models.User, dumpType string) (*models.DataDump, error) {
	if s.create != nil {
		return nil, s.create(dumpType)
	}
	return nil, nil
}

func (s *dataDumpServiceStub) CleanupExpired() error {
	return nil
}

func (s *dataDumpServiceStub) MarkStuckDumps() error {
	return nil
}

func (s *dataDumpServiceStub) DeleteByUser(userID string) error {
	return nil
}

func TestDataDumpHandler_Errors(t *testing.T) {
	config.Set(config.Empty())

	router := chi.NewRouter()
	apiRouter := chi.NewRouter()
	apiRouter.Use(middlewares.NewPrincipalMiddleware())
	router.Mount("/api", apiRouter)

	userServiceMock := new(mocks.UserServiceMock)
	userServiceMock.On("GetUserById", "AdminUser").Return(adminUser, nil)
	userServiceMock.On("GetUserByKey", "admin-user-api-key").Return(adminUser, nil)

	handler := NewDataDumpHandler(userServiceMock, &dataDumpServiceStub{
		getByUser: func(userID string) error {
			return errors.New("storage unavailable")
		},
		create: func(dumpType string) error {
			switch dumpType {
			case "custom":
				return fmt.Errorf("%w: %s", services.ErrInvalidDataDumpType, dumpType)
			default:
				return services.ErrDataDumpAlreadyInProgress
			}
		},
	})
	handler.RegisterRoutes(apiRouter)

	t.Run("get returns json error model", func(t *testing.T) {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/api/compat/wakatime/v1/users/{user}/data_dumps", nil)
		req = withUrlParam(req, "user", "AdminUser")
		req.Header.Add("Authorization", fmt.Sprintf("Bearer %s", base64.StdEncoding.EncodeToString([]byte(adminUser.ApiKey))))

		router.ServeHTTP(rec, req)

		res := rec.Result()
		defer res.Body.Close()

		if res.StatusCode != http.StatusInternalServerError {
			t.Fatalf("expected status %d, got %d", http.StatusInternalServerError, res.StatusCode)
		}
		if got := res.Header.Get("Content-Type"); !strings.Contains(got, "application/json") {
			t.Fatalf("expected json content type, got %q", got)
		}

		var body compat.DataDumpResultErrorModel
		if err := json.NewDecoder(res.Body).Decode(&body); err != nil {
			t.Fatalf("failed to decode response: %v", err)
		}
		if body.Error != "storage unavailable" {
			t.Fatalf("expected error message %q, got %q", "storage unavailable", body.Error)
		}
	})

	t.Run("post returns 400 json for invalid dump type", func(t *testing.T) {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/api/compat/wakatime/v1/users/{user}/data_dumps", strings.NewReader(`{"type":"custom"}`))
		req = withUrlParam(req, "user", "AdminUser")
		req.Header.Add("Authorization", fmt.Sprintf("Bearer %s", base64.StdEncoding.EncodeToString([]byte(adminUser.ApiKey))))
		req.Header.Set("Content-Type", "application/json")

		router.ServeHTTP(rec, req)

		res := rec.Result()
		defer res.Body.Close()

		if res.StatusCode != http.StatusBadRequest {
			t.Fatalf("expected status %d, got %d", http.StatusBadRequest, res.StatusCode)
		}

		var body compat.DataDumpResultErrorModel
		if err := json.NewDecoder(res.Body).Decode(&body); err != nil {
			t.Fatalf("failed to decode response: %v", err)
		}
		if body.Error != "invalid dump type: custom" {
			t.Fatalf("expected error message %q, got %q", "invalid dump type: custom", body.Error)
		}
	})

	t.Run("post returns 400 json for duplicate export", func(t *testing.T) {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/api/compat/wakatime/v1/users/{user}/data_dumps", strings.NewReader(`{"type":"heartbeats"}`))
		req = withUrlParam(req, "user", "AdminUser")
		req.Header.Add("Authorization", fmt.Sprintf("Bearer %s", base64.StdEncoding.EncodeToString([]byte(adminUser.ApiKey))))
		req.Header.Set("Content-Type", "application/json")

		router.ServeHTTP(rec, req)

		res := rec.Result()
		defer res.Body.Close()

		if res.StatusCode != http.StatusBadRequest {
			t.Fatalf("expected status %d, got %d", http.StatusBadRequest, res.StatusCode)
		}

		var body compat.DataDumpResultErrorModel
		if err := json.NewDecoder(res.Body).Decode(&body); err != nil {
			t.Fatalf("failed to decode response: %v", err)
		}
		if body.Error != services.ErrDataDumpAlreadyInProgress.Error() {
			t.Fatalf("expected error message %q, got %q", services.ErrDataDumpAlreadyInProgress.Error(), body.Error)
		}
	})
}
