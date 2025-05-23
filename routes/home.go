package routes

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/gorilla/schema"
	conf "github.com/hackclub/hackatime/config"
	"github.com/hackclub/hackatime/middlewares"
	"github.com/hackclub/hackatime/models/view"
	routeutils "github.com/hackclub/hackatime/routes/utils"
	"github.com/hackclub/hackatime/services"
)

type HomeHandler struct {
	config       *conf.Config
	userSrvc     services.IUserService
	keyValueSrvc services.IKeyValueService
}

var loginDecoder = schema.NewDecoder()
var signupDecoder = schema.NewDecoder()
var resetPasswordDecoder = schema.NewDecoder()

func NewHomeHandler(userService services.IUserService, keyValueService services.IKeyValueService) *HomeHandler {
	return &HomeHandler{
		config:       conf.Get(),
		userSrvc:     userService,
		keyValueSrvc: keyValueService,
	}
}

func (h *HomeHandler) RegisterRoutes(router chi.Router) {
	router.Group(func(r chi.Router) {
		r.Use(middlewares.NewAuthenticateMiddleware(h.userSrvc).WithOptionalFor("/").Handler)
		r.Get("/", h.GetIndex)
	})
}

func (h *HomeHandler) GetIndex(w http.ResponseWriter, r *http.Request) {
	if h.config.IsDev() {
		loadTemplates()
	}

	if user := middlewares.GetPrincipal(r); user != nil {
		http.Redirect(w, r, fmt.Sprintf("%s/summary", h.config.Server.BasePath), http.StatusFound)
		return
	}
	if h.config.Security.DisableFrontpage {
		http.Redirect(w, r, fmt.Sprintf("%s/login", h.config.Server.BasePath), http.StatusFound)
		return
	}

	templates[conf.IndexTemplate].Execute(w, h.buildViewModel(r, w))
}

func (h *HomeHandler) buildViewModel(r *http.Request, w http.ResponseWriter) *view.HomeViewModel {
	var totalHours string
	var totalUsers int
	var newsbox view.Newsbox

	formatThousands := func(n int) string {
		s := strconv.Itoa(n)
		for i := len(s) - 3; i > 0; i -= 3 {
			s = s[:i] + "," + s[i:]
		}
		return s
	}

	if kv, err := h.keyValueSrvc.GetString(conf.KeyLatestTotalTime); err == nil && kv != nil && kv.Value != "" {
		if d, err := time.ParseDuration(kv.Value); err == nil {
			totalHours = formatThousands(int(d.Hours()))
		}
	}

	if kv, err := h.keyValueSrvc.GetString(conf.KeyLatestTotalUsers); err == nil && kv != nil && kv.Value != "" {
		if d, err := strconv.Atoi(kv.Value); err == nil {
			totalUsers = d
		}
	}

	if kv, err := h.keyValueSrvc.GetString(conf.KeyNewsbox); err == nil && kv != nil && kv.Value != "" {
		if err := json.NewDecoder(strings.NewReader(kv.Value)).Decode(&newsbox); err != nil {
			conf.Log().Request(r).Error("failed to decode newsbox message", "error", err)
		}
	}

	vm := &view.HomeViewModel{
		SharedViewModel: view.NewSharedViewModel(h.config, nil),
		TotalHours:      totalHours,
		TotalUsers:      totalUsers,
		Newsbox:         &newsbox,
	}
	return routeutils.WithSessionMessages(vm, r, w)
}
