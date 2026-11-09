package profileapi

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"net/http"
	"net/url"
	"strings"

	"github.com/kmradished/GOsapiens/internal/domain"
)

var (
	ErrDependency       = errors.New("profile API failure")
	ErrResponseTooLarge = errors.New("profile response exceeds size limit")
)

type Client struct {
	baseURL      string
	httpClient   *http.Client
	maxBodyBytes int64
}

type profileResponse struct {
	ID     domain.UserID `json:"id"`
	Active bool          `json:"active"`
}

func NewClient(baseURL string, httpClient *http.Client, maxBodyBytes int64) (*Client, error) {
	base, err := url.Parse(baseURL)
	if err != nil || (base.Scheme != "http" && base.Scheme != "https") ||
		base.Hostname() == "" || base.User != nil || base.ForceQuery || base.RawQuery != "" ||
		base.Fragment != "" {
		return nil, errors.New("profile API requires an HTTP(S) base URL without credentials, query or fragment")
	}
	if httpClient == nil || httpClient.Timeout <= 0 {
		return nil, errors.New("profile API requires an HTTP client with a positive timeout")
	}
	if maxBodyBytes <= 0 || maxBodyBytes == math.MaxInt64 {
		return nil, errors.New("invalid profile API response limit")
	}

	return &Client{
		baseURL:      strings.TrimRight(base.String(), "/"),
		httpClient:   httpClient,
		maxBodyBytes: maxBodyBytes,
	}, nil
}

func (c *Client) GetContext(ctx context.Context, id domain.UserID) (domain.Profile, error) {
	escapedID := url.PathEscape(string(id))
	switch escapedID {
	case ".":
		escapedID = "%2E"
	case "..":
		escapedID = "%2E%2E"
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+"/profiles/"+escapedID, nil)
	if err != nil {
		return domain.Profile{}, fmt.Errorf("%w: create request: %w", ErrDependency, err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return domain.Profile{}, fmt.Errorf("%w: request: %w", ErrDependency, err)
	}
	defer resp.Body.Close()

	switch resp.StatusCode {
	case http.StatusOK:
	case http.StatusNotFound:
		return domain.Profile{}, fmt.Errorf("profile %q: %w", id, domain.ErrProfileNotFound)
	default:
		return domain.Profile{}, fmt.Errorf("%w: HTTP status %d", ErrDependency, resp.StatusCode)
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, c.maxBodyBytes+1))
	if err != nil {
		return domain.Profile{}, fmt.Errorf("%w: read response: %w", ErrDependency, err)
	}
	if int64(len(body)) > c.maxBodyBytes {
		return domain.Profile{}, fmt.Errorf("%w: %w", ErrDependency, ErrResponseTooLarge)
	}

	var payload struct {
		ID     domain.UserID `json:"id"`
		Active *bool         `json:"active"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		return domain.Profile{}, fmt.Errorf("%w: decode response: %w", ErrDependency, err)
	}
	if payload.ID == "" || payload.ID != id || payload.Active == nil {
		return domain.Profile{}, fmt.Errorf("%w: response must include the requested ID and active flag", ErrDependency)
	}

	return domain.Profile{ID: payload.ID, Active: *payload.Active}, nil
}
