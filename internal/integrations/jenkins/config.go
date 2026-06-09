package jenkins

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/shareinto/paas/internal/shared"
)

type Config struct {
	BaseURL  string
	Username string
	Token    string
	Timeout  time.Duration
	RetryMax int
}

type Client struct {
	baseURL  string
	username string
	token    string
	http     *http.Client
	retryMax int
}

func NewClient(config Config) *Client {
	timeout := config.Timeout
	if timeout == 0 {
		timeout = 10 * time.Second
	}
	retryMax := config.RetryMax
	if retryMax == 0 {
		retryMax = 2
	}
	return &Client{baseURL: strings.TrimRight(config.BaseURL, "/"), username: config.Username, token: config.Token, http: &http.Client{Timeout: timeout}, retryMax: retryMax}
}

func (c *Client) do(req *http.Request, target any) (*http.Response, error) {
	var last error
	for attempt := 0; attempt <= c.retryMax; attempt++ {
		next, err := cloneRequest(req)
		if err != nil {
			return nil, err
		}
		resp, err := c.http.Do(next)
		if err != nil {
			last = err
			if attempt < c.retryMax {
				continue
			}
			return nil, err
		}
		if retryableStatus(resp.StatusCode) && attempt < c.retryMax {
			_, _ = io.Copy(io.Discard, resp.Body)
			_ = resp.Body.Close()
			last = shared.NewError(shared.CodeUnavailable, "jenkins request failed")
			continue
		}
		if resp.StatusCode >= 400 {
			if resp.StatusCode == http.StatusBadRequest && strings.Contains(strings.ToLower(resp.Header.Get("X-Error")), "already exists") {
				_, _ = io.Copy(io.Discard, resp.Body)
				_ = resp.Body.Close()
				return nil, shared.NewError(shared.CodeConflict, "jenkins item already exists")
			}
			_ = resp.Body.Close()
			return nil, shared.NewError(mapStatus(resp.StatusCode), "jenkins request failed")
		}
		if target == nil {
			return resp, nil
		}
		defer resp.Body.Close()
		return resp, json.NewDecoder(resp.Body).Decode(target)
	}
	return nil, last
}

func cloneRequest(req *http.Request) (*http.Request, error) {
	next := req.Clone(req.Context())
	if req.Body == nil || req.GetBody != nil {
		if req.GetBody != nil {
			body, err := req.GetBody()
			if err != nil {
				return nil, err
			}
			next.Body = body
		}
		return next, nil
	}
	data, err := io.ReadAll(req.Body)
	if err != nil {
		return nil, err
	}
	_ = req.Body.Close()
	req.Body = io.NopCloser(bytes.NewReader(data))
	req.GetBody = func() (io.ReadCloser, error) {
		return io.NopCloser(bytes.NewReader(data)), nil
	}
	next.Body = io.NopCloser(bytes.NewReader(data))
	return next, nil
}

func retryableStatus(status int) bool {
	return status == http.StatusTooManyRequests || status == http.StatusBadGateway || status == http.StatusServiceUnavailable || status == http.StatusGatewayTimeout
}

func mapStatus(status int) shared.ErrorCode {
	switch status {
	case http.StatusUnauthorized:
		return shared.CodeUnauthenticated
	case http.StatusForbidden:
		return shared.CodePermissionDenied
	case http.StatusNotFound:
		return shared.CodeNotFound
	case http.StatusConflict:
		return shared.CodeConflict
	case http.StatusTooManyRequests, http.StatusBadGateway, http.StatusServiceUnavailable, http.StatusGatewayTimeout:
		return shared.CodeUnavailable
	default:
		return shared.CodeInternal
	}
}
