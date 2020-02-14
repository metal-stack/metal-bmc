FROM metalstack/builder:latest as builder

FROM registry.fi-ts.io/metal/supermicro:2.4.0 as sum
FROM debian:10

RUN apt update \
 && apt install --yes --no-install-recommends \
    ca-certificates
COPY --from=builder /work/bin/ipmi-catcher /
COPY --from=sum /usr/bin/sum /sum

CMD ["/ipmi-catcher"]
