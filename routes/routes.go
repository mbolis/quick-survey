package routes

import (
	"database/sql"
	"net/http"

	"github.com/go-chi/chi/middleware"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/oauth"
	"github.com/mbolis/quick-survey/routes/middlewares"
)

func Wire(db *sql.DB) http.Handler {
	bearerServer := NewBearerServer(db)

	root := chi.NewRouter()
	root.Use(middleware.Logger, middleware.Recoverer)

	root.Mount("/api", apiRouter(db, bearerServer))

	root.
		With(middlewares.CookieAuth(bearerServer), middlewares.Admin).
		Mount("/admin", servePrivateFiles("/admin"))
	root.Mount("/", servePublicFiles())

	return root
}

func apiRouter(db *sql.DB, bearerServer *oauth.BearerServer) http.Handler {
	api := chi.NewRouter()

	api.Get(`/surveys/{id:^\d+$}`, PublicGetSurveyById(db))
	api.Post(`/surveys/{id:^\d+$}/submissions`, PublicSubmitSurvey(db))

	api.Route("/admin", func(r chi.Router) {
		r.Use(middlewares.Admin)

		// CRUD survey
		r.Post("/surveys", CreateSurvey(db))
		r.Get("/surveys", ListSurveys(db))
		r.Get(`/surveys/{id:^\d+$}`, GetSurveyById(db))
		r.Put(`/surveys/{id:^\d+$}`, UpdateSurvey(db))
		r.Delete(`/surveys/{id:^\d+$}`, DeleteSurvey(db))

		r.Get(`/surveys/{id:^\d+$}/submissions`, GetSurveySubmissions(db))
	})

	api.Post("/login", Login(bearerServer))
	api.Post("/refresh", Refresh(bearerServer))

	return api
}

func servePublicFiles() http.Handler {
	return http.FileServer(http.Dir("public"))
}

func servePrivateFiles(path string) http.Handler {
	return http.StripPrefix(path, http.FileServer(http.Dir("private")))
}
