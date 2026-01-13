FROM ubuntu:24.04

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

USER root

COPY ./excel-exporter /usr/bin/excel-exporter
COPY ./credits/excel-exporter.CREDITS /usr/bin/excel-exporter.CREDITS
