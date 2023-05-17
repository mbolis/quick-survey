package httpx

import (
	"bytes"
	"net/http"
)

type ResponseBuffer interface {
	http.ResponseWriter
	Status() int
	Body() []byte
	Flush(w http.ResponseWriter) error
}

type responseBuffer struct {
	status int
	header http.Header
	body   *bytes.Buffer
}

func NewResponseBuffer() ResponseBuffer {
	return &responseBuffer{}
}

func (resp *responseBuffer) Status() int {
	return resp.status
}

func (resp *responseBuffer) Header() http.Header {
	if resp.header == nil {
		resp.header = http.Header{}
	}
	return resp.header
}

func (resp *responseBuffer) Body() []byte {
	return resp.body.Bytes()
}

func (resp *responseBuffer) Write(body []byte) (int, error) {
	if resp.body == nil {
		resp.body = &bytes.Buffer{}
	}
	return resp.body.Write(body)
}

func (resp *responseBuffer) WriteHeader(statusCode int) {
	resp.status = statusCode
}

func (resp *responseBuffer) Flush(w http.ResponseWriter) error {
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
