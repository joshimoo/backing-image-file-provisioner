package server

import (
	"github.com/gorilla/mux"
)

// NewRouter creates and configures a mux router
func NewRouter(fp *FileProvisioner) *mux.Router {

	// API framework routes
	router := mux.NewRouter().StrictSlash(true)

	// Application
	router.HandleFunc("/v1/file", fp.Get).Methods("Get")
	router.HandleFunc("/v1/file", fp.Upload).Methods("POST").Queries("action", "upload")
	router.HandleFunc("/v1/file", fp.Close).Methods("POST").Queries("action", "close")

	return router
}
