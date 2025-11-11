FROM debian:stable-slim
LABEL maintainer="Greg Drake <greg@madmallards.com>"

# Install runtime dependencies and clean up afterwards to keep the image small.
RUN apt-get update \
    && apt-get install -y --no-install-recommends ca-certificates \
    && rm -rf /var/lib/apt/lists/*

COPY rlmlm_exporter /usr/local/bin/rlmlm_exporter

EXPOSE 9319
USER nobody
ENTRYPOINT [ "/usr/local/bin/rlmlm_exporter" ]
