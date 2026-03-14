FROM debian:bookworm-slim

RUN <<EOF
apt -y update
apt -y install make
EOF

ADD Makefile Makefile
ADD Makefile.include.mk Makefile.include.mk

RUN <<EOF
make install-tools__DepotDownloader
rm Makefile Makefile.include.mk
EOF

COPY $TARGETPLATFORM/seven-days-to-die /usr/local/bin/seven-days-to-die

ENTRYPOINT [ "seven-days-to-die" ]
