package myuplink

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strconv"

	resty "resty.dev/v3"
)

const maxErrorBodyBytes = 64 << 10

var (
	ErrErrorBodyTooLarge = errors.New("error response exceeds 64 KiB")
	ErrInvalidErrorBody  = errors.New("invalid error response JSON")
)

// ErrorCode accepts string and integer representations without assuming an
// undocumented enum encoding. Unknown codes remain available to callers.
type ErrorCode string

func (c *ErrorCode) UnmarshalJSON(body []byte) error {
	if bytes.Equal(body, []byte("null")) {
		return nil
	}
	var text string
	if json.Unmarshal(body, &text) == nil {
		*c = ErrorCode(text)
		return nil
	}
	var number json.Number
	if err := json.Unmarshal(body, &number); err != nil {
		return ErrInvalidErrorBody
	}
	if _, err := strconv.ParseInt(number.String(), 10, 64); err != nil {
		return ErrInvalidErrorBody
	}
	*c = ErrorCode(number.String())
	return nil
}

// APIError preserves diagnostic fields while HTTPStatusCode always comes from
// the actual response. Data and Details are intentionally excluded from default
// logging: upstream messages can contain sensitive request information.
type APIError struct {
	Operation              string
	HTTPStatusCode         int
	ReportedHTTPStatusCode int
	ErrorCode              ErrorCode
	Timestamp              string
	Details                []string
	Data                   map[string]string
	RetryAfter             string
	ParseError             error
}

func (e *APIError) Error() string {
	return fmt.Sprintf("myUplink %s: HTTP %d", e.Operation, e.HTTPStatusCode)
}
func (e *APIError) Unwrap() error { return e.ParseError }
func (e *APIError) LogValue() slog.Value {
	return slog.GroupValue(slog.String("operation", e.Operation), slog.Int("http_status_code", e.HTTPStatusCode),
		slog.String("error_code", string(e.ErrorCode)), slog.String("source_timestamp", e.Timestamp), slog.Bool("body_parsed", e.ParseError == nil))
}

// OAuthError follows the OAuth token endpoint's distinct error format.
// ErrorDescription and ErrorURI are retained but not logged automatically.
type OAuthError struct {
	HTTPStatusCode   int
	Code             string
	ErrorDescription string
	ErrorURI         string
	RetryAfter       string
	ParseError       error
}

func (e *OAuthError) Error() string { return fmt.Sprintf("myUplink token: HTTP %d", e.HTTPStatusCode) }
func (e *OAuthError) Unwrap() error { return e.ParseError }
func (e *OAuthError) LogValue() slog.Value {
	return slog.GroupValue(slog.String("operation", "token"), slog.Int("http_status_code", e.HTTPStatusCode),
		slog.String("oauth_error", e.Code), slog.Bool("body_parsed", e.ParseError == nil))
}

func decodeErrorBody(body []byte, target any) error {
	if len(body) > maxErrorBodyBytes {
		return ErrErrorBodyTooLarge
	}
	if len(bytes.TrimSpace(body)) == 0 {
		return ErrInvalidErrorBody
	}
	if err := json.Unmarshal(body, target); err != nil {
		return ErrInvalidErrorBody
	}
	return nil
}

func responseError(resp *resty.Response, operation string, oauth bool) error {
	if oauth {
		e := &OAuthError{HTTPStatusCode: resp.StatusCode(), RetryAfter: resp.Header().Get("Retry-After")}
		var body struct {
			Code        string `json:"error"`
			Description string `json:"error_description"`
			URI         string `json:"error_uri"`
		}
		e.ParseError = decodeErrorBody(resp.Bytes(), &body)
		if e.ParseError == nil && body.Code == "" {
			e.ParseError = ErrInvalidErrorBody
		}
		if e.ParseError == nil {
			e.Code, e.ErrorDescription, e.ErrorURI = body.Code, body.Description, body.URI
		}
		return e
	}
	e := &APIError{Operation: operation, HTTPStatusCode: resp.StatusCode(), RetryAfter: resp.Header().Get("Retry-After")}
	var body struct {
		HTTPStatusCode int               `json:"httpStatusCode"`
		ErrorCode      ErrorCode         `json:"errorCode"`
		Timestamp      string            `json:"timestamp"`
		Details        []string          `json:"details"`
		Data           map[string]string `json:"data"`
	}
	e.ParseError = decodeErrorBody(resp.Bytes(), &body)
	if e.ParseError == nil && body.HTTPStatusCode == 0 && body.ErrorCode == "" && body.Timestamp == "" && body.Details == nil && body.Data == nil {
		e.ParseError = ErrInvalidErrorBody
	}
	if e.ParseError == nil {
		e.ReportedHTTPStatusCode, e.ErrorCode, e.Timestamp = body.HTTPStatusCode, body.ErrorCode, body.Timestamp
		e.Details, e.Data = body.Details, body.Data
	}
	return e
}

// Bound error bodies before Resty buffers them. Successful point responses keep
// their existing size behavior. Closing the original body prevents an oversized
// error from being drained into memory. Reading one extra byte detects overflow.
type errorBodyTransport struct{ base http.RoundTripper }

func (t errorBodyTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	resp, err := t.base.RoundTrip(req)
	if err != nil || resp.StatusCode >= 200 && resp.StatusCode < 300 {
		return resp, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, maxErrorBodyBytes+1))
	if err != nil {
		// Keep HTTP status available even when the error body cannot be read.
		body = nil
	}
	resp.Body = io.NopCloser(bytes.NewReader(body))
	resp.ContentLength = int64(len(body))
	return resp, nil
}
