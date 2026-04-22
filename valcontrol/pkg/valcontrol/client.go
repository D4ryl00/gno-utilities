package valcontrol

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

type Client struct {
	http *http.Client
}

type RPCStatus struct {
	Result struct {
		NodeInfo struct {
			Moniker string `json:"moniker"`
		} `json:"node_info"`
		SyncInfo struct {
			LatestBlockHeight string `json:"latest_block_height"`
			LatestBlockHash   string `json:"latest_block_hash"`
			CatchingUp        bool   `json:"catching_up"`
		} `json:"sync_info"`
	} `json:"result"`
}

type RuleView struct {
	Action string `json:"action"`
	Height *int64 `json:"height,omitempty"`
	Round  *int   `json:"round,omitempty"`
	Delay  string `json:"delay,omitempty"`
}

type TargetHit struct {
	Height int64     `json:"height"`
	Round  int       `json:"round"`
	At     time.Time `json:"at"`
}

type PhaseStats struct {
	Matched   int64      `json:"matched"`
	Dropped   int64      `json:"dropped"`
	Delayed   int64      `json:"delayed"`
	LastMatch *TargetHit `json:"last_match,omitempty"`
}

type SignerState struct {
	Address string                `json:"address"`
	PubKey  string                `json:"pub_key"`
	Rules   map[string]*RuleView  `json:"rules"`
	Stats   map[string]PhaseStats `json:"stats"`
}

type ValidatorSnapshot struct {
	Validator *Validator   `json:"validator"`
	RPC       *RPCStatus   `json:"rpc,omitempty"`
	Signer    *SignerState `json:"signer,omitempty"`
	RPCErr    string       `json:"rpc_err,omitempty"`
	SignerErr string       `json:"signer_err,omitempty"`
}

func NewClient(timeout time.Duration) *Client {
	return &Client{http: &http.Client{Timeout: timeout}}
}

func (c *Client) GetRPCStatus(rpcURL string) (*RPCStatus, error) {
	var status RPCStatus
	if err := c.getJSON(strings.TrimRight(rpcURL, "/")+"/status", &status); err != nil {
		return nil, err
	}
	return &status, nil
}

func (c *Client) GetSignerState(controlURL string) (*SignerState, error) {
	var state SignerState
	if err := c.getJSON(strings.TrimRight(controlURL, "/")+"/state", &state); err != nil {
		return nil, err
	}
	return &state, nil
}

func (c *Client) PutRule(controlURL, phase, action string, height *int64, round *int, delay string) error {
	payload := map[string]any{"action": action}
	if height != nil {
		payload["height"] = *height
	}
	if round != nil {
		payload["round"] = *round
	}
	if delay != "" {
		payload["delay"] = delay
	}

	return c.sendJSON(http.MethodPut, strings.TrimRight(controlURL, "/")+"/rules/"+phase, payload)
}

func (c *Client) ClearRule(controlURL, phase string) error {
	return c.sendJSON(http.MethodDelete, strings.TrimRight(controlURL, "/")+"/rules/"+phase, nil)
}

func (c *Client) Reset(controlURL string) error {
	return c.sendJSON(http.MethodPost, strings.TrimRight(controlURL, "/")+"/reset", nil)
}

func (c *Client) Snapshot(v Validator) ValidatorSnapshot {
	snap := ValidatorSnapshot{Validator: &v}

	rpc, err := c.GetRPCStatus(v.RPCURL)
	if err != nil {
		snap.RPCErr = err.Error()
	} else {
		snap.RPC = rpc
	}

	if v.ControlURL != nil {
		signer, err := c.GetSignerState(*v.ControlURL)
		if err != nil {
			snap.SignerErr = err.Error()
		} else {
			snap.Signer = signer
		}
	}

	return snap
}

func (c *Client) getJSON(url string, target any) error {
	resp, err := c.http.Get(url)
	if err != nil {
		return fmt.Errorf("get %s: %w", url, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return fmt.Errorf("get %s: status %s: %s", url, resp.Status, strings.TrimSpace(string(body)))
	}

	if err := json.NewDecoder(resp.Body).Decode(target); err != nil {
		return fmt.Errorf("decode %s: %w", url, err)
	}

	return nil
}

func (c *Client) sendJSON(method, url string, payload any) error {
	var body io.Reader
	if payload != nil {
		bz, err := json.Marshal(payload)
		if err != nil {
			return fmt.Errorf("marshal request: %w", err)
		}
		body = bytes.NewReader(bz)
	}

	req, err := http.NewRequest(method, url, body)
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}
	if payload != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("%s %s: %w", method, url, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return fmt.Errorf("%s %s: status %s: %s", method, url, resp.Status, strings.TrimSpace(string(body)))
	}

	return nil
}
