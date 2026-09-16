package tracker

import (
	"encoding/json"
	"html"
	"log/slog"
	"net/http"
)

func (h *handler) TrackerClubEventShareLink(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	eventID := r.PathValue("event_id")

	url, err := h.Campfire.CreateEventLink(ctx, eventID)
	if err != nil {
		slog.ErrorContext(ctx, "Failed to create event share link", slog.String("event_id", eventID), slog.Any("err", err))
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.WriteHeader(http.StatusBadGateway)
		_, _ = w.Write([]byte(`<p class="red">Failed to generate share link: ` + html.EscapeString(err.Error()) + `</p>`))
		return
	}

	escaped := html.EscapeString(url)
	urlJSON, err := json.Marshal(url)
	if err != nil {
		urlJSON = []byte(`""`)
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = w.Write([]byte(
		`<h3>Share link copied</h3>` +
			`<p class="share-link-url"><a href="` + escaped + `" target="_blank" rel="noopener noreferrer">` + escaped + `</a></p>` +
			`<script>navigator.clipboard.writeText(` + string(urlJSON) + `);</script>`,
	))
}
