#!/bin/sh
set -e

apt -y update
DEBIAN_FRONTEND=noninteractive apt -y install make

BIN=/usr/local/bin make install-tools
make install-docs
