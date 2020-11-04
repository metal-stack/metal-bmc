FROM metalstack/builder:latest as builder

FROM debian:10

RUN apt update \
 && apt install --yes --no-install-recommends \
    ca-certificates
COPY --from=builder /work/bin/bmc-catcher /

CMD ["/bmc-catcher"]
