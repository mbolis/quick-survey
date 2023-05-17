package main

import (
	"errors"
	"net/http"
	"time"

	"github.com/mbolis/quick-survey/database"
	"github.com/mbolis/quick-survey/routes"
)

func main() {
	db, err := database.Open()
	if err != nil {
		panic(err) // TODO handle better
	}
	defer db.Close()

	handler := routes.Wire(db)

	err = runServer(handler)
	if !errors.Is(err, http.ErrServerClosed) {
		panic(err)
	}
}

func runServer(handler http.Handler) error {
	srv := &http.Server{
		Addr:         "0.0.0.0:8080", // TODO make this parametric
		Handler:      handler,
		IdleTimeout:  time.Minute,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 30 * time.Second,
	}

	return srv.ListenAndServe()
}
