package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/BadgerOps/airgap/internal/config"
	"gopkg.in/yaml.v3"
)

func TestConfigSetRunPersistsTypedValues(t *testing.T) {
	tmp := t.TempDir()
	prevWD, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd failed: %v", err)
	}
	if err := os.Chdir(tmp); err != nil {
		t.Fatalf("Chdir failed: %v", err)
	}
	t.Cleanup(func() {
		_ = os.Chdir(prevWD)
	})

	prevCfgPath := cfgPath
	prevGlobalCfg := globalCfg
	t.Cleanup(func() {
		cfgPath = prevCfgPath
		globalCfg = prevGlobalCfg
	})

	cfgPath = filepath.Join(tmp, "airgap.yaml")
	globalCfg = config.DefaultConfig()

	if err := configSetRun(nil, []string{"server.listen", "127.0.0.1:9000"}); err != nil {
		t.Fatalf("configSetRun(server.listen) failed: %v", err)
	}
	if err := configSetRun(nil, []string{"providers.custom.sources", `["https://example.com/a","https://example.com/b"]`}); err != nil {
		t.Fatalf("configSetRun(providers.custom.sources) failed: %v", err)
	}
	if err := configSetRun(nil, []string{"schedule.enabled", "false"}); err != nil {
		t.Fatalf("configSetRun(schedule.enabled) failed: %v", err)
	}

	data, err := os.ReadFile(cfgPath)
	if err != nil {
		t.Fatalf("reading config file failed: %v", err)
	}
	raw := map[string]interface{}{}
	if err := yaml.Unmarshal(data, &raw); err != nil {
		t.Fatalf("unmarshal updated config failed: %v", err)
	}

	serverRaw, ok := raw["server"].(map[string]interface{})
	if !ok {
		t.Fatalf("server map missing or wrong type: %#v", raw["server"])
	}
	if got := serverRaw["listen"]; got != "127.0.0.1:9000" {
		t.Fatalf("server.listen mismatch: got %#v", got)
	}

	scheduleRaw, ok := raw["schedule"].(map[string]interface{})
	if !ok {
		t.Fatalf("schedule map missing or wrong type: %#v", raw["schedule"])
	}
	if got, ok := scheduleRaw["enabled"].(bool); !ok || got {
		t.Fatalf("schedule.enabled mismatch: got %#v", scheduleRaw["enabled"])
	}

	if globalCfg.Server.Listen != "127.0.0.1:9000" {
		t.Fatalf("globalCfg not reloaded: got %q", globalCfg.Server.Listen)
	}
}

func TestConfigSetRunCreatesDefaultConfigFile(t *testing.T) {
	tmp := t.TempDir()
	prevWD, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd failed: %v", err)
	}
	if err := os.Chdir(tmp); err != nil {
		t.Fatalf("Chdir failed: %v", err)
	}
	t.Cleanup(func() {
		_ = os.Chdir(prevWD)
	})

	prevCfgPath := cfgPath
	prevGlobalCfg := globalCfg
	t.Cleanup(func() {
		cfgPath = prevCfgPath
		globalCfg = prevGlobalCfg
	})

	cfgPath = ""
	globalCfg = config.DefaultConfig()

	if err := configSetRun(nil, []string{"server.listen", "127.0.0.1:8081"}); err != nil {
		t.Fatalf("configSetRun failed: %v", err)
	}

	target := filepath.Join(tmp, "airgap.yaml")
	if _, err := os.Stat(target); err != nil {
		t.Fatalf("expected fallback config file at %s: %v", target, err)
	}
}

func TestConfigSetRunValidationErrors(t *testing.T) {
	prevCfgPath := cfgPath
	prevGlobalCfg := globalCfg
	t.Cleanup(func() {
		cfgPath = prevCfgPath
		globalCfg = prevGlobalCfg
	})

	cfgPath = filepath.Join(t.TempDir(), "airgap.yaml")
	globalCfg = config.DefaultConfig()

	if err := configSetRun(nil, []string{"server..listen", "127.0.0.1:9000"}); err == nil {
		t.Fatal("expected error for invalid key with empty segment")
	}
	if err := configSetRun(nil, []string{"server.listen", "["}); err == nil {
		t.Fatal("expected error for invalid YAML value")
	}
}
