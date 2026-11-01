package myuplink

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/webdevops/go-common/log/slogger"
	resty "resty.dev/v3"
)

type (
	Client struct {
		logger *slogger.Logger
		http   *resty.Client

		clientId     string
		clientSecret string

		gate    *requestGate
		tokenMu contextMutex
		token   *AuthToken
	}
)

func NewClient(logger *slogger.Logger) *Client {
	c := Client{
		logger:  logger,
		http:    resty.New(),
		tokenMu: make(contextMutex, 1),
	}

	c.gate = &requestGate{base: errorBodyTransport{base: c.http.Transport()}, mu: make(contextMutex, 1)}
	c.http.SetTransport(c.gate)
	c.http.SetHeader("Accept", "application/json")
	c.http.SetRetryCount(5)
	c.http.SetRetryWaitTime(5 * time.Second)
	c.http.SetRetryMaxWaitTime(30 * time.Second)
	c.http.SetRetryDefaultConditions(true)

	return &c
}

func (c *Client) SetDebugMode(val bool) {
	c.http.SetDebug(val)
}

func (c *Client) SetApiUrl(val string) {
	c.http.SetBaseURL(val)
}

func (c *Client) SetUserAgent(val string) {
	c.http.SetHeader("User-Agent", val)
}

func (c *Client) SetAuth(clientId, clientSecret string) {
	c.clientId = clientId
	c.clientSecret = clientSecret
}

func (c *Client) Connect(ctx context.Context) error {
	if err := c.tokenMu.Lock(ctx); err != nil {
		return err
	}
	defer c.tokenMu.Unlock()
	return c.connect(ctx)
}

// connect is called with tokenMu held.
func (c *Client) connect(ctx context.Context) error {
	c.logger.Debug(`refresh myUplink auth token`)

	payload := map[string]string{
		"grant_type": "client_credentials",
		"scope":      "READSYSTEM",
	}

	result := &AuthToken{}

	resp, err := c.http.R().
		SetContext(ctx).
		SetHeader("Content-Type", "application/x-www-form-urlencoded").
		SetFormData(payload).
		SetBasicAuth(c.clientId, c.clientSecret).
		Post("oauth/token")
	if err != nil {
		return err
	}

	if !resp.IsStatusSuccess() {
		return responseError(resp, "token", true)
	}
	if err := decodeSuccess(resp, result, "token"); err != nil {
		return err
	}

	if result.AccessToken == "" || result.ExpiresIn <= 0 {
		return fmt.Errorf("myUplink returned an invalid access token")
	}
	c.token = result.Process()

	return nil
}

func (c *Client) createRequest(ctx context.Context) (*resty.Request, error) {
	if err := c.tokenMu.Lock(ctx); err != nil {
		return nil, err
	}
	defer c.tokenMu.Unlock()
	if c.token == nil || c.token.IsExpired() {
		if err := c.connect(ctx); err != nil {
			return nil, fmt.Errorf("refresh myUplink token: %w", err)
		}
	}
	return c.http.R().SetContext(ctx).SetAuthToken(c.token.AccessToken), nil
}

// getJSON retries a rejected token once; concurrent rejections of an older
// token must not invalidate a replacement another request has already obtained.
func (c *Client) getJSON(ctx context.Context, path, operation string, result any) error {
	for attempt := 0; ; attempt++ {
		req, err := c.createRequest(ctx)
		if err != nil {
			return err
		}
		resp, err := req.Get(path)
		if err != nil {
			return err
		}
		if resp.StatusCode() == http.StatusUnauthorized {
			if err := c.tokenMu.Lock(ctx); err != nil {
				return err
			}
			if c.token != nil && c.token.AccessToken == req.AuthToken {
				c.token = nil
			}
			c.tokenMu.Unlock()
			if attempt == 0 {
				continue
			}
		}
		if !resp.IsStatusSuccess() {
			return responseError(resp, operation, false)
		}
		return decodeSuccess(resp, result, operation)
	}
}

func decodeSuccess(resp *resty.Response, result any, operation string) error {
	// Decode explicitly: Resty otherwise silently skips JSON returned as text/plain.
	body := resp.Bytes()
	if bytes.Equal(bytes.TrimSpace(body), []byte("null")) || json.Unmarshal(body, result) != nil {
		// JSON parser errors may contain upstream data; do not log their text.
		return fmt.Errorf("myUplink %s: invalid JSON response", operation)
	}
	return nil
}

func (c *Client) GetSystems(ctx context.Context) (*ResultSystems, error) {
	result := &ResultSystems{}
	for page := 1; ; page++ {
		batch := &ResultSystems{}
		if err := c.getJSON(ctx, fmt.Sprintf("/v2/systems/me?page=%d&itemsPerPage=100", page), "systems", batch); err != nil {
			return nil, err
		}
		result.Systems = append(result.Systems, batch.Systems...)
		result.NumItems = batch.NumItems
		size := batch.ItemsPerPage
		if size <= 0 {
			size = 100
		}
		if len(batch.Systems) == 0 || len(result.Systems) >= batch.NumItems || len(batch.Systems) < size {
			return result, nil
		}
	}
}

// GetSystemActiveAlarms retrieves every page of active notifications.
func (c *Client) GetSystemActiveAlarms(ctx context.Context, systemID string) ([]Alarm, error) {
	var alarms []Alarm
	for page := 1; ; page++ {
		batch := &ResultAlarms{}
		if err := c.getJSON(ctx, fmt.Sprintf("/v2/systems/%s/notifications/active?page=%d&itemsPerPage=100", systemID, page), "active_alarms", batch); err != nil {
			return nil, err
		}
		alarms = append(alarms, batch.Notifications...)
		size := batch.ItemsPerPage
		if size <= 0 {
			size = 100
		}
		if len(batch.Notifications) == 0 || len(alarms) >= batch.NumItems || len(batch.Notifications) < size {
			return alarms, nil
		}
	}
}

func (c *Client) GetSystemDevicePoints(ctx context.Context, deviceId string) (*SystemDevicePoints, error) {
	result := &SystemDevicePoints{}

	c.logger.Debug(`fetch myUplink device points`, slog.String("deviceID", deviceId))

	if err := c.getJSON(ctx, fmt.Sprintf("/v2/devices/%s/points", deviceId), "device_points", result); err != nil {
		return nil, err
	}

	return result, nil
}

func (c *Client) GetSystemDeviceFirmware(ctx context.Context, deviceId string) (*SystemDeviceFirmwareInfo, error) {
	result := &SystemDeviceFirmwareInfo{}

	c.logger.Debug(`fetch myUplink device firmware info`, slog.String("deviceID", deviceId))

	if err := c.getJSON(ctx, fmt.Sprintf("/v2/devices/%s/firmware-info", deviceId), "firmware_info", result); err != nil {
		return nil, err
	}

	return result, nil
}
