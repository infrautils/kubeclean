package cleanupconfig

import (
	"context"
	"fmt"
	"os"
	"time"

	"gopkg.in/yaml.v2"
	ctrl "sigs.k8s.io/controller-runtime"
)

// LoadConfig loads CleanupConfig from YAML bytes.
func LoadConfig(data []byte) (*CleanupConfig, error) {
	var config CleanupConfig

	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to unmarshal config: %w", err)
	}

	if err := config.Validate(); err != nil {
		return nil, fmt.Errorf("invalid config: %w", err)
	}

	return &config, nil
}

// LoadConfigFromFile loads CleanupConfig from YAML config file.
func LoadConfigFromFile(configPath string) (*CleanupConfig, error) {
	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("unable to read config file %q: %w", configPath, err)
	}

	return LoadConfig(data)
}

// WatchConfig watches for configuration changes and reloads config.
func WatchConfig(ctx context.Context, configPath string, currentConfig *CleanupConfig, ticker *time.Ticker) {
	var setupLog = ctrl.Log.WithName("WatchConfig")

	defer ticker.Stop()

	var lastModTime time.Time
	if stat, err := os.Stat(configPath); err == nil {
		lastModTime = stat.ModTime()
	}

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			stat, err := os.Stat(configPath)
			if err != nil {
				setupLog.Error(err, "Failed to stat config file", "path", configPath)
				continue
			}

			if stat.ModTime().After(lastModTime) {
				setupLog.Info("Configuration file changed, reloading...", "path", configPath)

				newConfig, err := LoadConfigFromFile(configPath)
				if err != nil {
					setupLog.Error(err, "Failed to reload config file", "path", configPath)
					continue
				}

				*currentConfig = *newConfig
				lastModTime = stat.ModTime()
				setupLog.Info("Configuration reloaded successfully", "path", configPath)
			}
		}
	}
}
