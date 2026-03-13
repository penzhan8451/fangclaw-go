package commands

import (
	"fmt"

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
