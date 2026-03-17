FROM debian:bookworm-slim

ARG TARGETPLATFORM

RUN <<EOF
set -e

apt -y update
apt -y install make
rm -rf /var/lib/apt/lists/*

useradd -m -u 1000 game
mkdir -p /cache /data /game
chown -R game:game /cache /data /game
EOF

ADD Makefile Makefile
ADD Makefile.include.mk Makefile.include.mk
RUN <<EOF
set -e

make install-tools__DepotDownloader
rm Makefile Makefile.include.mk
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
