package gitlab

import (
	"bytes"
	"io"
	"net/http"
	"strings"
	"time"
)

type Config struct {
	BaseURL  string
	Token    string
	Timeout  time.Duration
	RetryMax int
}

type Client struct {
	baseURL  string
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
	return &Client{baseURL: strings.TrimRight(config.BaseURL, "/"), token: config.Token, http: &http.Client{Timeout: timeout}, retryMax: retryMax}
}

func (c *Client) newRequest(method, path string, body any) (*http.Request, error) {
	reader, contentType, err := encodeBody(body)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequest(method, c.baseURL+path, reader)
	if err != nil {
		return nil, err
	}
	if c.token != "" {
		req.Header.Set("PRIVATE-TOKEN", c.token)
	}
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}
	return req, nil
}

func (c *Client) do(req *http.Request, target any) error {
	var last error
	for attempt := 0; attempt <= c.retryMax; attempt++ {
		next, err := cloneRequest(req)
		if err != nil {
			return err
		}
		resp, err := c.http.Do(next)
		if err != nil {
			last = err
			if attempt < c.retryMax {
				continue
			}
			return err
		}
		if retryableStatus(resp.StatusCode) && attempt < c.retryMax {
			_, _ = io.Copy(io.Discard, resp.Body)
			_ = resp.Body.Close()
			last = mapHTTPError(resp.StatusCode, "gitlab request failed")
			continue
		}
		return decodeResponse(resp, target)
	}
	return last
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
