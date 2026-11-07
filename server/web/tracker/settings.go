package tracker

import (
	"log/slog"
	"net/http"
	"time"

	"github.com/topi314/campfire-tools/server/auth"
	"github.com/topi314/campfire-tools/server/database"
)

type timeZoneOption struct {
	ID    string
	Label string
}

type timeZoneGroup struct {
	Name  string
	Zones []timeZoneOption
}

var timeZoneGroups = []timeZoneGroup{
	{Name: "UTC", Zones: []timeZoneOption{{ID: "UTC", Label: "UTC"}}},
	{Name: "Africa", Zones: []timeZoneOption{
		{ID: "Africa/Cairo", Label: "Cairo"},
		{ID: "Africa/Johannesburg", Label: "Johannesburg"},
	}},
	{Name: "America", Zones: []timeZoneOption{
		{ID: "America/Los_Angeles", Label: "Los Angeles"},
		{ID: "America/Denver", Label: "Denver"},
		{ID: "America/Chicago", Label: "Chicago"},
		{ID: "America/New_York", Label: "New York"},
		{ID: "America/Toronto", Label: "Toronto"},
		{ID: "America/Mexico_City", Label: "Mexico City"},
		{ID: "America/Sao_Paulo", Label: "São Paulo"},
	}},
	{Name: "Asia", Zones: []timeZoneOption{
		{ID: "Asia/Dubai", Label: "Dubai"},
		{ID: "Asia/Kolkata", Label: "Kolkata"},
		{ID: "Asia/Bangkok", Label: "Bangkok"},
		{ID: "Asia/Singapore", Label: "Singapore"},
		{ID: "Asia/Shanghai", Label: "Shanghai"},
		{ID: "Asia/Tokyo", Label: "Tokyo"},
		{ID: "Asia/Seoul", Label: "Seoul"},
	}},
	{Name: "Australia", Zones: []timeZoneOption{
		{ID: "Australia/Perth", Label: "Perth"},
		{ID: "Australia/Sydney", Label: "Sydney"},
	}},
	{Name: "Europe", Zones: []timeZoneOption{
		{ID: "Europe/London", Label: "London"},
		{ID: "Europe/Dublin", Label: "Dublin"},
		{ID: "Europe/Lisbon", Label: "Lisbon"},
		{ID: "Europe/Madrid", Label: "Madrid"},
		{ID: "Europe/Paris", Label: "Paris"},
		{ID: "Europe/Amsterdam", Label: "Amsterdam"},
		{ID: "Europe/Berlin", Label: "Berlin"},
		{ID: "Europe/Rome", Label: "Rome"},
		{ID: "Europe/Warsaw", Label: "Warsaw"},
		{ID: "Europe/Athens", Label: "Athens"},
		{ID: "Europe/Helsinki", Label: "Helsinki"},
		{ID: "Europe/Istanbul", Label: "Istanbul"},
	}},
	{Name: "Pacific", Zones: []timeZoneOption{
		{ID: "Pacific/Honolulu", Label: "Honolulu"},
		{ID: "Pacific/Auckland", Label: "Auckland"},
	}},
}

type SettingsVars struct {
	DisplayName string
	Username    string
	AvatarURL   string
	Timezone    string
	HourCycle   string
	Theme       string
	Zones       []timeZoneGroup
	Saved       bool
	Error       string
}

func (h *handler) Settings(w http.ResponseWriter, r *http.Request) {
	session := auth.GetSession(r)
	if session.UserID == "" {
		h.forceLogin(w, r)
		return
	}
	settings, err := h.DB.GetUserSettings(r.Context(), session.UserID)
	if err != nil {
		slog.ErrorContext(r.Context(), "failed to load user settings", slog.Any("error", err))
		http.Error(w, "Failed to load settings", http.StatusInternalServerError)
		return
	}
	h.renderSettings(w, r, session.DiscordUser, settings, r.URL.Query().Get("saved") == "1", "")
}

func (h *handler) SaveSettings(w http.ResponseWriter, r *http.Request) {
	session := auth.GetSession(r)
	if session.UserID == "" {
		h.forceLogin(w, r)
		return
	}
	if err := r.ParseForm(); err != nil {
		http.Error(w, "Bad request", http.StatusBadRequest)
		return
	}
	timezone := r.FormValue("timezone")
	hourCycle := r.FormValue("hour_cycle")
	theme := r.FormValue("theme")
	if timezone != "" {
		if _, err := time.LoadLocation(timezone); err != nil || !knownTimeZone(timezone) {
			settings, loadErr := h.DB.GetUserSettings(r.Context(), session.UserID)
			if loadErr != nil {
				settings = database.UserSettings{UserID: session.UserID}
			}
			h.renderSettings(w, r, session.DiscordUser, settings, false, "Choose a time zone from the list.")
			return
		}
	}
	if hourCycle != "" && hourCycle != "h12" && hourCycle != "h23" {
		settings, loadErr := h.DB.GetUserSettings(r.Context(), session.UserID)
		if loadErr != nil {
			settings = database.UserSettings{UserID: session.UserID}
		}
		h.renderSettings(w, r, session.DiscordUser, settings, false, "Choose a clock style from the list.")
		return
	}
	if theme != "system" && theme != "light" && theme != "dark" {
		settings, loadErr := h.DB.GetUserSettings(r.Context(), session.UserID)
		if loadErr != nil {
			settings = database.UserSettings{UserID: session.UserID}
		}
		h.renderSettings(w, r, session.DiscordUser, settings, false, "Choose a theme from the list.")
		return
	}
	settings := database.UserSettings{
		UserID:    session.UserID,
		Timezone:  timezone,
		HourCycle: hourCycle,
		Theme:     theme,
	}
	if err := h.DB.SaveUserSettings(r.Context(), settings); err != nil {
		slog.ErrorContext(r.Context(), "failed to save user settings", slog.Any("error", err))
		h.renderSettings(w, r, session.DiscordUser, settings, false, "Could not save settings.")
		return
	}
	syncViewerCookies(w, session.DiscordUser, settings)
	http.Redirect(w, r, "/settings?saved=1", http.StatusSeeOther)
}

func (h *handler) renderSettings(w http.ResponseWriter, r *http.Request, user database.DiscordUser, settings database.UserSettings, saved bool, errorMessage string) {
	avatar := user.AvatarURL
	if avatar == "" {
		avatar = "/static/default.png"
	}
	name := user.DisplayName
	if name == "" {
		name = user.Username
	}
	if err := h.Templates().ExecuteTemplate(w, "settings.gohtml", SettingsVars{
		DisplayName: name,
		Username:    user.Username,
		AvatarURL:   avatar,
		Timezone:    settings.Timezone,
		HourCycle:   settings.HourCycle,
		Theme:       settings.Theme,
		Zones:       timeZoneGroups,
		Saved:       saved,
		Error:       errorMessage,
	}); err != nil {
		slog.ErrorContext(r.Context(), "failed to render settings", slog.Any("error", err))
	}
}

func knownTimeZone(id string) bool {
	for _, group := range timeZoneGroups {
		for _, zone := range group.Zones {
			if zone.ID == id {
				return true
			}
		}
	}
	return false
}
