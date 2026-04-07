package agent

import (
	"os"
	"testing"

	"github.com/sipeed/picoclaw/pkg/logger"
)

func TestMain(m *testing.M) {
	initialLevel := logger.GetLevel()
	logger.SetLevel(logger.ERROR)
	code := m.Run()
	logger.SetLevel(initialLevel)
	os.Exit(code)
}
