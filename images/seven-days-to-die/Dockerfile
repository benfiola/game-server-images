FROM debian:bookworm-slim

RUN <<EOF
apt -y update
apt -y install make
rm -rf /var/lib/apt/lists/*

useradd -m -u 1000 sdtd
mkdir -p /cache /data /sdtd
chown -R sdtd:sdtd /cache /data /sdtd
EOF

ADD Makefile Makefile
ADD Makefile.include.mk Makefile.include.mk
RUN <<EOF
make install-tools__DepotDownloader
rm Makefile Makefile.include.mk
EOF

COPY $TARGETPLATFORM/seven-days-to-die /usr/local/bin/seven-days-to-die

USER sdtd

ENTRYPOINT [ "seven-days-to-die" ]
