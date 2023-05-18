package app

import (
	"database/sql"

	"github.com/go-chi/oauth"
	"github.com/mbolis/quick-survey/config"
)

type App struct {
	*sql.DB
	*oauth.BearerServer
	config.Config
}
