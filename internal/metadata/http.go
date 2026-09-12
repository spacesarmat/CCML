package metadata

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
)

func getJSON(ctx context.Context, client *http.Client, req *http.Request, provider string, target any) (resultErr error) {
	if ctx == nil {
		return errors.New("context is required")
	}
	resp, err := client.Do(req.WithContext(ctx))
	if err != nil {
		return fmt.Errorf("request %s: %w", provider, err)
	}
	defer func() {
		if err := resp.Body.Close(); err != nil {
			resultErr = errors.Join(resultErr, fmt.Errorf("close %s response body: %w", provider, err))
		}
	}()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, readErr := io.ReadAll(io.LimitReader(resp.Body, 4096))
		if readErr != nil {
			return errors.Join(
				fmt.Errorf("%s returned HTTP %d", provider, resp.StatusCode),
				fmt.Errorf("read error response: %w", readErr),
			)
		}
		return fmt.Errorf("%s returned HTTP %d: %s", provider, resp.StatusCode, string(body))
	}
	if err := json.NewDecoder(resp.Body).Decode(target); err != nil {
		return fmt.Errorf("decode %s response: %w", provider, err)
	}
	return nil
}
