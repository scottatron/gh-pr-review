package github

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

type Client struct {
	endpoint   string
	token      string
	httpClient *http.Client
}

type GraphQLRequest struct {
	Query     string                 `json:"query"`
	Variables map[string]interface{} `json:"variables,omitempty"`
}

type graphQLError struct {
	Message string `json:"message"`
}

type graphQLResponse struct {
	Data   json.RawMessage `json:"data"`
	Errors []graphQLError  `json:"errors"`
}

func NewClient(endpoint, token string) *Client {
	return &Client{
		endpoint: endpoint,
		token:    token,
		httpClient: &http.Client{
			Timeout: 20 * time.Second,
		},
	}
}

func (c *Client) Do(ctx context.Context, query string, variables map[string]interface{}, out interface{}) error {
	if c == nil {
		return errors.New("nil github client")
	}
	payload, err := json.Marshal(GraphQLRequest{Query: query, Variables: variables})
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.endpoint, bytes.NewReader(payload))
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "bearer "+c.token)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return fmt.Errorf("github api error: status %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}

	var gr graphQLResponse
	if err := json.Unmarshal(body, &gr); err != nil {
		return err
	}
	if len(gr.Errors) > 0 {
		msgs := make([]string, 0, len(gr.Errors))
		for _, ge := range gr.Errors {
			if ge.Message != "" {
				msgs = append(msgs, ge.Message)
			}
		}
		return fmt.Errorf("graphql error: %s", strings.Join(msgs, "; "))
	}
	if out == nil {
		return nil
	}
	if len(gr.Data) == 0 {
		return errors.New("graphql response missing data")
	}
	return json.Unmarshal(gr.Data, out)
}

func GraphQLEndpoint(host string) string {
	if host == "" || host == "github.com" {
		return "https://api.github.com/graphql"
	}
	return fmt.Sprintf("https://%s/api/graphql", host)
}
