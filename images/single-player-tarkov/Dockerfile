FROM debian:bookworm-slim AS dotnet_downloader

ARG TARGETARCH
ARG DOTNET_VERSION=9.0.312

COPY scripts/install-dotnet.sh /usr/local/bin/install-dotnet.sh
RUN /usr/local/bin/install-dotnet.sh "${DOTNET_VERSION}" /usr/share/dotnet

FROM debian:bookworm-slim

ARG TARGETPLATFORM

COPY --from=dotnet_downloader /usr/share/dotnet /usr/share/dotnet

RUN <<EOF
set -e

export DEBIAN_FRONTEND=noninteractive
apt -y update
apt -y install curl
curl -fsSL https://deb.nodesource.com/setup_20.x | bash -
apt -y update
apt -y install \
  libarchive-tools \
  squashfs-tools \
  nodejs
rm -rf /var/lib/apt/lists/*

ln -s /usr/share/dotnet/dotnet /usr/local/bin/dotnet

useradd -m -u 1000 game
mkdir -p /cache /data /game
chown -R game:game /cache /data /game
EOF

COPY $TARGETPLATFORM/single-player-tarkov /usr/local/bin/single-player-tarkov

USER game

EXPOSE 6969/tcp
EXPOSE 8880/tcp

ENTRYPOINT [ "single-player-tarkov" ]
