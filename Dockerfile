FROM golang:1.21 as builder
WORKDIR /work
COPY . .
RUN make

FROM debian:12-slim

RUN apt update \
 && apt install --yes --no-install-recommends \
    ca-certificates \
    ipmitool \
    libvirt-clients \
 # /usr/bin/sum is provided by busybox
 && rm /usr/bin/sum

# Add missing file from ipmitool
ADD https://www.iana.org/assignments/enterprise-numbers.txt /usr/share/misc/enterprise-numbers.txt 

COPY --from=builder /work/bin/metal-bmc /
COPY --from=r.metal-stack.io/metal/supermicro:2.11.0 /usr/bin/sum /usr/bin/sum

CMD ["/metal-bmc"]
