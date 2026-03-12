package v1

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"
	conf "github.com/hackclub/hackatime/config"
	"github.com/hackclub/hackatime/helpers"
	"github.com/hackclub/hackatime/middlewares"
	v1 "github.com/hackclub/hackatime/models/compat/wakatime/v1"
	routeutils "github.com/hackclub/hackatime/routes/utils"
	"github.com/hackclub/hackatime/services"
)

type DataDumpHandler struct {
	config       *conf.Config
	userSrvc     services.IUserService
	dataDumpSrvc services.IDataDumpService
}

func NewDataDumpHandler(userService services.IUserService, dataDumpService services.IDataDumpService) *DataDumpHandler {
	return &DataDumpHandler{
		userSrvc:     userService,
		dataDumpSrvc: dataDumpService,
		config:       conf.Get(),
	}
}

func (h *DataDumpHandler) RegisterRoutes(router chi.Router) {
	router.Group(func(r chi.Router) {
		r.Use(middlewares.NewAuthenticateMiddleware(h.userSrvc).Handler)
		r.Get("/compat/wakatime/v1/users/{user}/data_dumps", h.Get)
		r.Get("/v1/users/{user}/data_dumps", h.Get)
		r.Post("/compat/wakatime/v1/users/{user}/data_dumps", h.Post)
		r.Post("/v1/users/{user}/data_dumps", h.Post)
	})
}

// @Summary Retrieve data exports for the given user
// @Description Mimics https://wakatime.com/developers#data_dumps
// @ID get-data-dumps
// @Tags wakatime
// @Produce json
// @Param user path string true "User ID to fetch data for (or 'current')"
// @Security ApiKeyAuth
// @Success 200 {object} v1.DataDumpViewModel
// @Router /compat/wakatime/v1/users/{user}/data_dumps [get]
func (h *DataDumpHandler) Get(w http.ResponseWriter, r *http.Request) {
	user, err := routeutils.CheckEffectiveUser(w, r, h.userSrvc, "current")
	if err != nil {
		return
	}

	dumps, err := h.dataDumpSrvc.GetByUser(user.ID)
	if err != nil {
		conf.Log().Request(r).Error("error loading data dumps", "error", err)
		helpers.RespondJSON(w, r, http.StatusInternalServerError, v1.DataDumpResultErrorModel{
			Error: err.Error(),
		})
		return
	}

	dumpData := make([]*v1.DataDumpData, len(dumps))
	for i, d := range dumps {
		var expires string
		if d.Expires != nil {
			expires = d.Expires.Format("2006-01-02T15:04:05Z")
		}
		dumpData[i] = &v1.DataDumpData{
			Id:              d.ID,
			Status:          d.Status,
			PercentComplete: d.PercentComplete,
			DownloadUrl:     d.DownloadUrl,
			Type:            d.Type,
			IsProcessing:    d.IsProcessing,
			IsStuck:         d.IsStuck,
			HasFailed:       d.HasFailed,
			Expires:         expires,
			CreatedAt:       d.CreatedAt.Format("2006-01-02T15:04:05Z"),
		}
	}

	helpers.RespondJSON(w, r, http.StatusOK, v1.DataDumpViewModel{
		Data:       dumpData,
		Total:      len(dumpData),
		TotalPages: 1,
	})
}

func (h *DataDumpHandler) Post(w http.ResponseWriter, r *http.Request) {
	user, err := routeutils.CheckEffectiveUser(w, r, h.userSrvc, "current")
	if err != nil {
		return
	}

	var reqBody struct {
		Type string `json:"type"`
	}
	if err := json.NewDecoder(r.Body).Decode(&reqBody); err != nil || reqBody.Type == "" {
		reqBody.Type = "heartbeats"
	}

	dump, err := h.dataDumpSrvc.Create(user, reqBody.Type)
	if err != nil {
		conf.Log().Request(r).Error("error creating data dump", "error", err)
		status := http.StatusInternalServerError
		if errors.Is(err, services.ErrDataDumpAlreadyInProgress) || errors.Is(err, services.ErrInvalidDataDumpType) {
			status = http.StatusBadRequest
		}
		helpers.RespondJSON(w, r, status, v1.DataDumpResultErrorModel{
			Error: err.Error(),
		})
		return
	}

	var expires string
	if dump.Expires != nil {
		expires = dump.Expires.Format("2006-01-02T15:04:05Z")
	}

	helpers.RespondJSON(w, r, http.StatusCreated, v1.DataDumpResultViewModel{
		Data: &v1.DataDumpData{
			Id:              dump.ID,
			Status:          dump.Status,
			PercentComplete: dump.PercentComplete,
			DownloadUrl:     dump.DownloadUrl,
			Type:            dump.Type,
			IsProcessing:    dump.IsProcessing,
			IsStuck:         dump.IsStuck,
			HasFailed:       dump.HasFailed,
			Expires:         expires,
			CreatedAt:       dump.CreatedAt.Format("2006-01-02T15:04:05Z"),
		},
	})
}
