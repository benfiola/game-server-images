FROM debian:bookworm-slim

ARG TARGETPLATFORM
ARG TARGETARCH

RUN <<EOF
set -e

apt update

apt -y install \
  libarchive-tools \
  squashfs-tools

apt install -y curl
curl -fsSL https://deb.nodesource.com/setup_20.x | bash -
apt install -y nodejs

apt install -y \
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
curl -fsSL -o /tmp/archive.tar.gz "https://builds.dotnet.microsoft.com/dotnet/Sdk/9.0.312/dotnet-sdk-9.0.312-linux-${arch}.tar.gz"
mkdir -p /usr/share/dotnet
tar -C /usr/share/dotnet -xvzf /tmp/archive.tar.gz
rm -rf /tmp/archive.tar.gz
ln -s /usr/share/dotnet/dotnet /usr/local/bin/dotnet

apt-get clean
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
