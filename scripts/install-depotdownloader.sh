#!/usr/bin/env bash
set -e

DEPOTDOWNLOADER_VERSION="3.4.0"

OUTPUT_PATH="$1"
if [ -z "${OUTPUT_PATH}" ]; then
  echo "Usage: $0 <output-path>" >&2
  exit 1
fi

TEMP_DIR=$(mktemp -d)
trap 'rm -rf "$TEMP_DIR"' EXIT

ARCH="$(dpkg --print-architecture)"
if [ "${ARCH}" = "amd64" ]; then
  ARCH="x64"
fi

export DEBIAN_FRONTEND=noninteractive
apt -y update
apt -y install \
  ca-certificates \
  curl \
  libssl3 \
  unzip

ARCHIVE_PATH="${TEMP_DIR}/archive.zip"
EXTRACT_PATH="${TEMP_DIR}/extract"
curl -fsSL -o "${ARCHIVE_PATH}" "https://github.com/SteamRE/DepotDownloader/releases/download/DepotDownloader_${DEPOTDOWNLOADER_VERSION}/DepotDownloader-linux-${ARCH}.zip"
mkdir -p "${EXTRACT_PATH}"
unzip -d "${EXTRACT_PATH}" "${ARCHIVE_PATH}"
mv "${EXTRACT_PATH}/DepotDownloader" "${OUTPUT_PATH}"
