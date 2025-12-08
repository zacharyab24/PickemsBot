package web

import (
	"pickems-bot/api/api"
)

// Config holds the configuration for the web server
type Config struct {
	Addr string
	API  *api.API
}

// Server is the HTTP server that handles webhook requests
type Server struct {
	api *api.API
}
