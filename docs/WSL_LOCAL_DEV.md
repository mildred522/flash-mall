# WSL Local Development

This project should run from the WSL ext4 filesystem, not from `/mnt/c` or
`/mnt/d`, when using the Linux Docker Engine.

Recommended workspace:

```sh
cd /home/mildred/code/flash-mall
```

Avoid running the Go services from paths like `/mnt/d/codeset/...`; that keeps
file IO on the Windows filesystem and brings back the performance problems this
migration is meant to remove.

## Toolchain

Expected local tools inside WSL:

- Go `1.24.11`
- Node.js `20.x` and npm
- Docker Engine with the compose and buildx plugins. `bootstrap-wsl.sh` can install the
  compose/buildx plugins into `~/.docker/cli-plugins`, but it does not install the Docker
  Engine itself.
- `goctl` `v1.9.2`
- `protoc`
- `protoc-gen-go` `v1.36.10`
- `protoc-gen-go-grpc` `v1.5.1`
- basic Unix tools: `git`, `curl`, `tar`, `gzip`, `unzip`

The migrated workspace currently uses:

```sh
/home/mildred/.local/go/bin/go
/usr/bin/docker
```

Install or repair the local user-level toolchain:

```sh
scripts/local/bootstrap-wsl.sh
```

Check prerequisites without building:

```sh
scripts/local/check-build-prereqs.sh
```

`bootstrap-wsl.sh` installs Go, Node, Docker Compose, `goctl`, `protoc`,
protobuf Go plugins, downloads Go modules, and runs `npm ci` for both `frontend`
and `web`. It does not need sudo. To skip npm dependency installation:

```sh
FLASH_MALL_BOOTSTRAP_SKIP_NPM=1 scripts/local/bootstrap-wsl.sh
```

To also pull compose dependency images:

```sh
FLASH_MALL_BOOTSTRAP_PULL_IMAGES=1 scripts/local/bootstrap-wsl.sh
```

Image pulls are best-effort and time-limited per image. Override the timeout
when the network is slow:

```sh
FLASH_MALL_BOOTSTRAP_PULL_IMAGES=1 FLASH_MALL_BOOTSTRAP_PULL_TIMEOUT_SECONDS=600 scripts/local/bootstrap-wsl.sh
```

Optional local overrides live in `deploy/.env`:

```sh
cp deploy/.env.example deploy/.env
```

## Start The Container Stack

Build Go services with the WSL Go toolchain, package them as local images, and
start the full compose stack:

```sh
scripts/local/start-compose-all.sh
```

The script waits for `entry-api` to report `overall=true` and verifies the Redis
stock seed keys used by the flash-sale order flow. If startup fails, it prints
compose status and key container logs before exiting.

If dependency images are missing in the WSL Docker Engine, pull them first:

```sh
scripts/local/start-compose-all.sh --pull-deps
```

Start without rebuilding service images:

```sh
scripts/local/start-compose-all.sh --no-build
```

Start without waiting for the application health endpoint:

```sh
scripts/local/start-compose-all.sh --no-wait
```

Run in the foreground:

```sh
scripts/local/start-compose-all.sh --foreground
```

Run only the health probe against the current stack:

```sh
scripts/local/health-compose.sh --wait 30 --logs-on-failure
```

From Windows PowerShell, call the WSL wrapper instead of running the Windows
compose path:

```powershell
scripts\local\start-wsl-compose.ps1 -NoBuild
```

The GUI launcher also exposes this path. From Windows PowerShell:

```powershell
pwsh -NoProfile -ExecutionPolicy Bypass -File scripts\local\launcher.ps1
```

Use the `WSL 启动` button for the migrated WSL compose stack. The older
`快速启动` and `完整启动` buttons still target the Windows exe workflow.

## Stop The Container Stack

```sh
scripts/local/stop-compose-all.sh
```

Remove local MySQL data as well:

```sh
scripts/local/stop-compose-all.sh --volumes
```

From Windows PowerShell:

```powershell
scripts\local\stop-wsl-compose.ps1
```

In the GUI launcher, use `WSL 停止` for the compose stack running inside WSL.

## Local Endpoints

- entry-api: `http://127.0.0.1:8888`
- auth-api: `http://127.0.0.1:8890`
- RabbitMQ management: `http://127.0.0.1:15672`

Use `127.0.0.1` from Windows as well. Do not bookmark the WSL `172.x.x.x`
address; it is an internal VM address and can change.

## Local K8s

K8s should run inside WSL as well. The recommended local chain is:

```text
WSL workspace -> WSL Docker Engine -> kind -> kubectl -> Windows browser
```

Build local images for K8s:

```sh
scripts/k8s/build-images.sh dev
```

Create a kind cluster, load images, apply manifests, and wait for core services:

```sh
scripts/k8s/dev-up.sh
```

The default K8s profile is `local`: one replica per Go service for the WSL
single-node kind cluster. Use `--profile demo` when you specifically want the
multi-replica demo behavior.

Rebuild images first:

```sh
scripts/k8s/dev-up.sh --rebuild-images
```

Apply manifests to an existing cluster:

```sh
scripts/k8s/apply.sh
```

Restore entry-api forwarding to Windows/WSL localhost:

```sh
scripts/k8s/restore-port-forward.sh
```

Check pods, services, entry-api health, MySQL stock buckets, and optional Redis stock keys:

```sh
scripts/k8s/health.sh
```

Recover after WSL/Docker restart, Unknown pods, or lost port-forward:

```sh
scripts/k8s/dev-restart.sh
```

Stop the local kind cluster:

```sh
scripts/k8s/dev-down.sh
```

The older `scripts/k8s/*.ps1` files are Windows-compatible entry points. Prefer
the `.sh` scripts for WSL development so image builds, kind, and kubectl all use
the same Linux Docker context.

## Notes

The PowerShell scripts under `scripts/local/*.ps1` are kept for Windows. Use the
`.sh` scripts from WSL so the Go build cache, Docker context, and filesystem IO
stay inside Linux.

The `start-wsl-compose.ps1` and `stop-wsl-compose.ps1` wrappers are the
exception: they are Windows entry points that immediately delegate into the WSL
workspace.

The migrated WSL worktree also keeps the split stash checkpoints. They can be
listed with:

```sh
git stash list
```

Restore them in this order when rebuilding the same in-progress state from a
clean clone:

```sh
git stash apply stash@{2}
git stash apply stash@{1}
git stash apply stash@{0}
git stash apply stash@{5}
git stash apply stash@{3}
git stash apply stash@{4}
```
