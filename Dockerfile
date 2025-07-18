FROM golang:1.24 AS builder
WORKDIR /work
COPY . .
RUN make

# we must stay at debian-11 because otherwise ipmitool v1.18.19 will be installed which is broken
# see comment below
FROM debian:11-slim

RUN apt update \
 && apt install --yes --no-install-recommends \
    ca-certificates \
    ipmitool \
    libvirt-clients \
 # /usr/bin/sum is provided by busybox
 && rm /usr/bin/sum

# Add missing file from ipmitool debian packaging
# see https://github.com/ipmitool/ipmitool/issues/377
# see https://groups.google.com/g/linux.debian.bugs.dist/c/ukUAcfnm280
# This file is only required in ipmitool v1.18.19, debian-11 still is at v1.18.18 which works fine
# ADD https://www.iana.org/assignments/enterprise-numbers.txt /usr/share/misc/enterprise-numbers.txt 

COPY --from=builder /work/bin/metal-bmc /
COPY --from=r.metal-stack.io/metal/supermicro:2.14.0 /usr/bin/sum /usr/bin/sum

CMD ["/metal-bmc"]
