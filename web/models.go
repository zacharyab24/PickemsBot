package web

import (
	"pickems-bot/app"
)

// Config holds the configuration for the web server
type Config struct {
	Addr string
	API  *app.App
}

// Server is the HTTP server that handles webhook requests
type Server struct {
	api *app.App
}
