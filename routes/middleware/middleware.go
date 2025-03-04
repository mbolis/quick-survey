package middleware

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/oauth"
	"github.com/mbolis/quick-survey/app"
	"github.com/mbolis/quick-survey/httpx"
	"github.com/mbolis/quick-survey/log"
)

func Default(next http.Handler) http.Handler {
	return chi.Chain(middleware.RequestLogger(logFormatter), middleware.Recoverer).Handler(next)
}

var logFormatter = &middleware.DefaultLogFormatter{
	Logger: log.Logger(),
}

// Admin middleware to check for the 'admin' role in an OAuth token.
func Admin(app app.App) func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return chi.Chain(oauth.Authorize(app.TokenSecret, nil), admin).Handler(next)
	}
}

func admin(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		claims := r.Context().Value(oauth.ClaimsContext).(map[string]string)

		isAdmin := false
		if rolesClaim, ok := claims["roles"]; ok {
			roles := strings.Split(rolesClaim, ",")
			for _, role := range roles {
				if role == "admin" {
					isAdmin = true
					break
				}
			}
		}

		if !isAdmin {
			http.Error(w, http.StatusText(http.StatusForbidden), http.StatusForbidden)
			return
		}

		next.ServeHTTP(w, r)
	})
}

func CookieAuth(app app.App) func(http.Handler) http.Handler {
	return func(h http.Handler) http.Handler {
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
				buf := httpx.NewResponseBuffer()
				h.ServeHTTP(buf, r)
				if buf.Status() != 401 {
					buf.Flush(w)
					return
				}
			}

			// XXX wanted to add this feature after a week... had to study how this function works again
			loginLocation := "/login?goto=" + url.QueryEscape(r.RequestURI)
			revokeCookie(w, "access_token")

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

			resp := httpx.NewResponseBuffer()
			app.UserCredentials(resp, req)
			if resp.Status() == 401 {
				// redirect to login page
				w.Header().Set("location", loginLocation)
				revokeCookie(w, "refresh_token")
				w.WriteHeader(http.StatusTemporaryRedirect)
				return
			}
			if resp.Status() != 200 {
				http.Error(w, http.StatusText(resp.Status()), resp.Status())
				return
			}

			var responseBody map[string]any
			err = json.Unmarshal(resp.Body(), &responseBody)
			if err != nil {
				http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
				return
			}

			token = &http.Cookie{
				Path:     "/",
				Name:     "access_token",
				Value:    responseBody["access_token"].(string),
				MaxAge:   int(responseBody["expires_in"].(float64)),
				HttpOnly: true,
				SameSite: http.SameSiteNoneMode,
			}
			http.SetCookie(w, token)

			refreshToken = &http.Cookie{
				Path:     "/",
				Name:     "refresh_token",
				Value:    responseBody["refresh_token"].(string),
				MaxAge:   60 * 60 * 24 * 365,
				HttpOnly: true,
				SameSite: http.SameSiteNoneMode,
			}
			http.SetCookie(w, refreshToken)

			r.Header.Set("authorization", "Bearer "+token.Value)
			h.ServeHTTP(w, r)
		})
	}
}

func revokeCookie(w http.ResponseWriter, name string) {
	http.SetCookie(w, &http.Cookie{
		Name:     name,
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
		SameSite: http.SameSiteNoneMode,
	})
}
