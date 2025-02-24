package api

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/hackclub/hackatime/helpers"

	conf "github.com/hackclub/hackatime/config"
	"github.com/hackclub/hackatime/middlewares"
	"github.com/hackclub/hackatime/services"
)

type SpecialApiHandler struct {
	config   *conf.Config
	userSrvc services.IUserService
}

func NewSpecialApiHandler(userService services.IUserService) *SpecialApiHandler {
	return &SpecialApiHandler{
		userSrvc: userService,
		config:   conf.Get(),
	}
}

func (h *SpecialApiHandler) RegisterRoutes(router chi.Router) {
	r := chi.NewRouter()
	r.Use(middlewares.NewAuthenticateMiddleware(h.userSrvc).Handler)
	r.Get("/email", h.GetEmail)
	r.Get("/apikey", h.GetApiKey)
	r.Get("/hasData", h.HasData)

	router.Mount("/special", r)
}

// @Summary Retrieve a users email
// @ID get-email
// @Tags email
// @Produce json
// @Param user query string false "The user to filter by if using Bearer authentication and the admin token"
// @Security ApiKeyAuth
// @Success 200 {object} models.Email
// @Router /special/email [get]
func (h *SpecialApiHandler) GetEmail(w http.ResponseWriter, r *http.Request) {
	user, err := h.userSrvc.GetUserById(r.URL.Query().Get("user"))
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(err.Error()))
		return
	}

	helpers.RespondJSON(w, r, http.StatusOK, map[string]any{
		"email": user.Email,
	})
}

// @Summary Retrieve a users API key
// @ID get-apikey
// @Tags apikey
// @Produce json
// @Param user query string false "The user to filter by if using Bearer authentication and the admin token"
// @Security ApiKeyAuth
// @Success 200 {object} models.ApiKey
// @Router /special/apikey [get]
func (h *SpecialApiHandler) GetApiKey(w http.ResponseWriter, r *http.Request) {
	user, err := h.userSrvc.GetUserById(r.URL.Query().Get("user"))
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(err.Error()))
		return
	}

	helpers.RespondJSON(w, r, http.StatusOK, map[string]any{
		"apiKey": user.ApiKey,
	})
}

// @Summary Whether the user has data any heartbeats received yet
// @ID has-data
// @Tags hasData
// @Produce json
// @Param user query string false "The user to filter by if using Bearer authentication and the admin token"
// @Security ApiKeyAuth
// @Success 200 {object} models.HasData
// @Router /special/hasData [get]
func (h *SpecialApiHandler) HasData(w http.ResponseWriter, r *http.Request) {
	user, err := h.userSrvc.GetUserById(r.URL.Query().Get("user"))
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(err.Error()))
		return
	}

	helpers.RespondJSON(w, r, http.StatusOK, map[string]any{
		"hasData": user.HasData,
	})
}
