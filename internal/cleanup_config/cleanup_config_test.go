package cleanupconfig

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestCleanupConfig_SetDefaults(t *testing.T) {
	config := CleanupConfig{}
	config.SetDefaults()

	require.Equal(t, 10, config.BatchSize, "default batch size should be 10")

	config = CleanupConfig{
		BatchSize: -10,
	}

	err := config.Validate()

	require.Error(t, err)
}

func TestCleanupConfig_Validate(t *testing.T) {
	tests := []struct {
		name      string
		config    CleanupConfig
		expectErr bool
	}{
		{
			name: "valid config",
			config: CleanupConfig{
				BatchSize: 5,
			},
			expectErr: false,
		},
		{
			name: "negative batch size",
			config: CleanupConfig{
				BatchSize: -1,
			},
			expectErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate()
			if tt.expectErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestDuration_UnmarshalYAML(t *testing.T) {
	type durationWrapper struct {
		TTL Duration `yaml:"ttl"`
	}

	yamlStr := "ttl: 1h30m"
	var wrapper durationWrapper
	err := yaml.Unmarshal([]byte(yamlStr), &wrapper)
	require.NoError(t, err, "unmarshal should not return an error")

	expectedDuration, _ := time.ParseDuration("1h30m")
	require.Equal(t, expectedDuration, wrapper.TTL.Duration, "duration should match expected value")

	yamlStr = "ttl: 1pk"
	err = yaml.Unmarshal([]byte(yamlStr), &wrapper)
	require.Error(t, err, "unmarshal should throw an error")

	yamlStr = "ttl: [123]"
	err = yaml.Unmarshal([]byte(yamlStr), &wrapper)
	require.Error(t, err, "unmarshal should throw an error")
}
func TestPodCleanupConfig_Validate(t *testing.T) {
	validRule := PodCleanRule{
		Name:    "test-rule",
		Enabled: true,
		TTL:     Duration{Duration: time.Hour},
		Phase:   "Succeeded",
	}

	tests := []struct {
		name      string
		config    PodCleanupConfig
		expectErr bool
	}{
		{
			name: "disabled config",
			config: PodCleanupConfig{
				Enabled: false,
			},
			expectErr: false,
		},
		{
			name: "valid rule",
			config: PodCleanupConfig{
				Enabled: true,
				Rules:   []PodCleanRule{validRule},
			},
			expectErr: false,
		},
		{
			name: "invalid rule inside config",
			config: PodCleanupConfig{
				Enabled: true,
				Rules: []PodCleanRule{
					{
						Name:    "",
						Enabled: true,
						TTL:     Duration{Duration: time.Hour},
						Phase:   "",
					},
				},
			},
			expectErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate()
			if tt.expectErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestPodCleanRule_Validate(t *testing.T) {
	tests := []struct {
		name      string
		rule      PodCleanRule
		expectErr bool
	}{
		{
			name: "disabled rule",
			rule: PodCleanRule{
				Enabled: false,
			},
			expectErr: false,
		},
		{
			name: "missing rule name",
			rule: PodCleanRule{
				Enabled: true,
				TTL:     Duration{Duration: time.Hour},
				Phase:   "Failed",
			},
			expectErr: true,
		},
		{
			name: "invalid TTL",
			rule: PodCleanRule{
				Name:    "invalid-ttl",
				Enabled: true,
				TTL:     Duration{Duration: 0},
				Phase:   "Failed",
			},
			expectErr: true,
		},
		{
			name: "missing selector and phase",
			rule: PodCleanRule{
				Name:    "missing-selector-phase",
				Enabled: true,
				TTL:     Duration{Duration: time.Hour},
			},
			expectErr: true,
		},
		{
			name: "valid rule with phase",
			rule: PodCleanRule{
				Name:    "valid",
				Enabled: true,
				TTL:     Duration{Duration: time.Hour},
				Phase:   "Succeeded",
			},
			expectErr: false,
		},
		{
			name: "valid rule with selector",
			rule: PodCleanRule{
				Name:    "valid-selector",
				Enabled: true,
				TTL:     Duration{Duration: time.Hour},
				Selector: metav1.LabelSelector{
					MatchLabels: map[string]string{"app": "myapp"},
				},
			},
			expectErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.rule.Validate()
			if tt.expectErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestYAMLUnmarshal_FullConfig(t *testing.T) {
	yamlConfig := `
dryRun: true
batchSize: 20
podCleanupConfig:
  enabled: true
  rules:
    - name: test-rule
      enabled: true
      ttl: "1h"
      phase: "Succeeded"
      namespaces:
        - default
        - kube-system
`

	var cfg CleanupConfig
	err := yaml.NewDecoder(strings.NewReader(yamlConfig)).Decode(&cfg)
	require.NoError(t, err)

	require.True(t, cfg.DryRun)
	require.Equal(t, 20, cfg.BatchSize)
	require.True(t, cfg.PodCleanupConfig.Enabled)
	require.Len(t, cfg.PodCleanupConfig.Rules, 1)
	require.Equal(t, "test-rule", cfg.PodCleanupConfig.Rules[0].Name)
	require.Equal(t, "Succeeded", cfg.PodCleanupConfig.Rules[0].Phase)
	require.Equal(t, cfg.PodCleanupConfig.Rules[0].TTL.Duration, time.Hour)
}

func TestYAMLUnmarshal_EmptyConfig(t *testing.T) {
	yamlConfig := `
dryRun: true
batchSize: 20
podCleanupConfig:
  enabled: true
  rules:
    - name: dvdvd
      enabled: true
      ttl: "1h"
      namespaces: []
`

	var cfg CleanupConfig
	err := yaml.NewDecoder(strings.NewReader(yamlConfig)).Decode(&cfg)

	require.NoError(t, err)

	err = cfg.Validate()

	require.Error(t, err)

}

// Test case for loading the configuration
func writeTempConfig(t *testing.T, yamlConfig string) string {
	t.Helper()

	tempFile, err := os.CreateTemp("", "config-*.yaml")
	require.NoError(t, err)

	_, err = tempFile.Write([]byte(yamlConfig))
	require.NoError(t, err)
	require.NoError(t, tempFile.Close())

	return tempFile.Name()
}

func deleteTempFile(t *testing.T, filePath string) {
	if err := os.Remove(filePath); err != nil {
		t.Logf("failed to remove temp file: %v", err)
	}
}

func Test_LoadConfigFromFile_Success(t *testing.T) {
	yamlConfig := `
dryRun: true
batchSize: 20
podCleanupConfig:
  enabled: true
  rules:
    - name: test-rule
      enabled: true
      ttl: "1h"
      phase: "Succeeded"
      namespaces:
        - default
        - kube-system
`
	filePath := writeTempConfig(t, yamlConfig)
	defer deleteTempFile(t, filePath)

	_, err := LoadConfigFromFile(filePath)
	require.NoError(t, err)
}

func Test_LoadConfigFromFile_YAMLError(t *testing.T) {
	yamlConfig := `
	dryRun: true
	batchSize: 20
	podCleanupConfig:
	enabled: true
	rules:
		- name: test-rule
		enabled: true
		ttl: "1h"
		phase: "Succeeded"
		namespaces:
			- default
			- kube-system
`
	filePath := writeTempConfig(t, yamlConfig)
	defer deleteTempFile(t, filePath)

	_, err := LoadConfigFromFile(filePath)
	require.Error(t, err)
	require.Contains(t, err.Error(), "failed to unmarshal config")
}

func Test_LoadConfigFromFile_InvalidConfigError(t *testing.T) {
	yamlConfig := `
dryRun: true
batchSize: 20
podCleanupConfig:
  enabled: true
  rules:
    - name: test-rule
      enabled: true
      ttl: "1h"
      phase: ""
      namespaces: []
`
	filePath := writeTempConfig(t, yamlConfig)
	defer deleteTempFile(t, filePath)

	_, err := LoadConfigFromFile(filePath)
	require.Error(t, err)
	require.Contains(t, err.Error(), "invalid config")
}

func Test_LoadConfigFromFile_FileReadError(t *testing.T) {
	// Non-existent file
	_, err := LoadConfigFromFile("non-existent-file.yaml")
	require.Error(t, err)
	require.Contains(t, err.Error(), "unable to read config file")
}

func Test_WatchConfig_ReloadsOnChange(t *testing.T) {
	initialConfig := `
dryRun: true
batchSize: 10
podCleanupConfig:
  enabled: true
  rules:
    - name: initial-rule
      enabled: true
      ttl: "1h"
      phase: "Succeeded"
      namespaces: [default]
`
	updatedConfig := `
dryRun: true
batchSize: 50
podCleanupConfig:
  enabled: true
  rules:
    - name: updated-rule
      enabled: true
      ttl: "2h"
      phase: "Succeeded"
      namespaces: [default, kube-system]
`

	invalidConfig := `
dryRun: true
batchSize: 50
podCleanupConfig:
  enabled: true
  rules:
      - name: updated-rule
      enabled: true
      ttl: "2h"
      phase: "Succeeded"
      namespaces: [default, kube-system]
`

	filePath := writeTempConfig(t, initialConfig)
	defer deleteTempFile(t, filePath)

	currentConfig, err := LoadConfigFromFile(filePath)
	require.NoError(t, err)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	ticker := time.NewTicker(100 * time.Millisecond)

	go WatchConfig(ctx, filePath, currentConfig, ticker)

	// Give watcher some time to start
	time.Sleep(150 * time.Millisecond)

	// Modify config file to trigger reload
	require.NoError(t, os.WriteFile(filePath, []byte(updatedConfig), 0644))

	// Give enough time for watcher to detect change and reload
	time.Sleep(300 * time.Millisecond)

	// Validate config has been updated
	require.Equal(t, 50, currentConfig.BatchSize)
	require.Equal(t, "updated-rule", currentConfig.PodCleanupConfig.Rules[0].Name)
	require.Equal(t, 2*time.Hour, currentConfig.PodCleanupConfig.Rules[0].TTL.Duration)
	require.Contains(t, currentConfig.PodCleanupConfig.Rules[0].Namespaces, "kube-system")

	// Modify config file to trigger reload
	require.NoError(t, os.WriteFile(filePath, []byte(invalidConfig), 0644))

	validConfig := currentConfig
	// Give enough time for watcher to detect change and reload
	time.Sleep(300 * time.Millisecond)

	require.Equal(t, currentConfig, validConfig)

	err = os.Remove(filePath)

	require.NoError(t, err)

	time.Sleep(300 * time.Millisecond)

	require.Equal(t, currentConfig, validConfig)

}
