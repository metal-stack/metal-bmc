FROM metalstack/builder:latest as builder

FROM r.metal-stack.io/metal/supermicro:2.8.1 as sum

FROM debian:11-slim

RUN apt update \
 && apt install --yes --no-install-recommends \
    ca-certificates \
    ipmitool \
    libvirt-clients \
 # /usr/bin/sum is provided by busybox
 && rm /usr/bin/sum

COPY --from=builder /work/bin/bmc-catcher /
COPY --from=sum /usr/bin/sum /usr/bin/sum

CMD ["/bmc-catcher"]
