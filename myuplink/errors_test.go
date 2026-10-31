package myuplink

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"testing"
)

func TestAPIErrorParsing(t *testing.T) {
	for _, tc := range []struct {
		name, body     string
		wantCode       ErrorCode
		wantParseError error
	}{
		{"string code", `{"httpStatusCode":400,"errorCode":"Forbidden","timestamp":"2026-10-31T12:00:00Z","details":["explanation"],"data":{"private":"secret"}}`, "Forbidden", nil},
		{"numeric code", `{"errorCode":123,"extra":"ignored"}`, "123", nil},
		{"html", `<html>bad gateway</html>`, "", ErrInvalidErrorBody},
		{"empty", "", "", ErrInvalidErrorBody},
		{"null", "null", "", ErrInvalidErrorBody},
		{"wrong type", `{"errorCode":true}`, "", ErrInvalidErrorBody},
		{"oversized", strings.Repeat("x", maxErrorBodyBytes+100), "", ErrErrorBodyTooLarge},
	} {
		t.Run(tc.name, func(t *testing.T) {
			client := testClient(t, func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				if r.URL.Path == "/oauth/token" {
					fmt.Fprint(w, `{"access_token":"test","expires_in":3600}`)
					return
				}
				w.Header().Set("Retry-After", "30")
				w.WriteHeader(http.StatusForbidden)
				fmt.Fprint(w, tc.body)
			})
			_, err := client.GetSystems(context.Background())
			var apiErr *APIError
			if !errors.As(err, &apiErr) {
				t.Fatalf("expected APIError, got %T: %v", err, err)
			}
			if apiErr.HTTPStatusCode != 403 || apiErr.ErrorCode != tc.wantCode || apiErr.RetryAfter != "30" {
				t.Fatalf("unexpected error: %+v", apiErr)
			}
			if !errors.Is(err, tc.wantParseError) && tc.wantParseError != nil {
				t.Fatalf("expected %v: %v", tc.wantParseError, err)
			}
			if tc.wantParseError == nil && apiErr.ParseError != nil {
				t.Fatal(apiErr.ParseError)
			}
			if tc.name == "string code" {
				if apiErr.ReportedHTTPStatusCode != 400 || len(apiErr.Details) != 1 || apiErr.Data["private"] != "secret" {
					t.Fatal("diagnostic fields lost")
				}
			}
			if strings.Contains(err.Error(), "secret") || strings.Contains(apiErr.LogValue().String(), "secret") || strings.Contains(apiErr.LogValue().String(), "explanation") {
				t.Fatal("diagnostics leaked to default logging")
			}
		})
	}
}

func TestOAuthErrorParsing(t *testing.T) {
	client := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		fmt.Fprint(w, `{"error":"invalid_client","error_description":"private diagnostic","error_uri":"https://example.com/help"}`)
	})
	_, err := client.GetSystems(context.Background())
	var oauthErr *OAuthError
	if !errors.As(err, &oauthErr) {
		t.Fatalf("wrapped OAuthError missing: %v", err)
	}
	if oauthErr.HTTPStatusCode != 400 || oauthErr.Code != "invalid_client" || oauthErr.ErrorDescription != "private diagnostic" || oauthErr.ParseError != nil {
		t.Fatalf("unexpected OAuth error: %+v", oauthErr)
	}
	if strings.Contains(err.Error(), "private") || strings.Contains(oauthErr.LogValue().String(), "private") {
		t.Fatal("OAuth description leaked")
	}
}
