package rewards

import (
	"log/slog"
	"net/http"
	"net/url"

	"github.com/topi314/campfire-tools/internal/xquery"
)

type IndexVars struct {
	ClubID    string
	SignUpURL string
}

func (h *handler) Index(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	query := r.URL.Query()

	clubID := xquery.ParseString(query, "club", "")

	signUpQuery := url.Values{}
	if clubID != "" {
		signUpQuery.Set("club", clubID)
	}

	signUpURL := url.URL{
		Path:     "/signup",
		RawQuery: signUpQuery.Encode(),
	}

	if err := h.Templates().ExecuteTemplate(w, "rewards_index.gohtml", IndexVars{
		ClubID:    clubID,
		SignUpURL: signUpURL.String(),
	}); err != nil {
		slog.ErrorContext(ctx, "Failed to render rewards template", slog.String("err", err.Error()))
	}
}
