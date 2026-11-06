FROM docker.io/library/archlinux:base-devel
RUN pacman -Syu --noconfirm --needed go ldc curl sqlite libnotify dbus systemd pkgconf rpm-tools \
    && pacman -Scc --noconfirm \
    && useradd -m -u 1000 builder \
    && printf 'MAKEFLAGS="-j3"\n' >> /etc/makepkg.conf
USER builder
WORKDIR /work
ENV HOME=/home/builder
