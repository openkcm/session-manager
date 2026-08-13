# Local Development

How to run and debug session-manager **directly on your machine** (binary on
the host, backing services in Docker). This is the fast inner-loop workflow: you
can set breakpoints and restart in seconds.

> This is **not** the k3d/Helm deployment (`make start`) documented in the
> README. Use that when you need to test the containerized service end-to-end;
> use this when you're developing and debugging the Go code.

## Prerequisites

- Go (see `go.mod` for the version)
- Docker (with Compose v2 — `docker compose`, not `docker-compose`)
- [`buf`](https://buf.build/docs/installation) — for gRPC calls (`grpcurl` does
  **not** work here; see [gRPC](#grpc-buf-not-grpcurl))
- Optional, for step debugging: [`dlv`](https://github.com/go-delve/delve)
  (`go install github.com/go-delve/delve/cmd/dlv@latest`)

## Quick start

```sh
make dev-deps     # start Postgres, Valkey, Dex (waits until healthy)
make migrate      # build + apply DB migrations
make run          # build + run the api-server in the foreground
```

`make run` blocks and streams logs. Open a second terminal for requests. Stop it
with `Ctrl-C`; stop the dependencies with `make dev-deps-down`.

Run `make help` to list all the dev targets.

## What runs where

| Component  | Where           | Address                     | Credentials                     |
|------------|-----------------|-----------------------------|---------------------------------|
| api-server | host (binary)   | REST `:8080`, gRPC `:9091`, status `:8888` | — |
| Postgres   | Docker Compose  | `localhost:5432`            | `postgres` / `secret`, db `session_manager` |
| Valkey     | Docker Compose  | `localhost:6379`            | password `secret`               |
| Dex (IdP)  | Docker Compose  | `localhost:5556`            | user `admin@example.com` / `password`; client `my-client` / `secret` |

Config comes from `./config.yaml` (search order: `/etc/session-manager/`, then
`$HOME/.session-manager/`, then `./`). It already points at these localhost
addresses with the credentials above. The compose stack lives in
[`dev/docker-compose.yaml`](../dev/docker-compose.yaml); the Dex config is in
[`dev/dex/config.yaml`](../dev/dex/config.yaml).

## The CLI

The binary has four subcommands:

| Subcommand    | Purpose                                             |
|---------------|-----------------------------------------------------|
| `api-server`  | Runs the REST + gRPC servers (this is what you run) |
| `migrate`     | Applies DB migrations (embedded, via goose)         |
| `housekeeper` | Periodic session cleanup + token refresh            |
| `version`     | Prints build info                                   |

## Health checks

```sh
curl localhost:8888/probe/liveness      # process is alive
curl localhost:8888/probe/readiness     # {"status":"up"} => DB + Valkey reachable
curl localhost:8888/version
```

Note the `/probe/` prefix — there is no `/healthz`.

## Sending requests

### gRPC (`buf`, not `grpcurl`)

`grpcurl` cannot introspect these services — the protos use proto `edition 2024`,
which grpcurl's runtime rejects (`EDITION_2024 not yet supported`). Use
`buf curl`, which works over server reflection with no local proto files:

```sh
# List every method
buf curl --protocol grpc --http2-prior-knowledge http://localhost:9091 --list-methods

# Register a tenant's trust mapping (REQUIRED before /sm/auth works for it)
buf curl --protocol grpc --http2-prior-knowledge \
  -d '{"tenant_id":"demo","oidc":{"issuer":"http://localhost:5556/dex","client_id":"my-client","audiences":["my-client"]}}' \
  http://localhost:9091/kms.api.cmk.sessionmanager.trustmapping.v1.Service/ApplyTrustMapping

# Read it back
buf curl --protocol grpc --http2-prior-knowledge -d '{"tenant_id":"demo"}' \
  http://localhost:9091/kms.api.cmk.sessionmanager.session.v1.Service/GetTrust

# Validate a session (session_id comes from the login flow below)
buf curl --protocol grpc --http2-prior-knowledge -d '{"session_id":"<id>","tenant_id":"demo"}' \
  http://localhost:9091/kms.api.cmk.sessionmanager.session.v1.Service/GetSession
```

### REST login flow

`/sm/auth` requires a **trust mapping** for the tenant (create one with
`ApplyTrustMapping` above) — otherwise it returns 404. Once seeded:

```sh
curl -i "http://localhost:8080/sm/auth?tenant_id=demo&request_uri=http://localhost:8080/"
```

This returns a **302** to the IdP (Dex) authorization endpoint with a PKCE
challenge, plus the login-CSRF cookie. In a browser you log in at Dex and get
redirected back to `/sm/callback`, which exchanges the code for tokens and sets
the `SESSION-<tenant>` and `CSRF-<tenant>` cookies.

## End-to-end login against Dex

With `dev-deps` up and a trust mapping seeded for `demo` (pointing at
`http://localhost:5556/dex`), open this in a browser and log in as
`admin@example.com` / `password`:

```
http://localhost:8080/sm/auth?tenant_id=demo&request_uri=http://localhost:8080/
```

You'll land back on `http://localhost:8080/` with session cookies set. Feed the
`SESSION-demo` cookie value to gRPC `GetSession` to confirm it validates.

### Local http note

Dex is served over plain `http://`, which two settings in `config.yaml` enable
for local dev (both default to the hardened behavior in production):

- `sessionManager.allowHttpScheme: true` and the gRPC session service's
  `allowHttpScheme: true` — permit `http://` OIDC issuers (production: https only).
- `sessionManager.loginCSRFCookieTemplate.name: "LoginCSRF"` — the default
  `__Host-LoginCSRF` name requires the cookie's `Secure` attribute, which
  browsers reject over http. Leaving `name` unset restores the hardened default.

## Debugging

Because the binary runs on the host, you can attach a debugger with breakpoints.
All approaches below use [Delve](https://github.com/go-delve/delve)
(`go install github.com/go-delve/delve/cmd/dlv@latest`); make sure your Delve
version supports your installed Go toolchain.

### Delve (terminal)

No editor integration required:

```sh
make dev-deps
dlv debug ./cmd/session-manager -- api-server
# (dlv) break internal/session/manager.go:117
# (dlv) continue
# ... fire a request from another terminal, then: next / step / print <var>
```

Swap `api-server` for `migrate` or `housekeeper` to debug the other subcommands.

### Editor / IDE (via Delve's DAP server)

Editor-specific config files (`.vscode/`, `.dir-locals.el`, etc.) are **not**
committed — set this up locally in whatever editor you use. Delve speaks the
Debug Adapter Protocol (DAP), which every mainstream editor debug client
(VS Code, Neovim `nvim-dap`, Emacs `dape`/`dap-mode`, JetBrains GoLand's native
debugger) can drive. Point your editor's Go/Delve debug configuration at:

| Setting            | Value                                                    |
|--------------------|----------------------------------------------------------|
| debugger / adapter | `dlv dap` (Delve in DAP mode)                            |
| request            | `launch`                                                 |
| mode               | `debug` (build from source)                              |
| program            | `./cmd/session-manager` (the `main` package)             |
| args               | `["api-server"]` (or `["migrate"]` / `["housekeeper"]`)  |
| working directory  | the repository root (so the binary finds `config.yaml`)  |

To **attach** to an already-running `make run` process instead of launching a
new one, use `request: attach`, `mode: local`, and the PID from
`pgrep -f 'session-manager api-server'`.

Working examples for two popular editors follow. These files are gitignored —
copy them into your local checkout as-is.

#### VS Code — `.vscode/launch.json`

Requires the [Go extension](https://marketplace.visualstudio.com/items?itemName=golang.Go)
(which bundles Delve). `${workspaceFolder}` resolves to the repo root.

```jsonc
{
  "version": "0.2.0",
  "configurations": [
    {
      "name": "api-server",
      "type": "go",
      "request": "launch",
      "mode": "debug",
      "program": "${workspaceFolder}/cmd/session-manager",
      "cwd": "${workspaceFolder}",
      "args": ["api-server"]
    },
    {
      "name": "migrate",
      "type": "go",
      "request": "launch",
      "mode": "debug",
      "program": "${workspaceFolder}/cmd/session-manager",
      "cwd": "${workspaceFolder}",
      "args": ["migrate"]
    },
    {
      "name": "housekeeper",
      "type": "go",
      "request": "launch",
      "mode": "debug",
      "program": "${workspaceFolder}/cmd/session-manager",
      "cwd": "${workspaceFolder}",
      "args": ["housekeeper"]
    },
    {
      "name": "attach (running api-server)",
      "type": "go",
      "request": "attach",
      "mode": "local",
      "processId": "session-manager"
    }
  ]
}
```

#### Emacs — `.dir-locals.el`

Requires [`dape`](https://github.com/svaante/dape). The `eval` form registers
the launch/attach configs when you open a Go file (Emacs prompts once to mark it
safe); the project root is resolved dynamically, and `session-manager-attach`
auto-discovers the running PID via `pgrep`.

```elisp
;;; Directory Local Variables            -*- no-byte-compile: t; -*-
((go-ts-mode
  . ((eval
      . (let ((root (when-let ((d (locate-dominating-file
                                   (or (buffer-file-name) default-directory)
                                   "go.mod")))
                      (expand-file-name d))))
          (when (and root (require 'dape nil t))
            (dolist (cfg
                     `((session-manager-api-server
                        modes (go-mode go-ts-mode) ensure dape-ensure-command
                        command "dlv"
                        command-args ("dap" "--listen" "127.0.0.1::autoport")
                        command-cwd ,root port :autoport
                        :type "debug" :request "launch" :mode "debug"
                        :program ,(expand-file-name "cmd/session-manager" root)
                        :cwd ,root :args ["api-server"])
                       (session-manager-migrate
                        modes (go-mode go-ts-mode) ensure dape-ensure-command
                        command "dlv"
                        command-args ("dap" "--listen" "127.0.0.1::autoport")
                        command-cwd ,root port :autoport
                        :type "debug" :request "launch" :mode "debug"
                        :program ,(expand-file-name "cmd/session-manager" root)
                        :cwd ,root :args ["migrate"])
                       (session-manager-housekeeper
                        modes (go-mode go-ts-mode) ensure dape-ensure-command
                        command "dlv"
                        command-args ("dap" "--listen" "127.0.0.1::autoport")
                        command-cwd ,root port :autoport
                        :type "debug" :request "launch" :mode "debug"
                        :program ,(expand-file-name "cmd/session-manager" root)
                        :cwd ,root :args ["housekeeper"])
                       (session-manager-attach
                        modes (go-mode go-ts-mode) ensure dape-ensure-command
                        command "dlv"
                        command-args ("dap" "--listen" "127.0.0.1::autoport")
                        command-cwd ,root port :autoport
                        :type "debug" :request "attach" :mode "local"
                        :processId
                        ,(lambda ()
                           (let ((pids (split-string
                                        (shell-command-to-string
                                         "pgrep -f 'session-manager api-server'")
                                        nil t)))
                             (cond ((= (length pids) 1) (string-to-number (car pids)))
                                   (pids (string-to-number
                                          (completing-read "Attach to PID: " pids nil t)))
                                   (t (read-number "PID to attach: ")))))
                        :cwd ,root)))
              (setf (alist-get (car cfg) dape-configs) (cdr cfg))))))
     ;; If you use lsp-mode + golangci-lint, mirror the integration build tag:
     (go-ts-mode-build-tags . ("integration"))
     (lsp-go-env . ((GOFLAGS . "-tags=integration"))))))
