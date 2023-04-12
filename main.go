package main

import (
	"bytes"
	"database/sql"
	"embed"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/middleware"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/oauth"
	"github.com/go-chi/render"
	"github.com/golang-migrate/migrate/v4"
	"github.com/golang-migrate/migrate/v4/database/sqlite3"
	"github.com/golang-migrate/migrate/v4/source/iofs"
	_ "github.com/mattn/go-sqlite3"
	"golang.org/x/crypto/bcrypt"
)

//go:embed migrations
var dbMigrations embed.FS

type Survey struct {
	ID          int           `json:"id,omitempty"`
	Version     int           `json:"version,omitempty"`
	Title       string        `json:"title"`
	Description string        `json:"description"`
	Fields      []SurveyField `json:"fields"`
}

type SurveyField struct {
	ID       int    `json:"id,omitempty"`
	Type     string `json:"type"`
	Name     string `json:"name"`
	Label    string `json:"label"`
	Required bool   `json:"required"`
	Options  any    `json:"options"`
}

type Submission struct {
	ID     int                        `json:"id"`
	Time   time.Time                  `json:"time"`
	IP     string                     `json:"ip"`
	Fields map[string]SubmissionField `json:"fields"`
}

type SubmissionField struct {
	ID    int    `json:"id"`
	Name  string `json:"name"`
	Label string `json:"label"`
	Value any    `json:"value"`
}

var reNoIdent = regexp.MustCompile(`\W+`)

type IpCheck struct {
	op     bool
	ip     string
	result chan<- bool
}

