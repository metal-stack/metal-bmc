FROM metalstack/builder:latest as builder

RUN apt update \
 && apt install --yes --no-install-recommends \
    ca-certificates
COPY --from=builder /work/bin/bmc-catcher /

CMD ["/bmc-catcher"]
