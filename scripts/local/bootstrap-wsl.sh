#!/usr/bin/env sh
set -eu

go_version="1.24.11"
go_sha256="bceca00afaac856bc48b4cc33db7cd9eb383c81811379faed3bdbc80edb0af65"
goctl_version="v1.9.2"
protoc_version="27.3"
protoc_gen_go_version="v1.36.10"
protoc_gen_go_grpc_version="v1.5.1"

home_dir="${HOME:-/home/mildred}"
local_dir="$home_dir/.local"
bin_dir="$local_dir/bin"
cache_dir="$home_dir/.cache/flash-mall-bootstrap"
docker_cli_plugins_dir="$home_dir/.docker/cli-plugins"

mkdir -p "$local_dir" "$bin_dir" "$cache_dir" "$docker_cli_plugins_dir"

path_line="export PATH=\"$local_dir/go/bin:$home_dir/go/bin:$bin_dir:\$PATH\""
for profile in "$home_dir/.profile" "$home_dir/.bashrc"; do
  touch "$profile"
  if ! grep -Fq "$local_dir/go/bin" "$profile"; then
    {
      echo
      echo "# flash-mall local toolchain"
      echo "$path_line"
    } >> "$profile"
  fi
done

export PATH="$local_dir/go/bin:$home_dir/go/bin:$bin_dir:$PATH"

download() {
  url="$1"
  out="$2"
  if [ ! -f "$out" ]; then
    echo "[DOWNLOAD] $url"
    curl -fL --retry 3 --connect-timeout 20 -o "$out" "$url"
  fi
}

install_go() {
  if command -v go >/dev/null 2>&1 && [ "$(go version | awk '{print $3}')" = "go$go_version" ]; then
    echo "[OK] go $go_version"
    return
  fi

  archive="go${go_version}.linux-amd64.tar.gz"
  download "https://dl.google.com/go/$archive" "$cache_dir/$archive"
  echo "$go_sha256  $cache_dir/$archive" | sha256sum -c -
  rm -rf "$local_dir/go$go_version" "$local_dir/go"
  tar -C "$local_dir" -xzf "$cache_dir/$archive"
  mv "$local_dir/go" "$local_dir/go$go_version"
  ln -s "$local_dir/go$go_version" "$local_dir/go"
  echo "[OK] installed go $go_version"
}

latest_node20() {
  curl -fsSL https://nodejs.org/dist/index.tab |
    awk 'NR > 1 && $1 ~ /^v20[.]/ {print $1; exit}'
}

install_node() {
  if command -v node >/dev/null 2>&1 && [ "$(node --version | sed 's/^v//' | cut -d. -f1)" = "20" ]; then
    echo "[OK] node $(node --version)"
    return
  fi

  node_version="$(latest_node20)"
  if [ -z "$node_version" ]; then
    echo "could not resolve latest Node 20 version" >&2
    exit 1
  fi

  archive="node-${node_version}-linux-x64.tar.xz"
  download "https://nodejs.org/dist/${node_version}/${archive}" "$cache_dir/$archive"
  rm -rf "$local_dir/node-$node_version" "$local_dir/node"
  tar -C "$local_dir" -xJf "$cache_dir/$archive"
  mv "$local_dir/node-${node_version}-linux-x64" "$local_dir/node-$node_version"
  ln -s "$local_dir/node-$node_version" "$local_dir/node"
  ln -sf "$local_dir/node/bin/node" "$bin_dir/node"
  ln -sf "$local_dir/node/bin/npm" "$bin_dir/npm"
  ln -sf "$local_dir/node/bin/npx" "$bin_dir/npx"
  echo "[OK] installed node $node_version"
}

install_go_tools() {
  echo "[GO INSTALL] goctl $goctl_version"
  go install "github.com/zeromicro/go-zero/tools/goctl@$goctl_version"
  echo "[GO INSTALL] protoc-gen-go $protoc_gen_go_version"
  go install "google.golang.org/protobuf/cmd/protoc-gen-go@$protoc_gen_go_version"
  echo "[GO INSTALL] protoc-gen-go-grpc $protoc_gen_go_grpc_version"
  go install "google.golang.org/grpc/cmd/protoc-gen-go-grpc@$protoc_gen_go_grpc_version"
}

download_go_modules() {
  repo_root=$(CDPATH= cd -- "$(dirname -- "$0")/../.." && pwd)
  echo "[GO MOD] download"
  (cd "$repo_root" && go mod download)
}

