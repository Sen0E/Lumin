// A standalone HTTP translator. Build it with CGO_ENABLED=0; no desktop SDK is linked.
package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"time"

	"lumin/pkg/pluginapi"
)

func translate(ctx context.Context, r pluginapi.Request) (string, error) {
	if r.Operation != "translate" {
		return "", errors.New("unsupported operation")
	}
	endpoint := r.Settings["endpoint"]
	u, err := url.Parse(endpoint)
	if err != nil || u.Host == "" || (u.Scheme != "http" && u.Scheme != "https") || u.User != nil {
		return "", errors.New("invalid HTTP endpoint")
	}
	body, _ := json.Marshal(map[string]string{"q": r.Text, "source": r.Source, "target": r.Target, "format": "text", "api_key": r.Settings["api_key"]})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	client := &http.Client{Timeout: 60 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	resp, err := client.Do(req)
	if err != nil {
		return "", errors.New("translation service unavailable")
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("translation service HTTP %d", resp.StatusCode)
	}
	b, err := io.ReadAll(io.LimitReader(resp.Body, (2<<20)+1))
	if err != nil || len(b) > 2<<20 {
		return "", errors.New("invalid response size")
	}
	var result struct {
		Text string `json:"translatedText"`
	}
	if err := json.Unmarshal(b, &result); err != nil {
		return "", errors.New("invalid service response")
	}
	return result.Text, nil
}

func main() {
	if err := pluginapi.Serve(translate); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
