FROM scratch

# Copy pre-compiled Linux binary (build with: GOOS=linux CGO_ENABLED=0 go build ...)
COPY bin/user-service-linux /app/user-service

# Default config (override via env vars at runtime)
COPY apps/user/user-service/config.yml /app/config.yml

# Default runtime config (override with -e at docker run)
ENV LISTEN_ADDR=0.0.0.0:50051
ENV ETCD_ENDPOINTS=host.docker.internal:2379

EXPOSE 50051

ENTRYPOINT ["/app/user-service"]
CMD ["-conf", "/app/config.yml"]
