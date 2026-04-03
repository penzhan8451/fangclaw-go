package commands

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/penzhan8451/fangclaw-go/internal/config"
)

func getDaemonAddress() (string, error) {
	cfg, err := config.Load("")
	if err != nil {
		return "", fmt.Errorf("failed to load config: %w", err)
	}
	return "http://" + cfg.APIListen, nil
}

func mustGetDaemonAddress() string {
	addr, err := getDaemonAddress()
	if err != nil {
		return "http://127.0.0.1:4200"
	}
	return addr
}

func cliHTTPRequest(method, url string, body io.Reader) (*http.Response, error) {
	req, err := http.NewRequest(method, url, body)
	if err != nil {
		return nil, err
	}
	req.Header.Set("X-Client-Type", "cli")
	return http.DefaultClient.Do(req)
}

func cliHTTPGet(url string) (*http.Response, error) {
	return cliHTTPRequest("GET", url, nil)
}

func cliHTTPPost(url, contentType string, body io.Reader) (*http.Response, error) {
	req, err := http.NewRequest("POST", url, body)
	if err != nil {
		return nil, err
	}
	req.Header.Set("X-Client-Type", "cli")
	req.Header.Set("Content-Type", contentType)
	return http.DefaultClient.Do(req)
}

func cliHTTPPostJSON(url string, body interface{}) (*http.Response, error) {
	data, err := json.Marshal(body)
	if err != nil {
		return nil, err
	}
	return cliHTTPPost(url, "application/json", bytes.NewReader(data))
}
