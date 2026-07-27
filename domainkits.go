package domainkits

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"time"
)

const (
	DefaultBaseURL = "https://premium-api.domainkits.com/api/v1"
	MaxLimit       = 500
	userAgent      = "domainkits-go/0.1.0"
)

type Params map[string]string

type RateLimit struct {
	Limit     int
	Remaining int
	ResetAt   time.Time
}

type APIError struct {
	Status    int
	Message   string
	RateLimit RateLimit
}

func (e *APIError) Error() string {
	return e.Message
}

func (e *APIError) IsRateLimit() bool {
	return e.Status == 429
}

func (e *APIError) IsAuth() bool {
	return e.Status == 401 || e.Status == 403
}

func (e *APIError) RetryAfter() time.Duration {
	if e.RateLimit.ResetAt.IsZero() {
		return 0
	}
	d := time.Until(e.RateLimit.ResetAt)
	if d < 0 {
		return 0
	}
	return d
}

type Domain struct {
	Domain         string `json:"domain"`
	RegisteredDate string `json:"registered_date,omitempty"`
	ExpiryDate     string `json:"expiry_date,omitempty"`
	Age            int    `json:"age,omitempty"`
	Status         string `json:"status,omitempty"`
	Marketplace    string `json:"marketplace,omitempty"`
	TLD            string `json:"tld,omitempty"`
	TLDCount       int    `json:"tld_count,omitempty"`
}

type SearchResult struct {
	Data  []Domain
	Total int
}

type ListResult struct {
	Data  []map[string]any
	Total int
}

type Client struct {
	APIKey     string
	BaseURL    string
	HTTPClient *http.Client
	MaxRetries int
}

func New(apiKey string) *Client {
	return &Client{
		APIKey:     apiKey,
		BaseURL:    DefaultBaseURL,
		HTTPClient: &http.Client{Timeout: 60 * time.Second},
		MaxRetries: 2,
	}
}

type envelope struct {
	Success *bool           `json:"success"`
	Error   string          `json:"error"`
	Data    json.RawMessage `json:"data"`
	Total   *int            `json:"total"`
}

func readRateLimit(h http.Header) RateLimit {
	num := func(name string) int {
		v, err := strconv.Atoi(h.Get(name))
		if err != nil {
			return 0
		}
		return v
	}
	rl := RateLimit{Limit: num("x-ratelimit-limit"), Remaining: num("x-ratelimit-remaining")}
	if reset := num("x-ratelimit-reset"); reset > 0 {
		rl.ResetAt = time.Unix(int64(reset), 0)
	}
	return rl
}

func (c *Client) RequestRaw(ctx context.Context, path string, params Params) ([]byte, http.Header, error) {
	if c.APIKey == "" {
		return nil, nil, errors.New("api key is required")
	}
	query := url.Values{}
	for k, v := range params {
		if v != "" {
			query.Set(k, v)
		}
	}
	target := c.BaseURL + path
	if len(query) > 0 {
		target += "?" + query.Encode()
	}

	var lastErr error
	for attempt := 0; attempt <= c.MaxRetries; attempt++ {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, target, nil)
		if err != nil {
			return nil, nil, err
		}
		req.Header.Set("Authorization", "Bearer "+c.APIKey)
		req.Header.Set("Accept", "application/json")
		req.Header.Set("User-Agent", userAgent)

		resp, err := c.HTTPClient.Do(req)
		if err != nil {
			return nil, nil, err
		}
		body, err := io.ReadAll(resp.Body)
		resp.Body.Close()
		if err != nil {
			return nil, nil, err
		}

		if resp.StatusCode < 400 {
			return body, resp.Header, nil
		}

		rl := readRateLimit(resp.Header)
		message := fmt.Sprintf("request failed with status %d", resp.StatusCode)
		var env envelope
		if json.Unmarshal(body, &env) == nil && env.Error != "" {
			message = env.Error
		}
		apiErr := &APIError{Status: resp.StatusCode, Message: message, RateLimit: rl}

		if apiErr.IsAuth() {
			return nil, nil, apiErr
		}
		if resp.StatusCode == 429 && attempt < c.MaxRetries {
			wait := apiErr.RetryAfter()
			if wait == 0 {
				wait = time.Second * time.Duration(1<<attempt)
			}
			if wait <= 2*time.Minute {
				lastErr = apiErr
				select {
				case <-ctx.Done():
					return nil, nil, ctx.Err()
				case <-time.After(wait):
				}
				continue
			}
		}
		if resp.StatusCode >= 500 && attempt < c.MaxRetries {
			lastErr = apiErr
			select {
			case <-ctx.Done():
				return nil, nil, ctx.Err()
			case <-time.After(500 * time.Millisecond * time.Duration(1<<attempt)):
			}
			continue
		}
		return nil, nil, apiErr
	}
	return nil, nil, lastErr
}

