package httpapi

import (
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"mime"
	"net/http"

	"kitdesign/services/user/internal/apperr"
)

type problemBody struct {
	Type      string `json:"type"`
	Title     string `json:"title"`
	Status    int    `json:"status"`
	Code      string `json:"code"`
	Detail    string `json:"detail,omitempty"`
	RequestID string `json:"request_id,omitempty"`
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	if value != nil {
		_ = json.NewEncoder(w).Encode(value)
	}
}

func decodeJSON(w http.ResponseWriter, r *http.Request, target any) error {
	mediaType, _, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if err != nil || mediaType != "application/json" {
		return apperr.Invalid("Content-Type must be application/json.")
	}

	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()

	if err := decoder.Decode(target); err != nil {
		var maxBytesError *http.MaxBytesError
		if errors.As(err, &maxBytesError) {
			return apperr.New(apperr.KindValidation, "payload_too_large", "Request body is too large.")
		}
		return apperr.Invalid("Request body must be valid JSON.")
	}

	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return apperr.Invalid("Request body must contain a single JSON object.")
	}
	return nil
}

func writeError(w http.ResponseWriter, r *http.Request, err error, logger *slog.Logger) {
	status := http.StatusInternalServerError
	title := "Internal Server Error"
	code := "internal_error"
	detail := "An unexpected error occurred."

	var appError *apperr.Error
	if errors.As(err, &appError) {
		code = appError.Code
		detail = appError.Detail
		switch appError.Kind {
		case apperr.KindValidation:
			status = http.StatusBadRequest
			title = "Bad Request"
		case apperr.KindUnauthorized:
			status = http.StatusUnauthorized
			title = "Unauthorized"
		case apperr.KindForbidden:
			status = http.StatusForbidden
			title = "Forbidden"
		case apperr.KindNotFound:
			status = http.StatusNotFound
			title = "Not Found"
		case apperr.KindConflict:
			status = http.StatusConflict
			title = "Conflict"
		case apperr.KindRateLimited:
			status = http.StatusTooManyRequests
			title = "Too Many Requests"
		case apperr.KindInternal:
			logger.Error("request failed", "error", err, "request_id", requestIDFrom(r.Context()))
		}
	} else {
		logger.Error("request failed", "error", err, "request_id", requestIDFrom(r.Context()))
	}

	if status == http.StatusTooManyRequests {
		w.Header().Set("Retry-After", "60")
	}

	w.Header().Set("Content-Type", "application/problem+json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(problemBody{
		Type:      "about:blank",
		Title:     title,
		Status:    status,
		Code:      code,
		Detail:    detail,
		RequestID: requestIDFrom(r.Context()),
	})
}
