package main

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"github.com/BadgerOps/airgap/internal/config"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

func newConfigCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "config",
		Short: "Manage configuration",
		Long: `Manage airgap configuration. Subcommands allow viewing and modifying
configuration settings.`,
		Example: `  airgap config show
  airgap config set server.listen 127.0.0.1:9000`,
	}

	cmd.AddCommand(
		newConfigShowCmd(),
		newConfigSetCmd(),
	)

	return cmd
}

func newConfigShowCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "show",
		Short: "Display current configuration",
		Long: `Display the current configuration in YAML format. If a config file
is loaded, shows the loaded configuration with any command-line overrides
applied.`,
		Example: `  airgap config show
  airgap config show --config /etc/airgap/config.yaml`,
		RunE: configShowRun,
	}

	return cmd
}

func configShowRun(cmd *cobra.Command, args []string) error {
	log := slog.Default()

	if globalCfg == nil {
		return fmt.Errorf("config not loaded")
	}

	log.Info("showing configuration")

	data, err := yaml.Marshal(globalCfg)
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	fmt.Println("Current Configuration:")
	fmt.Println("======================")
	fmt.Println(string(data))

	return nil
}

func newConfigSetCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "set KEY VALUE",
		Short: "Set a configuration value",
		Long: `Set a configuration value using dot-notation for nested keys.
Changes are written back to the config file.

Examples:
  server.listen 127.0.0.1:9000
  server.data_dir /var/lib/airgap
  export.split_size 10GB
  export.compression gzip`,
		Example: `  airgap config set server.listen 127.0.0.1:9000
  airgap config set export.split_size 10GB`,
		Args: cobra.ExactArgs(2),
		RunE: configSetRun,
	}

	return cmd
}

func configSetRun(cmd *cobra.Command, args []string) error {
	log := slog.Default()

	if globalCfg == nil {
		return fmt.Errorf("config not loaded")
	}

	key := args[0]
	value := args[1]

	log.Info("set configuration", "key", key, "value", value)

	targetPath, err := resolveConfigWritePath()
	if err != nil {
		return err
	}

	rawCfg, err := loadRawConfig(targetPath)
	if err != nil {
		return err
	}

	parsedValue, err := parseConfigValue(value)
	if err != nil {
		return err
	}

	if err := setDotKey(rawCfg, key, parsedValue); err != nil {
		return err
	}

	data, err := yaml.Marshal(rawCfg)
	if err != nil {
		return fmt.Errorf("failed to marshal updated config: %w", err)
	}

	if err := os.MkdirAll(filepath.Dir(targetPath), 0o755); err != nil {
		return fmt.Errorf("failed to create config directory: %w", err)
	}
	if err := os.WriteFile(targetPath, data, 0o644); err != nil {
		return fmt.Errorf("failed to write config file: %w", err)
	}

	loaded, err := config.Load(targetPath)
	if err != nil {
		return fmt.Errorf("failed to reload updated config: %w", err)
	}
	globalCfg = loaded

	fmt.Printf("Updated %s in %s\n", key, targetPath)

	return nil
}

func resolveConfigWritePath() (string, error) {
	if strings.TrimSpace(cfgPath) != "" {
		return cfgPath, nil
	}

	found, err := config.FindConfigFile()
	if err == nil {
		return found, nil
	}

	// Fallback when no discovered config exists yet.
	return "airgap.yaml", nil
}

func loadRawConfig(path string) (map[string]interface{}, error) {
	raw := make(map[string]interface{})

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return raw, nil
		}
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}
	if len(strings.TrimSpace(string(data))) == 0 {
		return raw, nil
	}

	if err := yaml.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("failed to parse existing config: %w", err)
	}
	if raw == nil {
		raw = make(map[string]interface{})
	}
	return raw, nil
}

func parseConfigValue(raw string) (interface{}, error) {
	var value interface{}
	if err := yaml.Unmarshal([]byte(raw), &value); err != nil {
		return nil, fmt.Errorf("invalid value %q: %w", raw, err)
	}
	return value, nil
}

func setDotKey(cfg map[string]interface{}, key string, value interface{}) error {
	parts := strings.Split(key, ".")
	if len(parts) == 0 {
		return fmt.Errorf("config key is required")
	}

	current := cfg
	for i := 0; i < len(parts)-1; i++ {
		part := strings.TrimSpace(parts[i])
		if part == "" {
			return fmt.Errorf("invalid key %q: empty segment", key)
		}

		existing, ok := current[part]
		if !ok {
			next := make(map[string]interface{})
			current[part] = next
			current = next
			continue
		}

		next, ok := existing.(map[string]interface{})
		if !ok {
			return fmt.Errorf("invalid key %q: segment %q is not a map", key, part)
		}
		current = next
	}

	last := strings.TrimSpace(parts[len(parts)-1])
	if last == "" {
		return fmt.Errorf("invalid key %q: empty segment", key)
	}
	current[last] = value
	return nil
}
