package main

import (
	_ "embed"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
)

const graphQLEndpoint = "https://niantic-social-api.nianticlabs.com/graphql"

//go:embed query.graphql
var graphQLQuery string

func main() {
	server := New()

	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=UTF-8")
		_, _ = w.Write([]byte(`
<!DOCTYPE html>
<html lang="en">
<head>
	<meta charset="UTF-8">
	<meta name="viewport" content="width=device-width, initial-scale=1.0">
	<title>Raffle</title>
</head>
<body>
	<h1>Raffle</h1>
	<form action="/raffle" method="POST">
		<label for="url">Campfire Event URL:</label>
		<br>
		<input type="text" id="url" name="url" placeholder="https://campfire.nianticlabs.com/discover/meetup/7d5719a2-e1a2-4d04-9638-e60eb35728bf" required>
		<br>
		<br>
		<label for="count">Number of Winners:</label>
		<br>
		<input type="number" id="count" name="count" min="1" value="1" required>
		<br>
		<br>
		<button type="submit">Raffle</button>
	</form>
	<p>
		<a href="https://campfire.nianticlabs.com/discover/meetup/7d5719a2-e1a2-4d04-9638-e60eb35728bf">Example Event</a>
	</p>
</body>
</html>
`))
	})
	mux.HandleFunc("POST /raffle", server.raffle)

	server.Server.Handler = mux

	go server.Start()

	log.Println("Server started at http://localhost:8080")

	s := make(chan os.Signal, 1)
	signal.Notify(s, syscall.SIGTERM, syscall.SIGINT)
	<-s
}
