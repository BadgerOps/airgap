# Configuration

## File Discovery

`airgap` searches for config in this order:
- `./airgap.yaml`
- `/etc/airgap/airgap.yaml`
- `$HOME/.config/airgap/airgap.yaml`

Use `--config` to specify an explicit path.

## Top-Level Schema

```yaml
server:
  listen: "0.0.0.0:8080"
  data_dir: "/var/lib/airgap"
  db_path: "/var/lib/airgap/airgap.db"

export:
  split_size: "25GB"
  compression: "zstd"
  output_dir: "/mnt/transfer-disk"
  manifest_name: "airgap-manifest.json"

schedule:
  enabled: true
  default_cron: "0 2 * * 0"

providers: {}
```

## Schedule Settings

- `schedule.enabled`: enables the recurring job scheduler while `airgap serve` is running.
- `schedule.default_cron`: default cron used by `airgap jobs add --cron` when `--cron` is omitted.

Notes:
- Scheduler execution is serve-only; it does not run from non-server CLI commands.
- Cron format is standard 5-field (`minute hour day month day-of-week`).

## Provider Config Storage Model

At runtime, provider configs are read from SQLite (`provider_configs`), not directly from YAML.

Behavior:
- On first startup with an empty `provider_configs` table, YAML `providers:` entries are seeded into DB.
- On later startups, DB provider configs are authoritative.
- Provider CRUD in the UI/API updates DB and hot-reloads active providers.

## Valid Provider Types

- `epel`
- `ocp_binaries`
- `ocp_clients`
- `rhcos`
- `container_images`
- `registry`
- `custom_files`

### Implementation Status

- Fully wired for sync/validate: `epel`, `ocp_binaries`, `ocp_clients`, `rhcos`, `container_images`
- Used as registry push target config: `registry`
- Accepted config type but not wired for sync: `custom_files`

## Example Config

See [configs/airgap.example.yaml](../configs/airgap.example.yaml).

## CLI Config Commands

- `airgap config show`: prints effective loaded config
- `airgap config set KEY VALUE`: persists updates to YAML (typed value parsing)

`config set` write target resolution:
- explicit `--config` path (if provided)
- discovered config path from file discovery order
- fallback `./airgap.yaml` when no config file exists

`VALUE` is parsed as YAML, so booleans/numbers/lists/maps are stored as typed values.

## Global CLI Flags

- `--config`: config path
- `--data-dir`: overrides `server.data_dir`
- `--log-level`: `debug|info|warn|error`
- `--log-format`: `text|json`
- `--quiet`: suppresses non-error output
