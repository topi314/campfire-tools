package rewards

import (
	"net/http"

	"github.com/topi314/campfire-tools/internal/middlewares"
	"github.com/topi314/campfire-tools/server"
)

type handler struct {
	*server.Server
}

func Routes(srv *server.Server) http.Handler {
	h := &handler{
		Server: srv,
	}

	mux := http.NewServeMux()
	mux.HandleFunc("GET /{$}", h.Index)
	mux.HandleFunc("GET /code", h.Code)
	mux.HandleFunc("GET /code/{code}", h.GetCode)
	mux.Handle("GET /code/{code}/qr", middlewares.Cache(http.HandlerFunc(h.QRCode)))

	return mux
}
