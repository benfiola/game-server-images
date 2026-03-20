FROM debian:bookworm-slim AS depotdownloader_builder

COPY scripts/install-depotdownloader.sh /usr/local/bin/install-depotdownloader.sh
RUN  /usr/local/bin/install-depotdownloader.sh /usr/local/bin/DepotDownloader

FROM debian:bookworm-slim

ARG TARGETPLATFORM

RUN <<EOF
set -e

export DEBIAN_FRONTEND=noninteractive
apt -y update
apt -y install \
  ca-certificates \
  libarchive-tools \
  libssl3 \
  squashfs-tools
rm -rf /var/lib/apt/lists/*

useradd -m -u 1000 game
mkdir -p /cache /data /game
chown -R game:game /cache /data /game
EOF

COPY --from=depotdownloader_builder /usr/local/bin/DepotDownloader /usr/local/bin/DepotDownloader

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
