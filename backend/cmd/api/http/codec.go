package http

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/http"
)

var (
	ErrContentTypeNotJSON = errors.New("content type of request is not application/json")
)

// bodyDecoder decodes the request body as JSON into a new T.
//
// Decoding is lenient: unknown fields and any content trailing the first JSON
// value are ignored. On failure it returns the zero value of T along with the
// decoder error, which may name Go types and struct fields and so must not be
// forwarded to the client.
func bodyDecoder[T any](r *http.Request) (T, error) {
	decoded := new(T)
	if r.Header.Get("Content-Type") != "application/json" {
		return *decoded, ErrContentTypeNotJSON
	}
	if err := json.NewDecoder(r.Body).Decode(decoded); err != nil {
		return *decoded, err
	}
	return *decoded, nil
}

// noContent responds with 204 No Content, discarding the value. It never fails.
func noContent[T any](w http.ResponseWriter, _ T) error {
	w.WriteHeader(http.StatusNoContent)
	return nil
}

// created responds with 201 Created and value as a JSON body. See writeJSON for
// the failure behaviour.
func created[T any](w http.ResponseWriter, value T) error {
	return writeJSON(w, http.StatusCreated, value)
}

// ok responds with 200 OK and value as a JSON body. See writeJSON for the
// failure behaviour.
func ok[T any](w http.ResponseWriter, value T) error {
	return writeJSON(w, http.StatusOK, value)
}

// writeJSON encodes value and, only once encoding succeeded, commits status and
// the JSON body to w
func writeJSON[T any](w http.ResponseWriter, status int, value T) error {
	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(value); err != nil {
		return err
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_, err := w.Write(buf.Bytes())
	return err
}
