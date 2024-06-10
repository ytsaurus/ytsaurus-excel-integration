FROM mirror.gcr.io/ubuntu:noble

USER root

WORKDIR /tmp

COPY ./excel-uploader /usr/bin/excel-uploader

# CREDITS files with licenses for all binaries.
COPY ./credits/excel-uploader.CREDITS /usr/bin/excel-uploader.CREDITS

RUN apt update && DEBIAN_FRONTEND=noninteractive TZ=Etc/UTC apt install -y \
  curl \
  less \
  gdb \
  lsof \
  strace \
  telnet \
  dnsutils \
  iputils-ping
