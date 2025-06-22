package server

import (
	"fmt"
	"io"
	"net/http"
	"path"
)

func (s *Server) Image(w http.ResponseWriter, r *http.Request) {
	imageID := r.PathValue("image_id")

	remoteImageURL := fmt.Sprintf("https://niantic-social-api.nianticlabs.com/images/%s", imageID)
	rq, err := http.NewRequestWithContext(r.Context(), http.MethodGet, remoteImageURL, nil)
	if err != nil {
		http.Error(w, "Failed to create request: "+err.Error(), http.StatusInternalServerError)
		return
	}

	rs, err := s.httpClient.Do(rq)
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
	h := w.Header()
	h.Set("Content-Type", rs.Header.Get("Content-Type"))
	h.Set("Content-Length", rs.Header.Get("Content-Length"))
	h.Set("Cache-Control", "public, max-age=31536000") // Cache for 1 year

	if _, err := io.Copy(w, rs.Body); err != nil {
		http.Error(w, "Failed to write image to response: "+err.Error(), http.StatusInternalServerError)
		return
	}
}

func imageURL(imageURL string) string {
	if imageURL == "" {
		return ""
	}

	return path.Join("/images", path.Base(imageURL))
}
