FROM golang:1.21 as builder

# Clone CoreDNS
ARG COREDNS_VERSION=v1.11.1
RUN git clone --depth 1 --branch ${COREDNS_VERSION} https://github.com/coredns/coredns.git /coredns

# Copy plugin source
COPY . /plugin

WORKDIR /coredns

# Add plugin to plugin.cfg (before kubernetes plugin)
RUN sed -i '/^kubernetes:kubernetes$/i emptyendpoints:emptyendpoints' plugin.cfg

# Add replace directive to use local plugin
RUN echo 'replace emptyendpoints => /plugin' >> go.mod

# Generate and build
RUN go generate
RUN go get emptyendpoints
RUN CGO_ENABLED=0 go build -o coredns

# Final image
FROM gcr.io/distroless/static-debian11:nonroot
COPY --from=builder /coredns/coredns /coredns
USER nonroot:nonroot
EXPOSE 53 53/udp
ENTRYPOINT ["/coredns"]
