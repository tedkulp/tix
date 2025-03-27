package utils

import (
	"github.com/pkg/browser"
	"github.com/tedkulp/tix/internal/logger"
)

// OpenURL opens a URL in the default browser
func OpenURL(url string) error {
	logger.Debug("Opening URL in browser", map[string]interface{}{
		"url": url,
	})

	err := browser.OpenURL(url)
	if err != nil {
		logger.Warn("Failed to open browser", map[string]interface{}{
			"error": err.Error(),
		})
		return err
	}

	return nil
}
