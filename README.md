# LLM Speed Test

A lightweight benchmark toolkit for LLM APIs. Run concurrent streaming requests, collect fine-grained latency metrics (TTFB, token rate, DNS/TCP/TLS breakdown), and visualize results through a web UI or CLI.

**Repository:** [github.com/SmilingXinyi/go-llm-speedtest](https://github.com/SmilingXinyi/go-llm-speedtest)

## Features

- **Single binary** — frontend embedded via `go:embed`; one executable serves API + web UI
- **Web dashboard** — run tests, review history, upload CSV, inspect per-request details
- **CLI mode** — headless benchmark without a browser
- **Streaming benchmark** — concurrent requests against OpenAI-compatible APIs
- **Rich metrics** — TTFB, total latency, token rate, DNS/TCP/TLS timing, connection reuse
- **Multi-channel config** — manage providers and models via `llm.yaml`

## Usage

`llm-studio` ships as one binary with the web UI baked in. Pick a mode below.

### 1. Get the binary

Download the latest release for your platform:

**[Releases](https://github.com/SmilingXinyi/go-llm-speedtest/releases)**

Or build from source — see [Development](#development).

### 2. Configure channels

Create a working directory and add your API config:

```bash
mkdir -p conf history
cp llm.yaml.example conf/llm.yaml   # or copy from the release bundle
```

Edit `conf/llm.yaml`:

```yaml
channels:
  - name: my-provider
    base_url: https://api.example.com/v1
    token: sk-your-token
    models:
      - gpt-4o-mini
```

> **Security:** Never share or commit real tokens.

Run `llm-studio` from this directory so default paths (`conf/llm.yaml`, `history/`) resolve correctly.

### 3a. Web UI mode

Start the server and open the browser:

```bash
./llm-studio
```

Visit **http://localhost:8787** — configure channels, run benchmarks, and review results in one place.

Optional flags:

```bash
./llm-studio --addr :8787 --config conf/llm.yaml --history history
```

### 3b. CLI mode

Run a benchmark without the web UI:

```bash
./llm-studio bench \
  --channel my-provider \
  --model gpt-4o-mini \
  --prompt "Introduce yourself in one sentence" \
  --concurrency 5 \
  --out csv
```

| Flag            | Description                                |
| --------------- | ------------------------------------------ |
| `--channel`     | Channel name (required)                    |
| `--model`       | Model name (defaults to first in channel)  |
| `--prompt`      | Prompt text; supports `file://path`        |
| `--thinking`    | Enable thinking mode                       |
| `--concurrency` | Concurrent requests (default 1, max 100) |
| `--out csv`     | Write results to `history/` as CSV         |
| `--config`      | Path to `llm.yaml` (default `conf/llm.yaml`)|

Print to stdout instead of saving:

```bash
./llm-studio bench --channel my-provider --prompt "Hello"
```

## Configuration

### `llm.yaml`

Each channel is one OpenAI-compatible API endpoint:

| Field      | Description                                     |
| ---------- | ----------------------------------------------- |
| `name`     | Channel identifier, used in UI and CLI          |
| `base_url` | API base URL (e.g. `https://api.openai.com/v1`) |
| `token`    | Bearer token                                    |
| `models`   | Supported models; first entry is the default    |

Built-in adapters exist for `nange` and `qianfan`; other channels use the generic OpenAI-compatible client.

### Environment variables

Place an optional `.env` next to the binary working directory. The server loads it without overriding existing environment variables. See `backend/.env.example` in the repository for reference.

## Benchmark Output

CLI and web UI both write CSV files to `history/`:

```
bench_<model>_<channel>_<count>_<date>_<time>.csv
```

Key metrics per request:

| Metric                         | Description                          |
| ------------------------------ | ------------------------------------ |
| `ttfb_ms`                      | Time to first token                  |
| `total_ms`                     | End-to-end latency                   |
| `token_rate`                   | Generation speed (tokens/sec)        |
| `prompt_tokens`                | Input token count                    |
| `completion_tokens`            | Output token count                   |
| `cached_tokens`                | Cache hit tokens                     |
| `dns_ms` / `tcp_ms` / `tls_ms` | Network phase timing                 |
| `conn_reused`                  | Whether the HTTP connection was reused |

## Architecture

```
┌─────────────────────────────────────────┐
│              llm-studio                 │
│  ┌─────────────┐    ┌────────────────┐  │
│  │  Web UI     │    │  HTTP API      │  │
│  │  (embedded) │◄──►│  (Echo v5)     │  │
│  └─────────────┘    └───────┬────────┘  │
└─────────────────────────────┼───────────┘
                              ▼
                     OpenAI-compatible APIs
```

| Layer    | Stack                                      |
| -------- | ------------------------------------------ |
| Backend  | Go 1.25+, Echo v5, YAML config, go:embed   |
| Frontend | React 19, Vite 8, Tailwind CSS 4, i18next |

## API Reference

| Method   | Path                  | Description              |
| -------- | --------------------- | ------------------------ |
| `GET`    | `/api/channels`       | List configured channels |
| `POST`   | `/api/channels`       | Add a channel            |
| `DELETE` | `/api/channels/:name` | Remove a channel         |
| `POST`   | `/api/bench`          | Run a benchmark          |
| `GET`    | `/api/history`        | List result files        |
| `GET`    | `/api/history/:name`  | Download a result file   |

## Development

For contributors who want to hack on the source.

### Prerequisites

- [Go](https://go.dev/) 1.25+
- [Node.js](https://nodejs.org/) 20+ with [pnpm](https://pnpm.io/)
- [Task](https://taskfile.dev/installation/)

### Clone and build

```bash
git clone https://github.com/SmilingXinyi/go-llm-speedtest.git
cd go-llm-speedtest

cp backend/conf/llm.yaml.example backend/conf/llm.yaml
task build
```

Output: `bin/llm-studio` (frontend embedded via `go:embed`).

```bash
cd backend && ../bin/llm-studio
```

### Task commands

| Command          | Description                                       |
| ---------------- | ------------------------------------------------- |
| `task install`   | Install frontend and backend dependencies         |
| `task build`     | Build single binary with embedded frontend        |
| `task build-api` | Build API-only binary (needs `--static` at runtime)|
| `task dev`       | Hot-reload dev: API (:8787) + Vite (:5173)        |
| `task server`    | API server only                                   |
| `task clean`     | Remove `bin/` and embedded frontend artifacts     |

### Local dev with hot reload

```bash
task dev
```

| Service | URL                   |
| ------- | --------------------- |
| Web UI  | http://localhost:5173 |
| API     | http://localhost:8787 |

Vite proxies `/api` to the backend. Production usage does not need this — use `task build` instead.

### Build pipeline

```bash
task frontend    # pnpm build → frontend/dist
task embed       # copy dist into backend for go:embed
task build       # go build -tags embed → bin/llm-studio
```

### Project structure

```
go-llm-speedtest/
├── Taskfile.yml
├── backend/
│   ├── cmd/llm-studio/       # Unified entry (server + bench)
│   ├── conf/llm.yaml.example
│   ├── history/
│   └── internal/
└── frontend/src/
```

## Contributing

1. Fork the repository
2. Create a feature branch (`git checkout -b feat/my-feature`)
3. Commit your changes
4. Open a Pull Request

Please do not include API keys or tokens in commits.

## License

This project is open source. Add a `LICENSE` file before publishing if you intend to distribute under a specific license (e.g. MIT).
