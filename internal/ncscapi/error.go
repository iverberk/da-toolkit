package ncscapi

import (
	"fmt"
	"net/http"
	"strings"

	"da-toolkit/internal/ncscapi/generated"
)

type APIError struct {
	Operation  string
	StatusCode int
	Message    string
	TraceID    string
	Details    []string
}

func (e *APIError) Error() string {
	parts := []string{e.Operation}
	if e.StatusCode != 0 {
		parts = append(parts, fmt.Sprintf("status %d", e.StatusCode))
	}
	if e.Message != "" {
		parts = append(parts, e.Message)
	}
	if e.TraceID != "" {
		parts = append(parts, "trace_id="+e.TraceID)
	}
	if len(e.Details) > 0 {
		parts = append(parts, strings.Join(e.Details, "; "))
	}
	return strings.Join(parts, ": ")
}

func normalizeAPIError(operation string, statusCode int, resp *http.Response, _ []byte, validation *generated.RequestValidationErrorResponse) error {
	apiErr := &APIError{
		Operation:  operation,
		StatusCode: statusCode,
		Message:    http.StatusText(statusCode),
	}
	if apiErr.Message == "" && resp != nil {
		apiErr.Message = resp.Status
	}
	if resp != nil {
		apiErr.TraceID = firstNonEmpty(resp.Header.Get("Trace-Id"), resp.Header.Get("X-Trace-Id"), resp.Header.Get("X-Request-Id"))
	}
	if validation != nil {
		if validation.Message != nil && *validation.Message != "" {
			apiErr.Message = *validation.Message
		}
		if validation.TraceId != nil {
			apiErr.TraceID = validation.TraceId.String()
		}
		for _, detail := range validation.Detail {
			if detail.Msg != "" {
				apiErr.Details = append(apiErr.Details, detail.Msg)
			}
		}
	}
	return apiErr
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}
