#!/usr/bin/env bash
set -euo pipefail

volume="${FLASH_MALL_UPLOAD_VOLUME:-flash-mall-uploads}"
repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"

if (($# == 0)); then
  set -- "${repo_root}/.runtime/uploads"
fi

docker volume create "${volume}" >/dev/null

copied_any=false
for source_dir in "$@"; do
  if [[ ! -d "${source_dir}" ]]; then
    echo "skip missing upload source: ${source_dir}"
    continue
  fi
  source_dir="$(cd "${source_dir}" && pwd)"
  echo "migrate uploads: ${source_dir} -> volume ${volume}"
  docker run --rm --entrypoint sh \
    -v "${volume}:/target" \
    -v "${source_dir}:/source:ro" \
    redis:latest -euc '
      cd /source
      find . -type f -print > /tmp/flash-mall-upload-files
      while IFS= read -r file; do
        destination="/target/${file#./}"
        mkdir -p "$(dirname "$destination")"
        if [ -f "$destination" ]; then
          source_hash="$(sha256sum "$file" | cut -d " " -f 1)"
          destination_hash="$(sha256sum "$destination" | cut -d " " -f 1)"
          if [ "$source_hash" != "$destination_hash" ]; then
            echo "upload collision with different content: ${file#./}" >&2
            exit 1
          fi
          continue
        fi
        cp "$file" "$destination"
        chmod 0644 "$destination"
      done < /tmp/flash-mall-upload-files
    '
  copied_any=true
done

if [[ "${copied_any}" != true ]]; then
  echo "no upload source was migrated" >&2
  exit 1
fi

docker run --rm --entrypoint sh -v "${volume}:/target:ro" redis:latest \
  -euc 'find /target -type f -print | sort; echo "files=$(find /target -type f | wc -l)"'