func (c *Client) request(ctx context.Context, path string, params Params) (*envelope, error) {
	body, header, err := c.RequestRaw(ctx, path, params)
	if err != nil {
		return nil, err
	}
	var env envelope
	if err := json.Unmarshal(body, &env); err != nil {
		return nil, err
	}
	if env.Success != nil && !*env.Success {
		message := env.Error
		if message == "" {
			message = "domainkits api returned an error"
		}
		return nil, &APIError{Status: 200, Message: message, RateLimit: readRateLimit(header)}
	}
	return &env, nil
}

func (c *Client) Object(ctx context.Context, path string, params Params) (map[string]any, error) {
	env, err := c.request(ctx, path, params)
	if err != nil {
		return nil, err
	}
	out := map[string]any{}
	if len(env.Data) > 0 && string(env.Data) != "null" {
		if err := json.Unmarshal(env.Data, &out); err != nil {
			var arr any
			if err2 := json.Unmarshal(env.Data, &arr); err2 == nil {
				return map[string]any{"data": arr}, nil
			}
			return nil, err
		}
	}
	return out, nil
}

func (c *Client) search(ctx context.Context, resource string, params Params) (*SearchResult, error) {
	env, err := c.request(ctx, "/search/"+resource, params)
	if err != nil {
		return nil, err
	}
	result := &SearchResult{}
	if len(env.Data) > 0 && string(env.Data) != "null" {
		if err := json.Unmarshal(env.Data, &result.Data); err != nil {
			return nil, err
		}
	}
	if env.Total != nil {
		result.Total = *env.Total
	} else {
		result.Total = len(result.Data)
	}
	return result, nil
}

func (c *Client) list(ctx context.Context, path string, params Params) (*ListResult, error) {
	env, err := c.request(ctx, path, params)
	if err != nil {
		return nil, err
	}
	result := &ListResult{}
	if len(env.Data) > 0 && string(env.Data) != "null" {
		if err := json.Unmarshal(env.Data, &result.Data); err != nil {
			return nil, err
		}
	}
	if env.Total != nil {
		result.Total = *env.Total
	} else {
		result.Total = len(result.Data)
	}
	return result, nil
}

func (c *Client) NRDs(ctx context.Context, params Params) (*SearchResult, error) {
	return c.search(ctx, "nrds", params)
}

func (c *Client) Expired(ctx context.Context, params Params) (*SearchResult, error) {
	return c.search(ctx, "expired", params)
}

func (c *Client) Aged(ctx context.Context, params Params) (*SearchResult, error) {
	return c.search(ctx, "aged", params)
}

func (c *Client) Active(ctx context.Context, params Params) (*SearchResult, error) {
	return c.search(ctx, "active", params)
}

func (c *Client) Deleted(ctx context.Context, params Params) (*SearchResult, error) {
	return c.search(ctx, "deleted", params)
}

func (c *Client) Market(ctx context.Context, params Params) (*SearchResult, error) {
	return c.search(ctx, "market", params)
}

func (c *Client) Paginate(ctx context.Context, resource string, params Params, fn func(Domain) bool) error {
	offset := 0
	if v, ok := params["offset"]; ok && v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			offset = n
		}
	}
	limit := MaxLimit
	for {
		page := Params{}
		for k, v := range params {
			page[k] = v
		}
		page["limit"] = strconv.Itoa(limit)
		page["offset"] = strconv.Itoa(offset)
		result, err := c.search(ctx, resource, page)
		if err != nil {
			return err
		}
		for _, d := range result.Data {
			if !fn(d) {
				return nil
			}
		}
		offset += len(result.Data)
		if len(result.Data) == 0 || len(result.Data) < limit || offset >= result.Total {
			return nil
		}
	}
}

func (c *Client) Export(ctx context.Context, resource string, params Params) ([]byte, error) {
	page := Params{}
	for k, v := range params {
		page[k] = v
	}
	page["export"] = "csv"
	body, _, err := c.RequestRaw(ctx, "/search/"+resource, page)
	return body, err
}

