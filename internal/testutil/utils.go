package testutil

import (
	"log"
	"os"
	"testing"
)

func TestLogger(t *testing.T) *log.Logger {
	logger := log.New(os.Stdout, "[test] ", log.LstdFlags)
	t.Cleanup(func() {
		logger.SetOutput(os.Stderr)
	})
	return logger
}
