package routes

import (
	"database/sql"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/oauth"
	"github.com/mbolis/quick-survey/httpx"
)

func NewBearerServer(db *sql.DB) *oauth.BearerServer {
	// TODO move secret and token ttl to config file
	return oauth.NewBearerServer("secret", 120*time.Second, httpx.CredentialsVerifier(db), nil)
}

func Login(bearerServer *oauth.BearerServer) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
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
	}
}

func Refresh(bearerServer *oauth.BearerServer) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
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

		resp := httpx.NewResponseBuffer()
		bearerServer.UserCredentials(resp, req)
		resp.Flush(w)
	}
}
