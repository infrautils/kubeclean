package cleanupconfig

import (
	"fmt"
	"time"
)

//
// Root Cleanup Configuration
//

// CleanupConfig defines the root configuration for the cleanup process.
// It includes global settings such as dry run mode, batch size, and pod cleanup-specific config.
type CleanupConfig struct {
	DryRun           bool             `yaml:"dryRun,omitempty"`           // If true, performs a dry-run without actual deletion.
	BatchSize        int              `yaml:"batchSize,omitempty"`        // Number of resources processed per batch; defaults to 10.
	PodCleanupConfig PodCleanupConfig `yaml:"podCleanupConfig,omitempty"` // Configuration specific to pod cleanup.
}

// SetDefaults sets default values for CleanupConfig.
// Currently, it ensures BatchSize is set to a reasonable default if not provided.
func (c *CleanupConfig) SetDefaults() {
	if c.BatchSize <= 0 {
		c.BatchSize = 10 // Default batch size
	}
}

// Validate checks the correctness of CleanupConfig.
// It validates BatchSize and recursively validates PodCleanupConfig.
func (c *CleanupConfig) Validate() error {
	if c.BatchSize < 0 {
		return fmt.Errorf("batch size cannot be negative")
	}

	if err := c.PodCleanupConfig.Validate(); err != nil {
		return fmt.Errorf("pod cleanup config error: %w", err)
	}

	return nil
}

//
// Resource Selector Utility
//

// LabelSelector specifies Kubernetes-style label selection criteria.
// It’s used to filter resources based on their labels.
type LabelSelector struct {
	MatchLabels map[string]string `yaml:"matchLabels,omitempty"` // Key-value pairs of labels to match.
}

//
// Duration Helper for YAML Parsing
//

// Duration is a wrapper around time.Duration that allows parsing YAML duration strings.
type Duration struct {
	time.Duration `yaml:"ttl"`
}

// UnmarshalYAML parses a string from YAML into a time.Duration.
// Example YAML value: "5m", "1h30m", etc.
func (d *Duration) UnmarshalYAML(unmarshal func(interface{}) error) error {
	var s string
	if err := unmarshal(&s); err != nil {
		return err
	}

	duration, err := time.ParseDuration(s)
	if err != nil {
		return fmt.Errorf("invalid duration %q: %w", s, err)
	}

	d.Duration = duration
	return nil
}

//
// Pod Cleanup Configuration
//

// PodCleanupConfig defines rules and settings for cleaning up Kubernetes pods.
type PodCleanupConfig struct {
	Enabled bool           `yaml:"enabled,omitempty"` // If false, pod cleanup is disabled.
	Rules   []PodCleanRule `yaml:"rules,omitempty"`   // List of rules for selecting and cleaning up pods.
}

// Validate ensures PodCleanupConfig is correctly configured.
// It validates each rule if the config is enabled.
func (p *PodCleanupConfig) Validate() error {
	if !p.Enabled {
		return nil // Skip validation if disabled
	}

	var errorMessages string

	fmt.Println(p.Rules)

	for idx, rule := range p.Rules {
		if err := rule.Validate(); err != nil {
			errorMessages += fmt.Sprintf("rule %d (%s): %v\n", idx+1, rule.Name, err)
		}
	}

	if errorMessages == "" {
		return nil
	}

	return fmt.Errorf("pod cleanup config validation errors:\n%s", errorMessages)
}

//
// Pod Cleanup Rule Configuration
//

// PodCleanRule defines an individual cleanup rule for selecting and deleting pods.
type PodCleanRule struct {
	Name       string        `yaml:"name"`                 // Unique name of the rule for identification.
	Enabled    bool          `yaml:"enabled,omitempty"`    // If false, the rule is skipped during processing.
	Selector   LabelSelector `yaml:"selector,omitempty"`   // Label selector to filter pods.
	Phase      string        `yaml:"phase,omitempty"`      // Pod phase (e.g., "Succeeded", "Failed") to filter pods.
	TTL        Duration      `yaml:"ttl"`                  // Time-to-live duration after which pods are eligible for cleanup.
	Namespaces []string      `yaml:"namespaces,omitempty"` // Specific namespaces where the rule applies.
}

// Validate checks whether the PodCleanRule is correctly defined.
// Ensures required fields are set and the configuration makes sense.
func (r *PodCleanRule) Validate() error {
	if !r.Enabled {
		return nil // Skip validation for disabled rules
	}

	if r.Name == "" {
		return fmt.Errorf("rule name must be provided")
	}

	if r.TTL.Duration <= 0 {
		return fmt.Errorf("ttl must be greater than zero")
	}

	// Require at least 'phase' or 'selector.matchLabels' to be set.
	if r.Phase == "" && len(r.Selector.MatchLabels) == 0 {
		return fmt.Errorf("either 'phase' or 'selector.matchLabels' must be specified")
	}

	return nil
}
