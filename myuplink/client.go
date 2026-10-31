package myuplink

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
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

		tokenMu sync.Mutex
		token   *AuthToken
	}
)

func NewClient(logger *slogger.Logger) *Client {
	c := Client{
		logger: logger,
		http:   resty.New(),
	}

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
	c.tokenMu.Lock()
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
		SetResult(result).
		Post("oauth/token")
	if err != nil {
		return err
	}

	if !resp.IsStatusSuccess() {
		return fmt.Errorf(`unable to login to myUplink, expected status code 200, got %v`, resp.Status())
	}

	if result.AccessToken == "" || result.ExpiresIn <= 0 {
		return fmt.Errorf("myUplink returned an invalid access token")
	}
	c.token = result.Process()

	return nil
}

func (c *Client) createRequest(ctx context.Context) (*resty.Request, error) {
	c.tokenMu.Lock()
	defer c.tokenMu.Unlock()
	if c.token == nil || c.token.IsExpired() {
		if err := c.connect(ctx); err != nil {
			return nil, fmt.Errorf("refresh myUplink token: %w", err)
		}
	}
	return c.http.R().SetContext(ctx).SetAuthToken(c.token.AccessToken), nil
}

func (c *Client) GetSystems(ctx context.Context) (*ResultSystems, error) {
	result := &ResultSystems{}
	for page := 1; ; page++ {
		req, err := c.createRequest(ctx)
		if err != nil {
			return nil, err
		}
		batch := &ResultSystems{}
		resp, err := req.SetResult(batch).Get(fmt.Sprintf("/v2/systems/me?page=%d&itemsPerPage=100", page))
		if err != nil {
			return nil, err
		}
		if !resp.IsStatusSuccess() {
			return nil, fmt.Errorf("unable to fetch systems, got %s", resp.Status())
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
		req, err := c.createRequest(ctx)
		if err != nil {
			return nil, err
		}
		batch := &ResultAlarms{}
		resp, err := req.SetResult(batch).Get(fmt.Sprintf("/v2/systems/%s/notifications/active?page=%d&itemsPerPage=100", systemID, page))
		if err != nil {
			return nil, err
		}
		if !resp.IsStatusSuccess() {
			return nil, fmt.Errorf("unable to fetch active alarms, got %s", resp.Status())
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

	req, err := c.createRequest(ctx)
	if err != nil {
		return nil, err
	}
	resp, err := req.SetResult(result).Get(fmt.Sprintf("/v2/devices/%s/points", deviceId))
	if err != nil {
		return nil, err
	}

	if !resp.IsStatusSuccess() {
		return nil, fmt.Errorf(`unable to fetch device points from myUplink, expected status code 200, got %v`, resp.Status())
	}

	return result, nil
}

func (c *Client) GetSystemDeviceFirmware(ctx context.Context, deviceId string) (*SystemDeviceFirmwareInfo, error) {
	result := &SystemDeviceFirmwareInfo{}

	c.logger.Debug(`fetch myUplink device firmware info`, slog.String("deviceID", deviceId))

	req, err := c.createRequest(ctx)
	if err != nil {
		return nil, err
	}
	resp, err := req.SetResult(result).Get(fmt.Sprintf("/v2/devices/%s/firmware-info", deviceId))
	if err != nil {
		return nil, err
	}

	if !resp.IsStatusSuccess() {
		return nil, fmt.Errorf(`unable to fetch device firmware info from myUplink, expected status code 200, got %v`, resp.Status())
	}

	return result, nil
}
