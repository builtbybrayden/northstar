package health

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"time"
)

type SidecarClient struct {
	BaseURL string
	Secret  string
	HTTP    *http.Client
}

func NewSidecarClient(baseURL, secret string) *SidecarClient {
	return &SidecarClient{
		BaseURL: baseURL, Secret: secret,
		HTTP: &http.Client{Timeout: 30 * time.Second},
	}
}

type SidecarRecovery struct {
	Date   string  `json:"date"`
	Score  *int    `json:"score"`
	HRVms  float64 `json:"hrv_ms"`
	RHR    *int    `json:"rhr"`
}
type SidecarSleep struct {
	Date        string `json:"date"`
	DurationMin *int   `json:"duration_min"`
	Score       *int   `json:"score"`
	DebtMin     *int   `json:"debt_min"`
}
type SidecarStrain struct {
	Date   string  `json:"date"`
	Score  float64 `json:"score"`
	AvgHR  *int    `json:"avg_hr"`
	MaxHR  *int    `json:"max_hr"`
}

func (c *SidecarClient) Recovery(ctx context.Context, days int) ([]SidecarRecovery, error) {
	var out []SidecarRecovery
	return out, c.get(ctx, "/recovery", q("days", days), &out)
}
func (c *SidecarClient) Sleep(ctx context.Context, days int) ([]SidecarSleep, error) {
	var out []SidecarSleep
	return out, c.get(ctx, "/sleep", q("days", days), &out)
}
func (c *SidecarClient) Strain(ctx context.Context, days int) ([]SidecarStrain, error) {
	var out []SidecarStrain
	return out, c.get(ctx, "/strain", q("days", days), &out)
}

func q(k string, v int) url.Values {
	out := url.Values{}
	out.Set(k, strconv.Itoa(v))
	return out
}

func (c *SidecarClient) get(ctx context.Context, path string, query url.Values, dst any) error {
	u := c.BaseURL + path
	if len(query) > 0 {
		u += "?" + query.Encode()
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return err
	}
	if c.Secret != "" {
		req.Header.Set("X-Sidecar-Secret", c.Secret)
	}
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return fmt.Errorf("whoop sidecar %s: %w", path, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode/100 != 2 {
		return fmt.Errorf("whoop sidecar %s: status %d", path, resp.StatusCode)
	}
	return json.NewDecoder(resp.Body).Decode(dst)
}
