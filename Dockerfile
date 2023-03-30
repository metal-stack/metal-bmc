FROM golang:1.20 as builder
WORKDIR /work
COPY . .
RUN make

FROM r.metal-stack.io/metal/supermicro:2.10.0 as sum

FROM debian:11-slim

RUN apt update \
 && apt install --yes --no-install-recommends \
    ca-certificates \
    ipmitool \
    libvirt-clients \
 # /usr/bin/sum is provided by busybox
 && rm /usr/bin/sum

COPY --from=builder /work/bin/metal-bmc /
COPY --from=sum /usr/bin/sum /usr/bin/sum

CMD ["/metal-bmc"]
