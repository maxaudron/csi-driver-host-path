FROM golang:buster AS builder
COPY . /work
WORKDIR /work
RUN make

FROM debian:buster
RUN echo "deb http://deb.debian.org/debian buster-backports main contrib\
deb-src http://deb.debian.org/debian buster-backports main contrib\
" >> /etc/apt/sources.list.d/buster-backports.list && \
    echo "Package: libnvpair1linux libuutil1linux libzfs2linux libzpool2linux spl-dkms zfs-dkms zfs-test zfsutils-linux zfsutils-linux-dev zfs-zed\
Pin: release n=buster-backports\
Pin-Priority: 990\
    " >> /etc/apt/preferences.d/90_zfs
RUN apt-get update && apt-get -y install zfsutils-linux zfs-dkms-
RUN apt-get clean && rm -rf /var/lib/apt/lists/*

COPY --from=builder /work/bin/zfsplugin /usr/local/bin/

ENTRYPOINT ["/usr/local/bin/zfsplugin"]
EXPOSE 7676 7777
