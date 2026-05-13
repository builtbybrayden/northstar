package finance

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"
)

func newJSONReader(b []byte) io.Reader { return bytes.NewReader(b) }

// SidecarClient talks to tools/actual-sidecar over localhost HTTP.
type SidecarClient struct {
	BaseURL string
	Secret  string
	HTTP    *http.Client
}

func NewSidecarClient(baseURL, secret string) *SidecarClient {
	return &SidecarClient{
		BaseURL: baseURL,
		Secret:  secret,
		HTTP:    &http.Client{Timeout: 30 * time.Second},
	}
}

type SidecarAccount struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	OffBudget bool   `json:"offbudget"`
	Closed    bool   `json:"closed"`
	Balance   int64  `json:"balance"` // cents
}

type SidecarCategory struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	GroupID  string `json:"group_id"`
	IsIncome bool   `json:"is_income"`
}

type SidecarTransaction struct {
	ID       string `json:"id"`
	Account  string `json:"account"`
	Date     string `json:"date"`
	Payee    string `json:"payee"`
	Category string `json:"category"` // Actual category id (may be empty)
	Amount   int64  `json:"amount"`   // cents, negative = outflow
	Notes    string `json:"notes"`
}

type SidecarBudgets struct {
	Month      string `json:"month"`
	Categories []struct {
		ID       string `json:"id"`
		Budgeted int64  `json:"budgeted"` // cents
	} `json:"categories"`
}

type SidecarHealth struct {
	OK      bool   `json:"ok"`
	Mode    string `json:"mode"`
	Version string `json:"version"`
}

// InitParams are the credentials forwarded to the sidecar's POST /init route.
// In mock mode init is a no-op; in actual mode the sidecar uses these to
// connect to an Actual server and download the budget into its local cache.
type InitParams struct {
	ServerURL          string `json:"serverURL"`
	Password           string `json:"password"`
	SyncID             string `json:"syncId"`
	EncryptionPassword string `json:"encryptionPassword,omitempty"`
	DataDir            string `json:"dataDir,omitempty"`
}

func (c *SidecarClient) Health(ctx context.Context) (*SidecarHealth, error) {
	var out SidecarHealth
	return &out, c.get(ctx, "/health", nil, &out)
}

// Init forwards Actual credentials to the sidecar. Must complete successfully
// before Accounts/Transactions/etc. work in real mode.
func (c *SidecarClient) Init(ctx context.Context, p InitParams) error {
	body, err := json.Marshal(p)
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.BaseURL+"/init",
		newJSONReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	if c.Secret != "" {
		req.Header.Set("X-Sidecar-Secret", c.Secret)
	}
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return fmt.Errorf("sidecar /init: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode/100 != 2 {
		return fmt.Errorf("sidecar /init: status %d", resp.StatusCode)
	}
	return nil
}

func (c *SidecarClient) Accounts(ctx context.Context) ([]SidecarAccount, error) {
	var out []SidecarAccount
	return out, c.get(ctx, "/accounts", nil, &out)
}

func (c *SidecarClient) Categories(ctx context.Context) ([]SidecarCategory, error) {
	var out []SidecarCategory
	return out, c.get(ctx, "/categories", nil, &out)
}

func (c *SidecarClient) Transactions(ctx context.Context, since string) ([]SidecarTransaction, error) {
	var out []SidecarTransaction
	q := url.Values{}
	if since != "" {
		q.Set("since", since)
	}
	return out, c.get(ctx, "/transactions", q, &out)
}

func (c *SidecarClient) Budgets(ctx context.Context, month string) (*SidecarBudgets, error) {
	var out SidecarBudgets
	q := url.Values{}
	if month != "" {
		q.Set("month", month)
	}
	return &out, c.get(ctx, "/budgets", q, &out)
}

func (c *SidecarClient) get(ctx context.Context, path string, q url.Values, dst any) error {
	u := c.BaseURL + path
	if len(q) > 0 {
		u += "?" + q.Encode()
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
		return fmt.Errorf("sidecar %s: %w", path, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode/100 != 2 {
		return fmt.Errorf("sidecar %s: status %d", path, resp.StatusCode)
	}
	return json.NewDecoder(resp.Body).Decode(dst)
}
