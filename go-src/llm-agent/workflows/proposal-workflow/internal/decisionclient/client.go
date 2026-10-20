package decisionclient

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"proposal-platform/api"
)

// Client owns only the HTTP candidate delivery and decision-feed boundary.
type Client struct {
	URL  string
	HTTP *http.Client
}

func (c *Client) request(ctx context.Context, method, path string, input, output any) error {
	var body io.Reader
	if input != nil {
		data, err := json.Marshal(input)
		if err != nil {
			return err
		}
		body = bytes.NewReader(data)
	}
	request, err := http.NewRequestWithContext(ctx, method, strings.TrimRight(c.URL, "/")+path, body)
	if err != nil {
		return err
	}
	if input != nil {
		request.Header.Set("Content-Type", "application/json")
	}
	response, err := c.HTTP.Do(request)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	data, err := io.ReadAll(io.LimitReader(response.Body, 32<<20))
	if err != nil {
		return err
	}
	if response.StatusCode != 200 {
		return &ResponseError{Status: response.StatusCode, Body: string(data)}
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	return decoder.Decode(output)
}

func (c *Client) Deliver(ctx context.Context, material api.Candidate) (api.Receipt, error) {
	input := api.Delivery{ID: material.Proposal + "/" + material.ID, Candidate: material}
	var receipt api.Receipt
	err := c.request(ctx, "POST", "/api/deliveries", input, &receipt)
	return receipt, err
}

func (c *Client) Feed(ctx context.Context, identity string, after uint64) (api.Feed, error) {
	query := url.Values{"journal": {identity}, "after": {strconv.FormatUint(after, 10)}, "wait": {"20"}}
	var feed api.Feed
	err := c.request(ctx, "GET", "/api/decisions?"+query.Encode(), nil, &feed)
	return feed, err
}