```

#### Other editors

- **Neovim** — [`leoluz/nvim-dap-go`](https://github.com/leoluz/nvim-dap-go) (or
  a raw `nvim-dap` adapter running `dlv dap`) with the program/args/cwd from the
  table above.
- **JetBrains GoLand** — a *Go Build* run/debug configuration with *Run kind:
  Package*, the `cmd/session-manager` package path, and program arguments
  `api-server`; GoLand ships its own Delve integration.

### Logs

Logs are JSON. Pipe through `jq` to read them, and bump the level in
`config.yaml` (`logger.level: debug`) for detail:

```sh
make run 2>&1 | jq .
make run 2>&1 | jq 'select(.level=="ERROR")'
```

Each request carries a `requestId` you can grep on.

## Inspecting state

```sh
# Postgres
PGPASSWORD=secret psql -h localhost -U postgres -d session_manager -c '\dt'
PGPASSWORD=secret psql -h localhost -U postgres -d session_manager -c 'SELECT * FROM trust;'

# Valkey (sessions + OIDC state, prefixed session-manager:)
redis-cli -h localhost -a secret --scan --pattern 'session-manager:*'
```

## Tear down

```sh
# stop the server: Ctrl-C in its terminal (or: pkill -f "session-manager api-server")
make dev-deps-down   # stop and remove the containers
```
