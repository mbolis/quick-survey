package routes

import (
	"encoding/json"
	"fmt"
	"net/http"
	"regexp"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"
	"github.com/mbolis/quick-survey/app"
	"github.com/mbolis/quick-survey/httpx"
	"github.com/mbolis/quick-survey/log"
	"github.com/mbolis/quick-survey/model"
)

var reNoIdent = regexp.MustCompile(`\W+`)

func CreateSurvey(app app.App) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		survey := model.Survey{}
		err := render.DecodeJSON(r.Body, &survey)
		if err != nil {
			httpx.LogStatus(w, http.StatusBadRequest, log.DebugLevel, "request.parse_body")
			return
		}

		// TODO input validation

		tx, err := app.BeginTx(r.Context(), nil)
		if err != nil {
			httpx.LogInternalError(w, "db.begin_tx", err)
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
			httpx.LogInternalError(w, "db.insert_survey", err)
			return
		}

		stmt, err := tx.PrepareContext(r.Context(), `
		INSERT INTO survey_field (survey_id, type, name, label, required, options)
		VALUES (?, ?, ?, ?, ?, ?)`)
		if err != nil {
			httpx.LogInternalError(w, "db.insert_survey.fields.prepare", err)
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
					httpx.LogInternalError(w, "db.insert_survey.fields.parse_options", err)
					return
				}
			}
			_, err := stmt.ExecContext(r.Context(), surveyId, f.Type, name, f.Label, f.Required, string(optionsJson))
			if err != nil {
				httpx.LogInternalError(w, "db.insert_survey.fields.insert", err)
				return
			}
		}

		err = tx.Commit()
		if err != nil {
			httpx.LogInternalError(w, "db.insert_survey.commit", err)
			return
		}

		w.WriteHeader(http.StatusCreated)
		render.JSON(w, r, map[string]any{
			"id": surveyId,
		})
	}
}

func ListSurveys(app app.App) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		rows, err := app.QueryContext(r.Context(), `
		SELECT s.id, s.version, s.title, s.description
		FROM survey s`)
		if err != nil {
			httpx.LogInternalError(w, "db.get_surveys", err)
			return
		}
		defer rows.Close()

		surveys := []model.Survey{}
		for rows.Next() {
			s := model.Survey{}
			err = rows.Scan(&s.ID, &s.Version, &s.Title, &s.Description)
			if err != nil {
				httpx.LogInternalError(w, "db.get_surveys.scan", err)
				return
			}

			surveys = append(surveys, s)
		}

		render.JSON(w, r, map[string]any{
			"surveys": surveys,
		})
	}
}

