FROM debian:bookworm-slim

ARG TARGETARCH
ARG TARGETPLATFORM
ARG DEPOTDOWNLOADER_VERSION=3.4.0

RUN <<EOF
set -e

apt -y update

# install tools
apt -y install \
  libarchive-tools \
  squashfs-tools

# install DepotDownloader
arch="${TARGETARCH}"
if [ "${arch}" = "amd64" ]; then
  arch="x64"
fi
curl -fsSL -o /tmp/archive.tar.gz "https://github.com/SteamRE/DepotDownloader/releases/download/DepotDownloader_${DEPOTDOWNLOADER_VERSION}/DepotDownloader-linux-${arch}.zip"
mkdir -p /tmp/extract
tar -C /tmp/extract -xvzf /tmp/archive.tar.gz
mv /tmp/extract/DepotDownloader /usr/local/bin/DepotDownloader
rm -rf /tmp/archive.tar.gz /tmp/extract

rm -rf /var/lib/apt/lists/*

# configure user and directories
useradd -m -u 1000 game
mkdir -p /cache /data /game
chown -R game:game /cache /data /game
EOF

COPY $TARGETPLATFORM/seven-days-to-die /usr/local/bin/seven-days-to-die

USER game

EXPOSE 8080/tcp
EXPOSE 8081/tcp
EXPOSE 8880/tcp
EXPOSE 26900/udp
EXPOSE 26900/tcp
EXPOSE 26901/udp
EXPOSE 26902/udp
EXPOSE 26903/udp

ENTRYPOINT [ "seven-days-to-die" ]
