# we must stay with ipmitool v1.8.18, newer version will break behavior
# see comment below
FROM debian:13-slim

COPY bullseye.sources /etc/apt/sources.list.d/
RUN apt update \
 && apt install --yes --no-install-recommends \
    ca-certificates \
    ipmitool=1.8.18-10.1 \
    libvirt-clients \
 # /usr/bin/sum is provided by busybox
 && rm /usr/bin/sum

# Add missing file from ipmitool debian packaging
# see https://github.com/ipmitool/ipmitool/issues/377
# see https://groups.google.com/g/linux.debian.bugs.dist/c/ukUAcfnm280
# This file is only required in ipmitool v1.8.19, debian-11 still is at v1.8.18 which works fine
# ADD https://www.iana.org/assignments/enterprise-numbers.txt /usr/share/misc/enterprise-numbers.txt

COPY bin/metal-bmc /
COPY --from=r.metal-stack.io/metal/supermicro:2.14.0 /usr/bin/sum /usr/bin/sum

CMD ["/metal-bmc"]
