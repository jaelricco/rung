package httpx

import (
	"encoding/json"
	"log"
	"net/http"
	"runtime/debug"
	"time"
)

// JSON writes v as the response body.
func JSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	if v == nil {
		return
	}
	if err := json.NewEncoder(w).Encode(v); err != nil {
		log.Printf("write response: %v", err)
	}
}

// Fail writes an error the frontend can show verbatim. Messages say what went
// wrong and what to do about it, never just "bad request".
func Fail(w http.ResponseWriter, status int, message string) {
	JSON(w, status, map[string]string{"error": message})
}

// Decode reads a JSON body, rejecting unknown fields so typos surface early.
func Decode(w http.ResponseWriter, r *http.Request, dst any) bool {
	dec := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20))
	dec.DisallowUnknownFields()
	if err := dec.Decode(dst); err != nil {
		Fail(w, http.StatusBadRequest, "That request body couldn't be read: "+err.Error())
		return false
	}
	return true
}

type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (s *statusRecorder) WriteHeader(code int) {
	s.status = code
	s.ResponseWriter.WriteHeader(code)
}

// Logging records method, path, status and duration for every request.
func Logging(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		rec := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(rec, r)
		log.Printf("%s %s %d %s", r.Method, r.URL.Path, rec.status, time.Since(start).Round(time.Millisecond))
	})
}

// Recover turns a panic into a 500 instead of killing the process.
func Recover(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if rec := recover(); rec != nil {
				log.Printf("panic on %s %s: %v\n%s", r.Method, r.URL.Path, rec, debug.Stack())
				Fail(w, http.StatusInternalServerError, "Something broke on our side. Try again.")
			}
		}()
		next.ServeHTTP(w, r)
	})
}
