package tracker

import (
	"net/http"
	"time"

	"github.com/topi314/campfire-tools/server/database"
)

const viewerCookieAge = 365 * 24 * time.Hour

func syncViewerCookies(w http.ResponseWriter, user database.DiscordUser, settings database.UserSettings) {
	if user.ID == "" {
		clearViewerCookie(w, "user_name")
		clearViewerCookie(w, "user_avatar")
		clearViewerCookie(w, "timezone")
		clearViewerCookie(w, "hour_cycle")
		clearViewerCookie(w, "theme")
		return
	}
	name := user.DisplayName
	if name == "" {
		name = user.Username
	}
	setViewerCookie(w, "user_name", name)
	setViewerCookie(w, "user_avatar", user.AvatarURL)
	setViewerCookie(w, "timezone", settings.Timezone)
	setViewerCookie(w, "hour_cycle", settings.HourCycle)
	if settings.Theme == "" {
		clearViewerCookie(w, "theme")
		return
	}
	setViewerCookie(w, "theme", settings.Theme)
}

func setViewerCookie(w http.ResponseWriter, name, value string) {
	if value == "" {
		clearViewerCookie(w, name)
		return
	}
	http.SetCookie(w, &http.Cookie{
		Name:     name,
		Value:    value,
		Path:     "/",
		MaxAge:   int(viewerCookieAge.Seconds()),
		SameSite: http.SameSiteLaxMode,
	})
}

func clearViewerCookie(w http.ResponseWriter, name string) {
	http.SetCookie(w, &http.Cookie{
		Name:     name,
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		SameSite: http.SameSiteLaxMode,
	})
}