func GetSurveyById(app app.App) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		surveyId, err := strconv.Atoi(chi.URLParam(r, "id"))
		if err != nil {
			httpx.LogStatus(w, http.StatusBadRequest, log.DebugLevel, "request.get_url_param.id")
			return
		}

		rows, err := app.QueryContext(r.Context(), `
			SELECT
				s.id, s.version, s.title, s.description,
				f.type, f.name, f.label, f.required, f.options
			FROM survey s
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

		survey := model.Survey{}
		for {
			f := model.SurveyField{}
			var opts string
			err = rows.Scan(
				&survey.ID, &survey.Version, &survey.Title, &survey.Description,
				&f.Type, &f.Name, &f.Label, &f.Required, &opts,
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

func UpdateSurvey(app app.App) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		surveyId, err := strconv.Atoi(chi.URLParam(r, "id"))
		if err != nil {
			httpx.LogStatus(w, http.StatusBadRequest, log.DebugLevel, "request.get_url_param.id")
			return
		}

		survey := model.Survey{}
		err = render.DecodeJSON(r.Body, &survey)
		if err != nil {
			httpx.LogStatus(w, http.StatusBadRequest, log.DebugLevel, "request.parse_body")
			return
		}

		tx, err := app.BeginTx(r.Context(), nil)
		if err != nil {
			httpx.LogInternalError(w, "db.begin_tx", err)
			return
		}
		defer tx.Rollback()

		// delete all fields
		_, err = tx.ExecContext(r.Context(), `
			DELETE FROM survey_field
			WHERE survey_id = ?`,
			surveyId,
		)
		if err != nil {
			httpx.LogInternalError(w, "db.update_survey.delete_fields", err)
			return
		}

		// recreate all fields
		stmt, err := tx.PrepareContext(r.Context(), `
			INSERT INTO survey_field (survey_id, type, name, label, required, options)
			VALUES (?, ?, ?, ?, ?, ?)`)
		if err != nil {
			httpx.LogInternalError(w, "db.update_survey.fields.prepare", err)
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
					httpx.LogInternalError(w, "db.update_survey.fields.parse_options", err)
					return
				}
			}
			_, err := stmt.ExecContext(r.Context(), surveyId, f.Type, name, f.Label, f.Required, string(optionsJson))
			if err != nil {
				httpx.LogInternalError(w, "db.update_survey.fields.update", err)
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
			surveyId,
			survey.Version,
		)
		if err != nil {
			httpx.LogInternalError(w, "db.update_survey", err)
			return
		}
		// optimistic lock
		n, err := res.RowsAffected()
		if err != nil {
			httpx.LogInternalError(w, "db.update_survey.verify", err)
			return
		}
		if n < 1 {
			httpx.LogStatus(w, http.StatusConflict, log.DebugLevel, "db.update_survey.verify.conflict")
			return
		}

		err = tx.Commit()
		if err != nil {
			httpx.LogInternalError(w, "db.update_survey.commit", err)
			return
		}

		w.WriteHeader(http.StatusNoContent)
	}
}

func DeleteSurvey(app app.App) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		surveyId, err := strconv.Atoi(chi.URLParam(r, "id"))
		if err != nil {
			httpx.LogStatus(w, http.StatusBadRequest, log.DebugLevel, "request.get_url_param.id")
			return
		}

		tx, err := app.BeginTx(r.Context(), nil)
		if err != nil {
			httpx.LogInternalError(w, "db.begin_tx", err)
			return
		}
		defer tx.Rollback()

		_, err = tx.ExecContext(r.Context(), `
			DELETE FROM survey_field
			WHERE survey_id = ?`,
			surveyId,
		)
		if err != nil {
			httpx.LogInternalError(w, "db.delete_survey.fields", err)
			return
		}

		res, err := tx.ExecContext(r.Context(), `
			DELETE FROM survey WHERE survey_id = ?`,
			surveyId,
		)
		if err != nil {
			httpx.LogInternalError(w, "db.delete_survey", err)
			return
		}
		n, err := res.RowsAffected()
		if err != nil {
			httpx.LogInternalError(w, "db.delete_survey.verify", err)
			return
		}
		if n < 1 {
			httpx.LogNotFound(w, "delete_survey", surveyId)
			return
		}

		w.WriteHeader(http.StatusNoContent)
	}
}

func GetSurveySubmissions(app app.App) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		surveyId, err := strconv.Atoi(chi.URLParam(r, "id"))
		if err != nil {
			httpx.LogStatus(w, http.StatusBadRequest, log.DebugLevel, "request.get_url_param.id")
			return
		}

		rows, err := app.QueryContext(r.Context(), `
			SELECT
				sf.id, sf.time, sf.ip,
				sf.name, sf.label, sf.value
			FROM survey x
			LEFT OUTER JOIN (
				SELECT
					s.survey_id,
					s.id, s.time, s.ip,
					f.name, f.label, v.value
				FROM submission s
				INNER JOIN submission_field v ON (s.id = v.submission_id)
				INNER JOIN survey_field f ON (f.id = v.field_id)
			) sf ON (x.id = sf.survey_id)
			WHERE x.id = ?`,
			surveyId,
		)
		if err != nil {
			httpx.LogInternalError(w, "db.get_submissions", err)
			return
		}
		defer rows.Close()

		if !rows.Next() {
			httpx.LogNotFound(w, "get_submissions", surveyId)
			return
		}

		submissions := []model.Submission{}
		var dummy *string
		rows.Scan(&dummy, &dummy, &dummy, &dummy, &dummy, &dummy)
		if dummy == nil {
			render.JSON(w, r, map[string]any{
				"submissions": submissions,
			})
			return
		}

		for {
			s := model.Submission{}
			f := model.SubmissionField{}
			var value string

			err = rows.Scan(&s.ID, &s.Time, &s.IP, &f.Name, &f.Label, &value)
			if err != nil {
				httpx.LogInternalError(w, "db.get_submissions.scan", err)
				return
			}
			if s.ID == 0 {
				break
			}

			err = json.Unmarshal([]byte(value), &f.Value)
			if err != nil {
				httpx.LogInternalError(w, "db.get_submissions.parse_value", err)
				return
			}

			lastIdx := len(submissions) - 1
			if lastIdx > -1 && submissions[lastIdx].ID == s.ID {
				submissions[lastIdx].Fields[f.Name] = f
			} else {
				s.Fields = map[string]model.SubmissionField{f.Name: f}
				submissions = append(submissions, s)
			}

			if !rows.Next() {
				break
			}
		}

		render.JSON(w, r, map[string]any{
			"submissions": submissions,
		})
	}
}