func (c *Client) Whois(ctx context.Context, domain string) (map[string]any, error) {
	return c.Object(ctx, "/whois", Params{"domain": domain})
}

func (c *Client) DNS(ctx context.Context, domain string) (map[string]any, error) {
	return c.Object(ctx, "/dns", Params{"domain": domain})
}

func (c *Client) Safety(ctx context.Context, domain string) (map[string]any, error) {
	return c.Object(ctx, "/safety", Params{"domain": domain})
}

func (c *Client) IPLookup(ctx context.Context, query string) (map[string]any, error) {
	return c.Object(ctx, "/ip-lookup", Params{"query": query})
}

func (c *Client) Registrar(ctx context.Context, query string) (map[string]any, error) {
	return c.Object(ctx, "/registrar", Params{"query": query})
}

func (c *Client) StatusGuide(ctx context.Context, query string) (map[string]any, error) {
	return c.Object(ctx, "/status-guide", Params{"query": query})
}

func (c *Client) TLDCheck(ctx context.Context, prefix string, params Params) (map[string]any, error) {
	merged := Params{"prefix": prefix}
	for k, v := range params {
		merged[k] = v
	}
	return c.Object(ctx, "/tld-check", merged)
}

func (c *Client) Typosquat(ctx context.Context, domain string, params Params) (*ListResult, error) {
	merged := Params{"domain": domain}
	for k, v := range params {
		merged[k] = v
	}
	return c.list(ctx, "/typosquat", merged)
}

func (c *Client) NSReverse(ctx context.Context, ns string, params Params) ([]string, int, error) {
	merged := Params{"ns": ns}
	for k, v := range params {
		merged[k] = v
	}
	env, err := c.request(ctx, "/ns-reverse", merged)
	if err != nil {
		return nil, 0, err
	}
	var data []string
	if len(env.Data) > 0 && string(env.Data) != "null" {
		if err := json.Unmarshal(env.Data, &data); err != nil {
			return nil, 0, err
		}
	}
	total := len(data)
	if env.Total != nil {
		total = *env.Total
	}
	return data, total, nil
}

func (c *Client) MonitorChanges(ctx context.Context, params Params) (*ListResult, error) {
	return c.list(ctx, "/monitor/changes", params)
}

func (c *Client) CTSubdomains(ctx context.Context, domain string, params Params) (*ListResult, error) {
	merged := Params{"domain": domain}
	for k, v := range params {
		merged[k] = v
	}
	return c.list(ctx, "/ct/subdomains", merged)
}

func (c *Client) CTCerts(ctx context.Context, params Params) (*ListResult, error) {
	return c.list(ctx, "/ct/certs", params)
}

func (c *Client) CTSearch(ctx context.Context, keyword string, params Params) (*ListResult, error) {
	merged := Params{"keyword": keyword}
	for k, v := range params {
		merged[k] = v
	}
	return c.list(ctx, "/ct/search", merged)
}

func (c *Client) TLDTrends(ctx context.Context, trendType string, params Params) ([]map[string]any, error) {
	result, err := c.list(ctx, "/trends/tlds/"+trendType, params)
	if err != nil {
		return nil, err
	}
	return result.Data, nil
}

func (c *Client) KeywordTrends(ctx context.Context, trendType string, params Params) ([]map[string]any, error) {
	result, err := c.list(ctx, "/trends/keywords/"+trendType, params)
	if err != nil {
		return nil, err
	}
	return result.Data, nil
}

func (c *Client) Usage(ctx context.Context) (map[string]any, error) {
	return c.Object(ctx, "/usage", nil)
}

func (c *Client) SearchStatus(ctx context.Context) (map[string]any, error) {
	return c.Object(ctx, "/search/status", nil)
}

func (c *Client) Health(ctx context.Context) (map[string]any, error) {
	body, _, err := c.RequestRaw(ctx, "/health", nil)
	if err != nil {
		return nil, err
	}
	out := map[string]any{}
	if err := json.Unmarshal(body, &out); err != nil {
		return nil, err
	}
	return out, nil
}

func (c *Client) NRDsDownload(ctx context.Context, params Params) ([]byte, error) {
	body, _, err := c.RequestRaw(ctx, "/nrds/download", params)
	return body, err
}
