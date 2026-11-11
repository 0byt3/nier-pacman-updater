FROM docker.io/library/archlinux:base-devel
RUN pacman -Syu --noconfirm --needed go lua ldc curl sqlite libnotify dbus systemd pkgconf rpm-tools cmake avahi openssl libplist gstreamer gst-plugins-base libx11 \
    && pacman -Scc --noconfirm \
    && useradd -m -u 1000 builder \
    && printf 'MAKEFLAGS="-j3"\n' >> /etc/makepkg.conf
USER builder
WORKDIR /work
ENV HOME=/home/builder
