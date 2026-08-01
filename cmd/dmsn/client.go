package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/thomas-elliott/damson/internal/axi"
)

// maxErrorBodyBytes bounds how much of a failing response we read looking for
// the API's {"error": …} message.
const maxErrorBodyBytes = 64 << 10

// Client is a thin HTTP client for the damson API. It attaches the bearer,
// bounds the request, and maps every non-2xx onto the structured error
// envelope so no caller has to interpret a status code.
type Client struct {
	BaseURL string
	Token   string
	HTTP    *http.Client
}

func NewClient(baseURL, token string, timeout time.Duration) (*Client, error) {
	base := strings.TrimRight(strings.TrimSpace(baseURL), "/")
	if base == "" {
		return nil, axi.Usagef(
			[]string{"dmsn login --server <url> --token <token>"},
			"no server configured; pass --server or set DAMSON_SERVER")
	}
	u, err := url.Parse(base)
	if err != nil || u.Scheme == "" || u.Host == "" {
		return nil, axi.Usagef(
			[]string{"dmsn login --server https://damson.example.com --token <token>"},
			"%q is not a valid server URL", baseURL)
	}
	if timeout <= 0 {
		timeout = defaultTimeout
	}
	return &Client{BaseURL: base, Token: token, HTTP: &http.Client{Timeout: timeout}}, nil
}

// GetJSON performs a GET and decodes the body into out.
func (c *Client) GetJSON(ctx context.Context, path string, query url.Values, out any) error {
	return c.do(ctx, http.MethodGet, path, query, nil, "", out)
}

// PostJSON performs a POST with a JSON body and decodes the response.
func (c *Client) PostJSON(ctx context.Context, path string, body, out any) error {
	return c.sendJSON(ctx, http.MethodPost, path, body, out)
}

// PatchJSON performs a PATCH with a JSON body and decodes the response.
func (c *Client) PatchJSON(ctx context.Context, path string, body, out any) error {
	return c.sendJSON(ctx, http.MethodPatch, path, body, out)
}

// Delete performs a DELETE, discarding any response body.
func (c *Client) Delete(ctx context.Context, path string) error {
	return c.do(ctx, http.MethodDelete, path, nil, nil, "", nil)
}

func (c *Client) sendJSON(ctx context.Context, method, path string, body, out any) error {
	var buf bytes.Buffer
	if body != nil {
		if err := json.NewEncoder(&buf).Encode(body); err != nil {
			return axi.Wrap(axi.CodeIO, err, "encode request body")
		}
	}
	return c.do(ctx, method, path, nil, &buf, "application/json", out)
}

func (c *Client) do(ctx context.Context, method, path string, query url.Values, body io.Reader, contentType string, out any) error {
	target := c.BaseURL + path
	if len(query) > 0 {
		target += "?" + query.Encode()
	}
	req, err := http.NewRequestWithContext(ctx, method, target, body)
	if err != nil {
		return axi.Wrap(axi.CodeUsage, err, "build request for %s", target)
	}
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}
	c.authorise(req)

	resp, err := c.HTTP.Do(req)
	if err != nil {
		return networkError(err, c.BaseURL)
	}
	defer resp.Body.Close()

	return finish(resp, out)
}

// finish maps a response onto the error envelope and decodes the body. Every
// request path ends here, including the multipart upload, so status handling
// cannot drift between them.
func finish(resp *http.Response, out any) error {
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return axi.FromHTTP(resp.StatusCode, serverMessage(resp.Body))
	}
	if out == nil || resp.StatusCode == http.StatusNoContent {
		_, _ = io.Copy(io.Discard, resp.Body)
		return nil
	}
	return decodeJSON(resp.Body, out)
}

func decodeJSON(r io.Reader, out any) error {
	if err := json.NewDecoder(r).Decode(out); err != nil {
		return axi.Wrap(axi.CodeServer, err, "the server returned a response %s could not read", binaryName)
	}
	return nil
}

// authorise attaches the bearer. Kept separate so it is impossible to build a
// request path that forgets it.
func (c *Client) authorise(req *http.Request) {
	if c.Token != "" {
		req.Header.Set("Authorization", "Bearer "+c.Token)
	}
}

// serverMessage extracts the API's {"error": …} string. A body that isn't
// that shape yields "", and the caller falls back to the status text.
func serverMessage(r io.Reader) string {
	b, err := io.ReadAll(io.LimitReader(r, maxErrorBodyBytes))
	if err != nil || len(b) == 0 {
		return ""
	}
	var envelope struct {
		Error string `json:"error"`
	}
	if err := json.Unmarshal(b, &envelope); err == nil && envelope.Error != "" {
		return envelope.Error
	}
	return ""
}

// networkError reports a transport failure without echoing the URL's query
// string, which can carry search terms the user typed.
func networkError(err error, base string) error {
	e := axi.Wrap(axi.CodeNetwork, err, "cannot reach %s: %v", base, err)
	e.Help = []string{
		fmt.Sprintf("%s whoami", binaryName),
		fmt.Sprintf("%s login --server <url> --token <token>", binaryName),
	}
	return e
}
