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
	"github.com/mbolis/quick-survey/app"
	"github.com/mbolis/quick-survey/httpx"
	"github.com/mbolis/quick-survey/log"
	"github.com/mbolis/quick-survey/model"
)

func PublicGetSurveyById(app app.App) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		surveyId, err := strconv.Atoi(chi.URLParam(r, "id"))
		if err != nil {
			httpx.LogStatus(w, http.StatusBadRequest, log.DebugLevel, "request.get_url_param.id")
			return
		}

		rows, err := app.QueryContext(r.Context(), `
			SELECT
				s.title, s.description,
				sub.ip,
				f.id, f.type, f.name, f.label, f.required, f.options
			FROM survey s
			LEFT OUTER JOIN submission sub ON (s.id = sub.survey_id)
			LEFT OUTER JOIN survey_field f ON (s.id = f.survey_id)
			WHERE s.id = ?`,
			surveyId,
		)
		if err != nil {
			httpx.LogInternalError(w, "db.get_survey", err)
			return
		}
		defer rows.Close()

		if !rows.Next() {
			httpx.LogNotFound(w, "get_survey", surveyId)
			return
		}

		// TODO find a cleaner way?
		survey := model.Survey{}
		var ip string
		var dummy string
		err = rows.Scan(
			&survey.Title, &survey.Description,
			&ip,
			&dummy, &dummy, &dummy, &dummy, &dummy, &dummy,
		)
		if err != nil {
			httpx.LogInternalError(w, "db.get_survey.ip", err)
			return
		}
		if ip == strings.Split(r.RemoteAddr, ":")[0] {
			survey.Submitted = true
			render.JSON(w, r, survey)
			return
		}

		for {
			f := model.SurveyField{}
			var opts string
			err = rows.Scan(
				&survey.Title, &survey.Description,
				&dummy,
				&f.ID, &f.Type, &f.Name, &f.Label, &f.Required, &opts,
			)
			if err != nil {
				httpx.LogInternalError(w, "db.get_survey.scan", err)
				return
			}

			if opts != "" {
				err = json.Unmarshal([]byte(opts), &f.Options)
				if err != nil {
					httpx.LogInternalError(w, "db.get_survey.parse_options", err)
					return
				}
			}

			survey.Fields = append(survey.Fields, f)

			if !rows.Next() {
				break
			}
		}

		render.JSON(w, r, survey)
	}
}

type IpCheck struct {
	op     bool
	ip     string
	result chan<- bool
}

func PublicSubmitSurvey(app app.App) http.HandlerFunc {
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
		surveyId, err := strconv.Atoi(chi.URLParam(r, "id"))
		if err != nil {
			httpx.LogStatus(w, http.StatusBadRequest, log.DebugLevel, "request.get_url_param.id")
			return
		}

		submission := model.Submission{}
		err = render.DecodeJSON(r.Body, &submission)
		if err != nil {
			httpx.LogStatus(w, http.StatusBadRequest, log.DebugLevel, "request.parse_body")
			return
		}

		// TODO input validation, i.e. required fields

		tx, err := app.BeginTx(r.Context(), nil)
		if err != nil {
			httpx.LogInternalError(w, "db.begin_tx", err)
			return
		}
		defer tx.Rollback()

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
				httpx.LogNotFound(w, "get_survey", surveyId)
			} else {
				httpx.LogInternalError(w, "db.get_survey", err)
			}
			return
		}
		defer rows.Close()

		if !rows.Next() {
			httpx.LogNotFound(w, "get_survey", surveyId)
		}

		survey := model.Survey{}
		for {
			f := model.SurveyField{}
			var opts string
			err = rows.Scan(
				&survey.Title, &survey.Description,
				&f.Type, &f.Label, &f.Required, &opts,
			)
			if err != nil {
				httpx.LogInternalError(w, "db.get_survey.scan", err)
				return
			}

			if opts != "" {
				err = json.Unmarshal([]byte(opts), &f.Options)
				if err != nil {
					httpx.LogInternalError(w, "db.get_survey.parse_options", err)
					return
				}
			}

			survey.Fields = append(survey.Fields, f)

			if !rows.Next() {
				break
			}
		}

		// TODO move to own module
		ip := strings.Split(r.RemoteAddr, ":")[0]
		// check ip is not submitting now
		validateIpDone := make(chan bool)
		validateIpStart <- IpCheck{true, ip, validateIpDone}
		if <-validateIpDone {
			httpx.LogStatus(w, http.StatusConflict, log.DebugLevel, "ip.already_submitted")
			return
		}
		defer func() { validateIpStart <- IpCheck{false, ip, nil} }()
		// check ip did not already submit
		var alreadySubmitted bool
		err = app.QueryRowContext(r.Context(), `
			SELECT 1 FROM submission
			WHERE survey_id = ?
				AND ip = ?`,
			surveyId,
			ip,
		).Scan(&alreadySubmitted)
		if err != nil && !errors.Is(err, sql.ErrNoRows) {
			httpx.LogInternalError(w, "db.get_ip.scan", err)
			return
		}
		if alreadySubmitted {
			httpx.LogStatus(w, http.StatusConflict, log.DebugLevel, "ip.already_submitted")
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
			httpx.LogInternalError(w, "db.insert_submission", err)
			return
		}

		stmt, err := tx.PrepareContext(r.Context(), `
			INSERT INTO submission_field (submission_id, field_id, value)
			VALUES (?, ?, ?)`)
		if err != nil {
			httpx.LogInternalError(w, "db.insert_submission.fields.prepare", err)
			return
		}
		defer stmt.Close()

		for _, f := range submission.Fields {
			var valueJson []byte
			if f.Value != nil {
				valueJson, err = json.Marshal(f.Value)
				if err != nil {
					httpx.LogInternalError(w, "db.insert_submission.fields.parse_value", err)
					return
				}
			}
			_, err := stmt.ExecContext(r.Context(), submissionId, f.ID, string(valueJson))
			if err != nil {
				httpx.LogInternalError(w, "db.insert_submission.fields.insert", err)
				return
			}
		}

		err = tx.Commit()
		if err != nil {
			httpx.LogInternalError(w, "db.insert_submission.commit", err)
			return
		}

		// write response
		w.WriteHeader(http.StatusCreated)
		render.JSON(w, r, map[string]any{
			"id": submissionId,
		})
	}
}
