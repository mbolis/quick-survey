package routes

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"regexp"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"
	"github.com/mbolis/quick-survey/model"
)

var reNoIdent = regexp.MustCompile(`\W+`)

func CreateSurvey(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		survey := model.Survey{}
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
	}
}

func ListSurveys(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		rows, err := db.QueryContext(r.Context(), `
		SELECT s.id, s.version, s.title, s.description
		FROM survey s`)
		if err != nil {
			http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
			return
		}
		defer rows.Close()

		surveys := []model.Survey{}
		for rows.Next() {
			s := model.Survey{}
			err = rows.Scan(&s.ID, &s.Version, &s.Title, &s.Description)
			if err != nil {
				http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
				return
			}

			surveys = append(surveys, s)
		}

		render.JSON(w, r, map[string]any{
			"surveys": surveys,
		})
	}
}

func GetSurveyById(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
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

		if !rows.Next() {
			http.Error(w, http.StatusText(http.StatusNotFound), http.StatusNotFound)
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

			if !rows.Next() {
				break
			}
		}

		render.JSON(w, r, survey)
	}
}

func UpdateSurvey(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		surveyId, err := strconv.Atoi(chi.URLParam(r, "id"))
		if err != nil {
			http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
			return
		}

		survey := model.Survey{}
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
			surveyId,
			survey.Version,
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
	}
}

func DeleteSurvey(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
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
	}
}

func GetSurveySubmissions(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		surveyId, err := strconv.Atoi(chi.URLParam(r, "id"))
		if err != nil {
			http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
			return
		}

		rows, err := db.QueryContext(r.Context(), `
			SELECT
				s.id, s.time, s.ip,
				f.name, f.label, v.value
			FROM survey x
			INNER JOIN submission s ON (x.id = s.survey_id)
			INNER JOIN submission_field v ON (s.id = v.submission_id)
			INNER JOIN survey_field f ON (f.id = v.field_id)
			WHERE x.id = ?`,
			surveyId,
		)
		if err != nil {
			http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
			return
		}
		defer rows.Close()

		submissions := []model.Submission{}
		for rows.Next() {
			s := model.Submission{}
			f := model.SubmissionField{}
			var value string

			err = rows.Scan(&s.ID, &s.Time, &s.IP, &f.Name, &f.Label, &value)
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
				s.Fields = map[string]model.SubmissionField{f.Name: f}
				submissions = append(submissions, s)
			}
		}

		// finally return result
		render.JSON(w, r, map[string]any{"submissions": submissions})
	}
}
