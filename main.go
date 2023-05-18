package main

import (
	"errors"
	"net/http"
	"time"

	"github.com/mbolis/quick-survey/app"
	"github.com/mbolis/quick-survey/config"
	"github.com/mbolis/quick-survey/database"
	"github.com/mbolis/quick-survey/httpx"
	"github.com/mbolis/quick-survey/log"
	"github.com/mbolis/quick-survey/routes"
)

func main() {
	cfg, err := config.ParseFlags()
	if err != nil {
		log.Fatal("main.config:", err)
	}
	if cfg.Debug {
		log.SetLevel(log.DebugLevel)
	}

	db, err := database.Open(cfg)
	if err != nil {
		log.Fatal("main.db.open:", err)
	}
	defer db.Close()

	bearerServer := httpx.NewBearerServer(db, cfg)

	app := app.App{
		DB:           db,
		BearerServer: bearerServer,
		Config:       cfg,
	}

	handler := routes.Wire(app)

	err = runServer(cfg, handler)
	if !errors.Is(err, http.ErrServerClosed) {
		log.Fatal("main.server:", err)
	}
}

func runServer(cfg config.Config, handler http.Handler) error {
	srv := &http.Server{
		Addr:         cfg.Addr,
		Handler:      handler,
		IdleTimeout:  time.Minute,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 30 * time.Second,
	}

	log.Info("Listening on " + cfg.Url())
	return srv.ListenAndServe()
}
