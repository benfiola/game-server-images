#!/usr/bin/env bash
set -e

DOTNET_VERSION="${1}"
OUTPUT_PATH="${2}"
if [ -z "${DOTNET_VERSION}" ] || [ -z "${OUTPUT_PATH}" ]; then
  echo "Usage: ${0} <version> <output-path>" >&2
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
  curl \
  libc6 \
  libgcc-s1 \
  libgssapi-krb5-2 \
  libicu72 \
  libssl3 \
  libstdc++6 \
  zlib1g

ARCHIVE_PATH="${TEMP_DIR}/archive.tar.gz"
curl -fsSL -o "${ARCHIVE_PATH}" "https://builds.dotnet.microsoft.com/dotnet/Sdk/${DOTNET_VERSION}/dotnet-sdk-${DOTNET_VERSION}-linux-${ARCH}.tar.gz"
mkdir -p "${OUTPUT_PATH}"
tar -C "${OUTPUT_PATH}" -xzf "${ARCHIVE_PATH}"
