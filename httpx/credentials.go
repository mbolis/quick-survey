package httpx

import (
	"database/sql"
	"errors"
	"net/http"
	"time"

	"github.com/go-chi/oauth"
	"golang.org/x/crypto/bcrypt"
)

type credentialsVerifier struct {
	db *sql.DB
}

func CredentialsVerifier(db *sql.DB) oauth.CredentialsVerifier {
	return &credentialsVerifier{db}
}

func (cs *credentialsVerifier) ValidateUser(username string, password string, scope string, r *http.Request) error {
	var hash []byte
	err := cs.db.
		QueryRow("SELECT password_hash FROM user WHERE username=?", username).
		Scan(&hash)
	if err != nil {
		return err
	}

	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(password))
}
func (cs *credentialsVerifier) StoreTokenID(tokenType oauth.TokenType, credential string, tokenID string, refreshTokenID string) error {
	_, err := cs.db.Exec(
		"INSERT INTO token (username, token_id, refresh_token_id, expiration) VALUES (?, ?, ?, ?)",
		credential,
		tokenID,
		refreshTokenID,
		time.Now().Add(8760*time.Hour),
	)
	return err
}
func (cs *credentialsVerifier) ValidateTokenID(tokenType oauth.TokenType, credential string, tokenID string, refreshTokenID string) error {
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
func (*credentialsVerifier) AddClaims(tokenType oauth.TokenType, credential string, tokenID string, scope string, r *http.Request) (map[string]string, error) {
	return map[string]string{"roles": "admin"}, nil
}
func (*credentialsVerifier) AddProperties(tokenType oauth.TokenType, credential string, tokenID string, scope string, r *http.Request) (map[string]string, error) {
	return map[string]string{}, nil
}
func (*credentialsVerifier) ValidateClient(clientID string, clientSecret string, scope string, r *http.Request) error {
	return errors.New("not supported")
}
