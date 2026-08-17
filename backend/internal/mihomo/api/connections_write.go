package api

import (
	"fmt"
	"io"
	"net/http"
	"strings"
)

// CloseConnection asks Mihomo to drop a live connection by id
// (DELETE /connections/{id}).
func (c *Client) CloseConnection(id string) error {
	if c == nil {
		return fmt.Errorf("mihomo api client is nil")
	}
	id = strings.TrimSpace(id)
	if id == "" {
		return fmt.Errorf("connection id is required")
	}
	req, err := http.NewRequest(http.MethodDelete, c.BaseURL+"/connections/"+id, nil)
	if err != nil {
		return err
	}
	if c.Secret != "" {
		req.Header.Set("Authorization", "Bearer "+c.Secret)
	}
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 1024))
	if resp.StatusCode >= 300 {
		return fmt.Errorf("mihomo close connection returned status %d", resp.StatusCode)
	}
	return nil
}