install_protoc() {
  if command -v protoc >/dev/null 2>&1; then
    echo "[OK] protoc $(protoc --version)"
    return
  fi

  archive="protoc-${protoc_version}-linux-x86_64.zip"
  download "https://github.com/protocolbuffers/protobuf/releases/download/v${protoc_version}/${archive}" "$cache_dir/$archive"
  rm -rf "$local_dir/protoc-$protoc_version"
  mkdir -p "$local_dir/protoc-$protoc_version"
  unzip -q "$cache_dir/$archive" -d "$local_dir/protoc-$protoc_version"
  ln -sf "$local_dir/protoc-$protoc_version/bin/protoc" "$bin_dir/protoc"
  echo "[OK] installed protoc $protoc_version"
}

latest_compose() {
  curl -fsIL -o /dev/null -w '%{url_effective}' https://github.com/docker/compose/releases/latest |
    sed 's#.*/tag/##'
}

latest_buildx() {
  curl -fsIL -o /dev/null -w '%{url_effective}' https://github.com/docker/buildx/releases/latest |
    sed 's#.*/tag/##'
}

install_docker_compose() {
  if command -v docker >/dev/null 2>&1 && docker compose version >/dev/null 2>&1; then
    echo "[OK] docker compose $(docker compose version --short 2>/dev/null || docker compose version)"
    return
  fi

  compose_version="${FLASH_MALL_COMPOSE_VERSION:-$(latest_compose)}"
  if [ -z "$compose_version" ]; then
    echo "could not resolve latest Docker Compose version" >&2
    exit 1
  fi

  target="$docker_cli_plugins_dir/docker-compose"
  download "https://github.com/docker/compose/releases/download/${compose_version}/docker-compose-linux-x86_64" "$cache_dir/docker-compose-$compose_version"
  cp "$cache_dir/docker-compose-$compose_version" "$target"
  chmod +x "$target"
  echo "[OK] installed docker compose $compose_version"
}

install_docker_buildx() {
  if command -v docker >/dev/null 2>&1 && docker buildx version >/dev/null 2>&1; then
    echo "[OK] docker buildx $(docker buildx version | awk '{print $2}')"
    return
  fi

  buildx_version="${FLASH_MALL_BUILDX_VERSION:-$(latest_buildx)}"
  if [ -z "$buildx_version" ]; then
    echo "could not resolve latest Docker Buildx version" >&2
    exit 1
  fi

  target="$docker_cli_plugins_dir/docker-buildx"
  download "https://github.com/docker/buildx/releases/download/${buildx_version}/buildx-${buildx_version}.linux-amd64" "$cache_dir/docker-buildx-$buildx_version"
  cp "$cache_dir/docker-buildx-$buildx_version" "$target"
  chmod +x "$target"
  echo "[OK] installed docker buildx $buildx_version"
}

install_node_dependencies() {
  repo_root=$(CDPATH= cd -- "$(dirname -- "$0")/../.." && pwd)
  if [ -f "$repo_root/frontend/package-lock.json" ]; then
    echo "[NPM CI] frontend"
    npm ci --prefix "$repo_root/frontend"
  fi
  if [ -f "$repo_root/web/package-lock.json" ]; then
    echo "[NPM CI] web"
    npm ci --prefix "$repo_root/web"
  fi
}

pull_compose_images() {
  pull_timeout="${FLASH_MALL_BOOTSTRAP_PULL_TIMEOUT_SECONDS:-180}"
  echo "[DOCKER PULL] dependency images"
  for image in \
    yedf/dtm \
    bitnamilegacy/etcd:3.5 \
    mysql:5.7 \
    redis:latest \
    rabbitmq:3.12-management \
    jaegertracing/all-in-one:1.57 \
    alpine:3.20
  do
    echo "[DOCKER PULL] $image"
    if ! timeout "$pull_timeout" docker pull "$image"; then
      echo "[WARN] docker pull timed out or failed: $image" >&2
    fi
  done
}

install_go
install_node
install_protoc
install_docker_compose
install_docker_buildx
install_go_tools
download_go_modules

if [ "${FLASH_MALL_BOOTSTRAP_SKIP_NPM:-0}" != "1" ]; then
  install_node_dependencies
fi

if [ "${FLASH_MALL_BOOTSTRAP_PULL_IMAGES:-0}" = "1" ]; then
  pull_compose_images
fi

echo
echo "Bootstrap complete. Open a new shell or run:"
echo "  export PATH=\"$local_dir/go/bin:$home_dir/go/bin:$bin_dir:\$PATH\""
echo
echo "Check with:"
echo "  scripts/local/check-build-prereqs.sh"
