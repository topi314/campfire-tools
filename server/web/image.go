package web

import (
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"path"
)

func (h *handler) Image(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	imageID := r.PathValue("image_id")

	remoteImageURL := fmt.Sprintf("https://niantic-social-api.nianticlabs.com/images/%s", imageID)
	rq, err := http.NewRequestWithContext(ctx, http.MethodGet, remoteImageURL, nil)
	if err != nil {
		http.Error(w, "Failed to create request: "+err.Error(), http.StatusInternalServerError)
		return
	}

	rs, err := h.HttpClient.Do(rq)
	if err != nil {
		http.Error(w, "Failed to fetch image: "+err.Error(), http.StatusInternalServerError)
		return
	}
	defer rs.Body.Close()

	// Check if the response status is OK
	if rs.StatusCode != http.StatusOK {
		http.Error(w, "Failed to fetch image: "+rs.Status, rs.StatusCode)
		return
	}

	// Set the appropriate content type based on the file extension
	header := w.Header()
	header.Set("Content-Type", rs.Header.Get("Content-Type"))
	header.Set("Content-Length", rs.Header.Get("Content-Length"))
	header.Set("Cache-Control", "public, max-age=31536000") // Cache for 1 year

	if _, err = io.Copy(w, rs.Body); err != nil {
		slog.ErrorContext(ctx, "Failed to write image to response", slog.Any("err", err))
		return
	}
}

func imageURL(imageURL string) string {
	if imageURL == "" {
		return ""
	}

	return path.Join("/images", path.Base(imageURL))
}
