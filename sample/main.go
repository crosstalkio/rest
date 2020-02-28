package main

import (
	"net/http"
	"os"

	"github.com/crosstalkio/log"
	"github.com/crosstalkio/rest"
	"github.com/gorilla/mux"
)

func main() {
	log := log.NewSugar(log.NewLogger(log.Color(log.GoLogger(log.Debug, os.Stderr, "", log.LstdFlags))))
	log.Fatal(http.ListenAndServe("localhost:8080", router(log)))
}

func router(log log.Sugar) http.Handler {
	r := mux.NewRouter()
	rest := rest.NewServer(log)
	r.Path("/user/{id:[a-z]+}").Handler(rest.HandlerFunc(UserHandler))
	return r
}
