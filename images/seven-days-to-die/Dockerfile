FROM debian:bookworm-slim AS depotdownloader_builder

ARG TARGETARCH
ARG DEPOTDOWNLOADER_VERSION=3.4.0

RUN <<EOF
set -e

apt -y update
apt -y install \
  curl \
  unzip
arch="${TARGETARCH}"
if [ "${arch}" = "amd64" ]; then
  arch="x64"
fi
echo "https://github.com/SteamRE/DepotDownloader/releases/download/DepotDownloader_${DEPOTDOWNLOADER_VERSION}/DepotDownloader-linux-${arch}.zip"
curl -fsSL -o /tmp/archive.zip "https://github.com/SteamRE/DepotDownloader/releases/download/DepotDownloader_${DEPOTDOWNLOADER_VERSION}/DepotDownloader-linux-${arch}.zip"
mkdir -p /tmp/extract
unzip -d /tmp/extract /tmp/archive.zip
mv /tmp/extract/DepotDownloader /usr/local/bin/DepotDownloader
rm -rf /tmp/archive.zip /tmp/extract
EOF

FROM debian:bookworm-slim

ARG TARGETPLATFORM

COPY --from=depotdownloader_builder /usr/local/bin/DepotDownloader /usr/local/bin/DepotDownloader

RUN <<EOF
set -e

apt -y update
apt -y install \
  libarchive-tools \
  squashfs-tools
rm -rf /var/lib/apt/lists/*

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
