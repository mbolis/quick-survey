package model

import "time"

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
