package providers

import (
	"strings"

	"github.com/sipeed/picoclaw/pkg/config"
)

func normalizeCLIExecutionMode(mode string) string {
	switch strings.ToLower(strings.TrimSpace(mode)) {
	case config.ExecutionModePermissive:
		return config.ExecutionModePermissive
	case "", config.ExecutionModeSafe:
		return config.ExecutionModeSafe
	default:
		return config.ExecutionModeSafe
	}
}
