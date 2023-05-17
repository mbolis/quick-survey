package routes

import (
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"
	"github.com/mbolis/quick-survey/model"
)

func PublicGetSurveyById(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
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

		s := model.Survey{}
		for {
			f := model.SurveyField{}
			var opts string
			err = rows.Scan(
				&s.Title, &s.Description,
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
	}
}

type IpCheck struct {
	op     bool
	ip     string
	result chan<- bool
}

func PublicSubmitSurvey(db *sql.DB) http.HandlerFunc {
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

	return func(w http.ResponseWriter, r *http.Request) {
		submission := model.Submission{}
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

		survey := model.Survey{}
		for rows.Next() {
			f := model.SurveyField{}
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

		// TODO move to own module
		ip := strings.Split(r.RemoteAddr, ":")[0]
		// check ip is not submitting now
		validateIpDone := make(chan bool)
		validateIpStart <- IpCheck{true, ip, validateIpDone}
		if <-validateIpDone {
			http.Error(w, "Una risposta è già pervenuta da questo IP", http.StatusConflict) // TODO i18n
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
			http.Error(w, "Una risposta è già pervenuta da questo IP", http.StatusInternalServerError)
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
	}
}
