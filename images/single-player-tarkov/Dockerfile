FROM debian:bookworm-slim AS dotnet_downloader

ARG TARGETARCH
ARG DOTNET_VERSION=9.0.312

RUN <<EOF
set -e

apt -y update
apt install -y \
  curl \
  libc6 \
  libgcc-s1 \
  libgssapi-krb5-2 \
  libicu72 \
  libssl3 \
  libstdc++6 \
  zlib1g
arch="${TARGETARCH}"
if [ "${arch}" = "amd64" ]; then
  arch="x64"
fi
curl -fsSL -o /tmp/archive.tar.gz "https://builds.dotnet.microsoft.com/dotnet/Sdk/${DOTNET_VERSION}/dotnet-sdk-${DOTNET_VERSION}-linux-${arch}.tar.gz"
mkdir -p /usr/share/dotnet
tar -C /usr/share/dotnet -xzf /tmp/archive.tar.gz
rm -rf /tmp/archive.tar.gz
EOF

FROM debian:bookworm-slim

ARG TARGETPLATFORM

COPY --from=dotnet_downloader /usr/share/dotnet /usr/share/dotnet

RUN <<EOF
set -e

ln -s /usr/share/dotnet/dotnet /usr/local/bin/dotnet

apt -y update
apt -y install curl
curl -fsSL https://deb.nodesource.com/setup_20.x | bash -
apt -y update
apt -y install \
  libarchive-tools \
  squashfs-tools \
  nodejs
rm -rf /var/lib/apt/lists/*

useradd -m -u 1000 game
mkdir -p /cache /data /game
chown -R game:game /cache /data /game
EOF

COPY $TARGETPLATFORM/single-player-tarkov /usr/local/bin/single-player-tarkov

USER game

EXPOSE 6969/tcp
EXPOSE 8880/tcp

ENTRYPOINT [ "single-player-tarkov" ]
