package httpx

import (
	"fmt"
	"net/http"

	"github.com/mbolis/quick-survey/log"
)

// Will log an error, and send an HTTP response with status 500 and default text
func LogInternalError(w http.ResponseWriter, code string, err error) {
	log.Errorf("%s: %s", code, err)
	http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
}

// Will log a debug message, and send an HTTP response with status 404 and default text
func LogNotFound(w http.ResponseWriter, code string, id any) {
	log.Debugf("%s: not found (%v)", code, id)
	w.WriteHeader(http.StatusNotFound)
}

// Will log an error code at the given level, and send
// an HTTP response with status and default text
func LogStatus(w http.ResponseWriter, status int, level log.Level, code string) {
	log.Log(level, code)
	http.Error(w, http.StatusText(status), status)
}

// Will log an error code and message at the given level,
// and send an HTTP response with the given status and formatted message
func LogStatusMsg(w http.ResponseWriter, status int, level log.Level, code string, msg string, args ...any) {
	errMsg := fmt.Sprintf(msg, args...)
	log.Log(level, code+":", errMsg)
	http.Error(w, errMsg, status)
}
