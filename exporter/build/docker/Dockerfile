FROM mirror.gcr.io/ubuntu:noble

USER root

WORKDIR /tmp

COPY ./excel-exporter /usr/bin/excel-exporter

# CREDITS files with licenses for all binaries.
COPY ./credits/excel-exporter.CREDITS /usr/bin/excel-exporter.CREDITS

# Install debug tools
RUN <<-EOT
#!/bin/bash
set -eux -o pipefail

export DEBIAN_FRONTEND=noninteractive
export TZ=Etc/UTC

apt-get update -q --error-on=any

packages=(
  curl
  dnsutils
  gdb
  google-perftools
  htop
  iperf
  iproute2
  iputils-ping
  jq
  less
  lsb-release
  lsof
  ncdu
  netcat-openbsd
  procps
  python3
  python3-pip
  strace
  tcpdump
  telnet
  tini
  unzip
  vim
  wget
  zstd
)

apt-get install -y --no-install-recommends "${packages[@]}"

# Clean up to reduce image size
apt-get clean
rm -rf /var/lib/apt/lists/*

EOT
