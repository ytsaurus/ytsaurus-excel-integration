FROM mirror.gcr.io/ubuntu:noble

USER root

WORKDIR /tmp

COPY ./excel-exporter /usr/bin/excel-exporter

# CREDITS files with licenses for all binaries.
COPY ./credits/excel-exporter.CREDITS /usr/bin/excel-exporter.CREDITS

RUN apt update && DEBIAN_FRONTEND=noninteractive TZ=Etc/UTC apt install -y \
  curl \
  less \
  gdb \
  lsof \
  strace \
  telnet \
  dnsutils \
  iputils-ping \
  iperf \
  iproute2 \
  jq \
  netcat-openbsd \
  tcpdump \
  wget