func main() {
	// create database handle
	db, err := sql.Open("sqlite3", "qsurvey.sqlite")
	if err != nil {
		panic(err)
	}
	// will close db at end of application
	defer db.Close()
	// config db to honor foreign key contraints
	if _, err := db.Exec("PRAGMA foreign_keys = ON"); err != nil {
		panic(err)
	}
	// db tuning options
	db.SetMaxOpenConns(20)
	db.SetMaxIdleConns(10)
	db.SetConnMaxIdleTime(5 * time.Minute)
	db.SetConnMaxLifetime(2 * time.Hour)

	// migrate db source
	src, err := iofs.New(dbMigrations, "migrations")
	if err != nil {
		panic(err)
	}
	// migrate db destination
	dst, err := sqlite3.WithInstance(db, &sqlite3.Config{})
	if err != nil {
		panic(err)
	}
	// migration instance
	m, err := migrate.NewWithInstance("iofs", src, "sqlite3", dst)
	if err != nil {
		panic(err)
	}
	// do migration
	err = m.Up()
	switch {
	case errors.Is(err, migrate.ErrNoChange):
		// db already up to date
		break
	case err != nil:
		// error occurred
		panic(err)
	}

	// API endpoint routing
	api := chi.NewRouter()

	// check bearer token if present
	// XXX after a bit of fiddling, I removed a piece of code here... the comment stuck!
	adminMiddleware := func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			claims := r.Context().Value(oauth.ClaimsContext).(map[string]string)

			if rolesClaim, ok := claims["roles"]; ok {
				roles := strings.Split(rolesClaim, ",")
				for _, role := range roles {
					if role == "admin" {
						// Token is authenticated, pass it through
						next.ServeHTTP(w, r)
						return
					}
				}
			}

			http.Error(w, http.StatusText(http.StatusForbidden), http.StatusForbidden)
		})
	}

	// /admin endpoints
	api.Route("/admin", func(r chi.Router) {
		// private endpoint
		// TODO move secret to config file
		r.Use(oauth.Authorize("secret", nil), adminMiddleware)

		// CRUD survey
		r.Post("/surveys", func(w http.ResponseWriter, r *http.Request) {
			survey := Survey{}
			err := render.DecodeJSON(r.Body, &survey)
			if err != nil {
				http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
				log.Println(err) // XXX added this for debug... now I should go around adding it everywhere... groan!
				return
			}

			// TODO input validation

			tx, err := db.BeginTx(r.Context(), nil)
			if err != nil {
				http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
				return
			}
			defer tx.Rollback()

			var surveyId int
			err = tx.QueryRowContext(r.Context(), `
				INSERT INTO survey (title, description) VALUES (?, ?)
				RETURNING id`,
				survey.Title,
				survey.Description,
			).Scan(&surveyId)
			if err != nil {
				http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
				return
			}

			stmt, err := tx.PrepareContext(r.Context(), `
				INSERT INTO survey_field (survey_id, type, name, label, required, options)
				VALUES (?, ?, ?, ?, ?, ?)`)
			if err != nil {
				http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
				return
			}
			defer stmt.Close()

			names := make([]string, len(survey.Fields))
			for i, f := range survey.Fields {
				name := strings.ToLower(f.Label)
				name = reNoIdent.ReplaceAllLiteralString(name, " ")
				name = strings.Join(strings.Fields(name), "_")

				n := 0
				for _, prev := range names[:i] {
					if prev == name {
						n++
					}
				}
				if n > 0 {
					name = fmt.Sprintf("%s__%d", name, n)
				}

				var optionsJson []byte
				if f.Options != nil {
					optionsJson, err = json.Marshal(f.Options)
					if err != nil {
						http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
						return
					}
				}
				_, err := stmt.ExecContext(r.Context(), surveyId, f.Type, name, f.Label, f.Required, string(optionsJson))
				if err != nil {
					http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
					return
				}
			}

			err = tx.Commit()
			if err != nil {
				http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
				return
			}

			// write response
			w.WriteHeader(http.StatusCreated)
			render.JSON(w, r, map[string]any{
				"id": surveyId,
			})
		})
		r.Get("/surveys", func(w http.ResponseWriter, r *http.Request) {
			rows, err := db.QueryContext(r.Context(), `
				SELECT s.id, s.version, s.title, s.description
				FROM survey s`)
			if err != nil {
				http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
				return
			}
			defer rows.Close()

			surveys := []Survey{}
			for rows.Next() {
				s := Survey{}
				err = rows.Scan(&s.ID, &s.Version, &s.Title, &s.Description)
				if err != nil {
					http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
					return
				}

				surveys = append(surveys, s)
			}

			render.JSON(w, r, map[string]any{"surveys": surveys})
		})
		r.Get(`/surveys/{id:^\d+$}`, func(w http.ResponseWriter, r *http.Request) {
			surveyId, err := strconv.Atoi(chi.URLParam(r, "id"))
			if err != nil {
				http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
				return
			}

			rows, err := db.QueryContext(r.Context(), `
				SELECT
					s.id, s.version, s.title, s.description,
					f.type, f.name, f.label, f.required, f.options
				FROM survey s
				LEFT OUTER JOIN survey_field f ON (s.id = f.survey_id)
				WHERE s.id = ?`,
				surveyId,
			)
			if err != nil {
				http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
				return
			}
			defer rows.Close()

			// XXX this is born of a refactor that had me hunting and poking around the file for 15 minutes
			if !rows.Next() {
				http.Error(w, http.StatusText(http.StatusNotFound), http.StatusNotFound)
				return
			}

			s := Survey{}
			for {
				f := SurveyField{}
				var opts string
				err = rows.Scan(
					&s.ID, &s.Version, &s.Title, &s.Description,
					&f.Type, &f.Name, &f.Label, &f.Required, &opts,
				)
				if err != nil {
					http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
					return
				}

				if opts != "" {
					err = json.Unmarshal([]byte(opts), &f.Options)
					if err != nil {
						http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
						return
					}
				}

				s.Fields = append(s.Fields, f)

				if !rows.Next() {
					break
				}
			}

			render.JSON(w, r, s)
		})
		r.Put(`/surveys/{id:^\d+$}`, func(w http.ResponseWriter, r *http.Request) {
			surveyId, err := strconv.Atoi(chi.URLParam(r, "id"))
			if err != nil {
				http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
				return
			}

			survey := Survey{}
			err = render.DecodeJSON(r.Body, &survey)
			if err != nil {
				http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
				return
			}

			tx, err := db.BeginTx(r.Context(), nil)
			if err != nil {
				http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
				return
			}
			defer tx.Rollback()

			// delete all fields
			tx.ExecContext(r.Context(), `
				DELETE FROM survey_field
				WHERE survey_id = ?`,
				surveyId,
			)

			// recreate all fields
			stmt, err := tx.PrepareContext(r.Context(), `
				INSERT INTO survey_field (survey_id, type, name, label, required, options)
				VALUES (?, ?, ?, ?, ?, ?)`)
			if err != nil {
				http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
				return
			}
			defer stmt.Close()

			names := make([]string, len(survey.Fields))
			for i, f := range survey.Fields {
				name := strings.ToLower(f.Label)
				name = reNoIdent.ReplaceAllLiteralString(name, " ")
				name = strings.Join(strings.Fields(name), "_")

				n := 0
				for _, prev := range names[:i] {
					if prev == name { // FIXME could be already found! Need to use regexp
						n++
					}
				}
				if n > 0 {
					name = fmt.Sprintf("%s__%d", name, n)
				}

				var optionsJson []byte
				if f.Options != nil {
					optionsJson, err = json.Marshal(f.Options)
					if err != nil {
						http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
						return
					}
				}
				_, err := stmt.ExecContext(r.Context(), surveyId, f.Type, name, f.Label, f.Required, string(optionsJson))
				if err != nil {
					http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
					return
				}
			}

			res, err := tx.ExecContext(r.Context(), `
				UPDATE survey
				SET
					title = ?,
					description = ?,
					version = version+1
				WHERE	id = ?
					AND version = ?`,
				survey.Title,
				survey.Description,
				survey.Version,
				surveyId,
			)
			if err != nil {
				http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
				return
			}
			// optimistic lock
			n, err := res.RowsAffected()
			if err != nil {
				http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
				return
			}
			if n < 1 {
				http.Error(w, http.StatusText(http.StatusConflict), http.StatusConflict)
				return
			}

			err = tx.Commit()
			if err != nil {
				http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
				return
			}

			// write response
			w.WriteHeader(http.StatusNoContent)
		})
		r.Delete(`/surveys/{id:^\d+$}`, func(w http.ResponseWriter, r *http.Request) {
			surveyId, err := strconv.Atoi(chi.URLParam(r, "id"))
			if err != nil {
				http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
				return
			}

			tx, err := db.BeginTx(r.Context(), nil)
			if err != nil {
				http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
				return
			}
			defer tx.Rollback()

			_, err = tx.ExecContext(r.Context(), `
				DELETE FROM survey_field
				WHERE survey_id = ?`,
				surveyId,
			)
			if err != nil {
				http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
				return
			}

			res, err := tx.ExecContext(r.Context(), `
				DELETE FROM survey WHERE survey_id = ?`,
				surveyId,
			)
			if err != nil {
				http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
				return
			}
			n, err := res.RowsAffected()
			if err != nil {
				http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
				return
			}
			if n < 1 {
				w.WriteHeader(http.StatusNotFound)
				return
			}

			// write response
			w.WriteHeader(http.StatusNoContent)
		})

		// see survey results
		r.Get(`/surveys/{id:^\d+$}/submissions`, func(w http.ResponseWriter, r *http.Request) {
			surveyId, err := strconv.Atoi(chi.URLParam(r, "id"))
			if err != nil {
				http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
				return
			}

			rows, err := db.QueryContext(r.Context(), `
				SELECT
					x.id,
					s.id, s.time, s.ip,
					f.name, f.label, v.value
				FROM survey x
				LEFT OUTER JOIN submission s ON (x.id = s.survey_id)
				LEFT OUTER JOIN submission_field v ON (s.id = v.submission_id)
				LEFT OUTER JOIN survey_field f ON (f.id = v.field_id)
				WHERE x.id = ?`,
				surveyId,
			)
			if err != nil {
				http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
				return
			}
			defer rows.Close()

			if !rows.Next() {
				w.WriteHeader(http.StatusNotFound)
				return
			}

			// XXX this is getting a little too complicated for my tastes... :-(

			submissions := []Submission{}
			for {
				s := Submission{}
				f := SubmissionField{}
				var value string

				err = rows.Scan(&s.ID, &s.ID, &s.Time, &s.IP, &f.Name, &f.Label, &value)
				if err != nil {
					http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
					return
				}
				if s.ID == 0 {
					break
				}

				err = json.Unmarshal([]byte(value), &f.Value)
				if err != nil {
					http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
					return
				}

				lastIdx := len(submissions) - 1
				if lastIdx > -1 && submissions[lastIdx].ID == s.ID {
					submissions[lastIdx].Fields[f.Name] = f
				} else {
					s.Fields = map[string]SubmissionField{f.Name: f}
					submissions = append(submissions, s)
				}

				if !rows.Next() {
					break
				}
			}

			// finally return result
			render.JSON(w, r, map[string]any{"submissions": submissions})
		})
	})

	// login endpoint
	// XXX at a certian point, I realized I had not configured the user credentials...
	//		it took me 5 minutes just to find this line in the file!
	bearerServer := oauth.NewBearerServer("secret", 120*time.Second, &CredentialsVerifier{db: db}, nil)
	api.Post("/login", func(w http.ResponseWriter, r *http.Request) {
		user, pass, ok := r.BasicAuth()
		if !ok {
			http.Error(w, http.StatusText(http.StatusUnauthorized), http.StatusUnauthorized)
			return
		}

		body := url.Values{
			"grant_type": {"password"},
			"username":   {user},
			"password":   {pass},
		}
		r.Body = io.NopCloser(strings.NewReader(body.Encode()))
		r.Header.Set("content-type", "application/x-www-form-urlencoded")
		r.Header.Set("content-length", strconv.Itoa(len(body.Encode())))
		bearerServer.UserCredentials(w, r)
	})
	api.Post("/refresh", func(w http.ResponseWriter, r *http.Request) {
		auth := r.Header.Get("authorization")
		match := regexp.MustCompile(`(?i)^refresh\s+(.*)`).FindStringSubmatch(auth)
		if len(match) == 0 {
			http.Error(w, http.StatusText(http.StatusUnauthorized), http.StatusUnauthorized)
			return
		}
		token := match[1]

		body := url.Values{
			"grant_type":    {"refresh_token"},
			"refresh_token": {token},
		}

		req, err := http.NewRequest("POST", "/", strings.NewReader(body.Encode()))
		if err != nil {
			http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
			return
		}
		req.Header.Set("content-type", "application/x-www-form-urlencoded")
		req.Header.Set("content-length", strconv.Itoa(len(body.Encode())))

		resp := &ResponseBuffer{}
		bearerServer.UserCredentials(resp, req)
		resp.Flush(w)
	})

	// /surveys endpoints
	// XXX started 3 days after... took 10 minutes to find the next right piece of code to copy-pasta!

	// see survey
	api.Get(`/surveys/{id:^\d+$}`, func(w http.ResponseWriter, r *http.Request) {
		surveyId, err := strconv.Atoi(chi.URLParam(r, "id"))
		if err != nil {
			http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
			return
		}

		rows, err := db.QueryContext(r.Context(), `
			SELECT
				s.title, s.description,
				f.id, f.type, f.name, f.label, f.required, f.options
			FROM survey s
			LEFT OUTER JOIN survey_field f ON (s.id = f.survey_id)
			WHERE s.id = ?`,
			surveyId,
		)
		if err != nil {
			http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
			return
		}
		defer rows.Close()

		if !rows.Next() {
			http.Error(w, http.StatusText(http.StatusNotFound), http.StatusNotFound)
			return
		}

		s := Survey{}
		for {
			f := SurveyField{}
			var opts string
			err = rows.Scan(
				&s.Title, &s.Description, // changed this line (no id and version)
				&f.ID, &f.Type, &f.Name, &f.Label, &f.Required, &opts,
			)
			if err != nil {
				http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
				return
			}

			if opts != "" {
				err = json.Unmarshal([]byte(opts), &f.Options)
				if err != nil {
					http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
					return
				}
			}

			s.Fields = append(s.Fields, f)

			if !rows.Next() {
				break
			}
		}

		render.JSON(w, r, s)
	})

	validateIpStart := make(chan IpCheck)
	go func() {
		submissionIPs := make(map[string]bool)

		for {
			req := <-validateIpStart
			if req.op {
				req.result <- submissionIPs[req.ip]
				submissionIPs[req.ip] = true
			} else {
				delete(submissionIPs, req.ip)
			}
		}
	}()

	// submit survey
	api.Post(`/surveys/{id:^\d+$}/submissions`, func(w http.ResponseWriter, r *http.Request) {
		submission := Submission{}
		err := render.DecodeJSON(r.Body, &submission)
		if err != nil {
			http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
			return
		}

		// TODO input validation, i.e. required fields

		tx, err := db.BeginTx(r.Context(), nil)
		if err != nil {
			http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
			return
		}
		defer tx.Rollback()

		// get survey
		// XXX begin copy-pasta
		surveyId, err := strconv.Atoi(chi.URLParam(r, "id"))
		if err != nil {
			http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
			return
		}

		// change here: use current transaction
		rows, err := tx.QueryContext(r.Context(), `
			SELECT
				s.title, s.description,
				f.type, f.label, f.required, f.options
			FROM survey s
			LEFT OUTER JOIN survey_field f ON (s.id = f.survey_id)
			WHERE s.id = ?`,
			surveyId,
		)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				http.Error(w, http.StatusText(http.StatusNotFound), http.StatusNotFound)
			} else {
				http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
			}
			return
		}
		defer rows.Close()

		survey := Survey{}
		for rows.Next() {
			f := SurveyField{}
			var opts string
			err = rows.Scan(
				&survey.Title, &survey.Description,
				&f.Type, &f.Label, &f.Required, &opts,
			)
			if err != nil {
				http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
				return
			}

			if opts != "" {
				err = json.Unmarshal([]byte(opts), &f.Options)
				if err != nil {
					http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
					return
				}
			}

			survey.Fields = append(survey.Fields, f)
		}
		// XXX end copy-pasta

		// XXX the following 24 are the only exciting lines in this 140 lines function
		ip := strings.Split(r.RemoteAddr, ":")[0]
		// check ip is not submitting now
		validateIpDone := make(chan bool)
		validateIpStart <- IpCheck{true, ip, validateIpDone}
		if <-validateIpDone {
			http.Error(w, http.StatusText(http.StatusConflict), http.StatusConflict) // FIXME NOW!!!!!!!!!!!!!!!!!!!!!!!!!
			return
		}
		defer func() { validateIpStart <- IpCheck{false, ip, nil} }()
		// check ip did not already submit
		var alreadySubmitted bool
		err = db.QueryRowContext(r.Context(), `
			SELECT 1 FROM submission
			WHERE survey_id = ?
				AND ip = ?`,
			surveyId,
			ip,
		).Scan(&alreadySubmitted)
		if err != nil && !errors.Is(err, sql.ErrNoRows) {
			http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
			return
		}
		if alreadySubmitted {
			http.Error(w, http.StatusText(http.StatusConflict), http.StatusConflict)
			return
		}

		var submissionId int
		err = tx.QueryRowContext(r.Context(), `
			INSERT INTO submission (survey_id, time, ip) VALUES (?, ?, ?)
			RETURNING id`,
			surveyId,
			time.Now(),
			ip,
		).Scan(&submissionId)
		if err != nil {
			http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
			return
		}

		stmt, err := tx.PrepareContext(r.Context(), `
			INSERT INTO submission_field (submission_id, field_id, value)
			VALUES (?, ?, ?)`)
		if err != nil {
			http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
			return
		}
		defer stmt.Close()

		for _, f := range submission.Fields {
			var valueJson []byte
			if f.Value != nil {
				valueJson, err = json.Marshal(f.Value)
				if err != nil {
					http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
					return
				}
			}
			_, err := stmt.ExecContext(r.Context(), submissionId, f.ID, string(valueJson))
			if err != nil {
				http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
				return
			}
		}

		err = tx.Commit()
		if err != nil {
			http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
			return
		}

		// write response
		w.WriteHeader(http.StatusCreated)
		render.JSON(w, r, map[string]any{
			"id": submissionId,
		})
	})

	root := chi.NewRouter()
	root.Use(middleware.Logger, middleware.Recoverer)
	root.Mount("/api", api)
	// serve static content
	cookieAuth := func(h http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method != "GET" {
				h.ServeHTTP(w, r)
				return
			}

			token, err := r.Cookie("access_token")
			if err != nil && !errors.Is(err, http.ErrNoCookie) {
				http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
				return
			}
			if err == nil {
				r.Header.Set("authorization", "Bearer "+token.Value)
				buf := &ResponseBuffer{}
				h.ServeHTTP(buf, r)
				if buf.status != 401 {
					buf.Flush(w)
					return
				}
			}

			// XXX wanted to add this feature after a week... had to study how this function works again
			loginLocation := "/login?goto=" + url.QueryEscape(r.RequestURI)

			// token was empty or unauthorized
			refreshToken, err := r.Cookie("refresh_token")
			if err != nil {
				if !errors.Is(err, http.ErrNoCookie) {
					http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
					return
				}

				// refresh token was empty: redirect to login page
				w.Header().Set("location", loginLocation)
				w.WriteHeader(http.StatusTemporaryRedirect)
				return
			}

			// produce new token by calling bearer server
			// XXX this is GROSS... oauth.BearerServer has a bad interface for handling this...
			body := url.Values{
				"grant_type":    {"refresh_token"},
				"refresh_token": {refreshToken.Value},
			}
			req, err := http.NewRequest("POST", "/", strings.NewReader(body.Encode()))
			if err != nil {
				http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
				return
			}
			req.Header.Set("content-type", "application/x-www-form-urlencoded")
			req.Header.Set("content-length", strconv.Itoa(len(body.Encode())))

			resp := &ResponseBuffer{}
			bearerServer.UserCredentials(resp, req)
			if resp.status == 401 {
				// redirect to login page
				w.Header().Set("location", loginLocation)
				http.SetCookie(w, &http.Cookie{
					Path:     "/",
					Name:     "refresh_token",
					Value:    "",
					MaxAge:   -1,
					SameSite: http.SameSiteNoneMode,
				})
				w.WriteHeader(http.StatusTemporaryRedirect)
				return
			}
			if resp.status != 200 {
				http.Error(w, http.StatusText(resp.status), resp.status)
				return
			}

			var responseBody map[string]any
			err = json.Unmarshal(resp.body.Bytes(), &responseBody)
			if err != nil {
				http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
				return
			}

			token = &http.Cookie{
				Path:     "/",
				Name:     "access_token",
				Value:    responseBody["access_token"].(string),
				MaxAge:   int(responseBody["expires_in"].(float64)),
				SameSite: http.SameSiteNoneMode,
			}
			http.SetCookie(w, token)

			refreshToken = &http.Cookie{
				Path:     "/",
				Name:     "refresh_token",
				Value:    responseBody["refresh_token"].(string),
				MaxAge:   60 * 60 * 24 * 365,
				SameSite: http.SameSiteNoneMode,
			}
			http.SetCookie(w, refreshToken)

			r.Header.Set("authorization", "Bearer "+token.Value)
			h.ServeHTTP(w, r)
		})
	}
	root.
		With(cookieAuth, oauth.Authorize("secret", nil), adminMiddleware). // TODO refactor, move secret to file
		Mount("/admin", http.StripPrefix("/admin", http.FileServer(http.Dir("private"))))
		// XXX missing StripPrefix had me jumping about the code for more than 2 hours...
	root.Mount("/", http.FileServer(http.Dir("public")))

	// start server
	srv := &http.Server{
		Addr:         "0.0.0.0:8080",
		Handler:      root,
		IdleTimeout:  time.Minute,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 30 * time.Second,
	}
	err = srv.ListenAndServe()
	if !errors.Is(err, http.ErrServerClosed) {
		panic(err)
	}
}

