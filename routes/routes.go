package routes

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/mbolis/quick-survey/app"
	"github.com/mbolis/quick-survey/routes/middleware"
)

func Wire(app app.App) http.Handler {

	root := chi.NewRouter()
	root.Use(middleware.Default)

	root.Mount("/api", apiRouter(app))

	root.
		With(middleware.CookieAuth(app), middleware.Admin(app)).
		Mount("/admin", servePrivateFiles("/admin"))
	root.Mount("/", servePublicFiles())

	return root
}

func apiRouter(app app.App) http.Handler {
	api := chi.NewRouter()

	api.Get(`/surveys/{id:^\d+$}`, PublicGetSurveyById(app))
	api.Post(`/surveys/{id:^\d+$}/submissions`, PublicSubmitSurvey(app))

	api.Route("/admin", func(r chi.Router) {
		r.Use(middleware.Admin(app))

		// CRUD survey
		r.Post("/surveys", CreateSurvey(app))
		r.Get("/surveys", ListSurveys(app))
		r.Get(`/surveys/{id:^\d+$}`, GetSurveyById(app))
		r.Put(`/surveys/{id:^\d+$}`, UpdateSurvey(app))
		r.Delete(`/surveys/{id:^\d+$}`, DeleteSurvey(app))

		r.Get(`/surveys/{id:^\d+$}/submissions`, GetSurveySubmissions(app))
	})

	api.Post("/login", Login(app))
	api.Post("/refresh", Refresh(app))

	return api
}

func servePublicFiles() http.Handler {
	return http.FileServer(http.Dir("public"))
}

func servePrivateFiles(path string) http.Handler {
	return http.StripPrefix(path, http.FileServer(http.Dir("private")))
}
