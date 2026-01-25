package api

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

type Client struct {
	baseURL string
	client  *http.Client
}

func NewClient(base string, timeout time.Duration) *Client {
	return &Client{baseURL: base, client: &http.Client{Timeout: timeout}}
}

func (c *Client) AcquireLease(ctx context.Context, req *AcquireLeaseRequest) (*Lease, error) {
	return post[AcquireLeaseRequest, Lease](ctx, c, "/lease/acquire", req)
}

func (c *Client) RenewLease(ctx context.Context, req *RenewLeaseRequest) (*Lease, error) {
	return post[RenewLeaseRequest, Lease](ctx, c, "/lease/renew", req)
}

func (c *Client) GetLease(ctx context.Context, rangeID string) (*Lease, error) {
	u := fmt.Sprintf("%s/lease/get?range_id=%s", c.baseURL, url.QueryEscape(rangeID))
	var out Lease
	if err := c.doJSON(ctx, http.MethodGet, u, nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func (c *Client) AppendSegment(ctx context.Context, seg *Segment) (*AppendSegmentResponse, error) {
	return post[Segment, AppendSegmentResponse](ctx, c, "/segment/append", seg)
}

func (c *Client) Subscribe(ctx context.Context, from uint64) ([]*Segment, error) {
	u := fmt.Sprintf("%s/segment/subscribe?from_commit_index=%d", c.baseURL, from)
	var out []*Segment
	if err := c.doJSON(ctx, http.MethodGet, u, nil, &out); err != nil {
		return nil, err
	}
	return out, nil
}

func (c *Client) Status(ctx context.Context) (*StatusResponse, error) {
	var out StatusResponse
	if err := c.doJSON(ctx, http.MethodGet, c.baseURL+"/status", nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func post[In any, Out any](ctx context.Context, c *Client, path string, req *In) (*Out, error) {
	var out Out
	if err := c.doJSON(ctx, http.MethodPost, c.baseURL+path, req, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func (c *Client) doJSON(ctx context.Context, method, url string, in any, out any) error {
	var body io.Reader
	if in != nil {
		buf, err := json.Marshal(in)
		if err != nil {
			return err
		}
		body = bytes.NewReader(buf)
	}
	req, err := http.NewRequestWithContext(ctx, method, url, body)
	if err != nil {
		return err
	}
	if in != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := c.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		data, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("http %d: %s", resp.StatusCode, string(data))
	}
	if out == nil {
		return nil
	}
	return json.NewDecoder(resp.Body).Decode(out)
}