type ResponseBuffer struct {
	status int
	header http.Header
	body   *bytes.Buffer
}

func (resp *ResponseBuffer) Header() http.Header {
	if resp.header == nil {
		resp.header = http.Header{}
	}
	return resp.header
}
func (resp *ResponseBuffer) Write(body []byte) (int, error) {
	if resp.body == nil {
		resp.body = &bytes.Buffer{}
	}
	return resp.body.Write(body)
}
func (resp *ResponseBuffer) WriteHeader(statusCode int) {
	resp.status = statusCode
}
func (resp *ResponseBuffer) Flush(w http.ResponseWriter) error {
	if resp.header != nil {
		header := w.Header()
		for key, value := range resp.header {
			header[key] = value
		}
	}
	if resp.status != 0 {
		w.WriteHeader(resp.status)
	}
	if resp.body != nil {
		_, err := w.Write(resp.body.Bytes())
		return err
	}
	return nil
}

type CredentialsVerifier struct {
	db *sql.DB
}

func (cs *CredentialsVerifier) ValidateUser(username string, password string, scope string, r *http.Request) error {
	var hash []byte
	err := cs.db.
		QueryRow("SELECT password_hash FROM user WHERE username=?", username).
		Scan(&hash)
	if err != nil {
		return err
	}

	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(password))
}
func (cs *CredentialsVerifier) StoreTokenID(tokenType oauth.TokenType, credential string, tokenID string, refreshTokenID string) error {
	_, err := cs.db.Exec(
		"INSERT INTO token (username, token_id, refresh_token_id, expiration) VALUES (?, ?, ?, ?)",
		credential,
		tokenID,
		refreshTokenID,
		time.Now().Add(8760*time.Hour),
	)
	return err
}
func (cs *CredentialsVerifier) ValidateTokenID(tokenType oauth.TokenType, credential string, tokenID string, refreshTokenID string) error {
	var expiration time.Time
	var ok bool

	cs.db.
		QueryRow(`
			DELETE FROM token
			WHERE username = ?
				AND token_id = ?
				AND refresh_token_id = ?
			RETURNING expiration, 1`,
			credential,
			tokenID,
			refreshTokenID,
		).
		Scan(&expiration, &ok)
	if !ok {
		return errors.New("could not refresh")
	}

	if expiration.Before(time.Now()) {
		return errors.New("could not refresh")
	}
	return nil
}
func (*CredentialsVerifier) AddClaims(tokenType oauth.TokenType, credential string, tokenID string, scope string, r *http.Request) (map[string]string, error) {
	return map[string]string{"roles": "admin"}, nil
}
func (*CredentialsVerifier) AddProperties(tokenType oauth.TokenType, credential string, tokenID string, scope string, r *http.Request) (map[string]string, error) {
	return map[string]string{}, nil
}
func (*CredentialsVerifier) ValidateClient(clientID string, clientSecret string, scope string, r *http.Request) error {
	return errors.New("not supported")
}
