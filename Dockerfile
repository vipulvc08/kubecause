FROM golang:1.23-alpine AS builder

WORKDIR /src

COPY go.mod ./
# COPY go.sum ./
RUN go mod download

COPY . .

ARG VERSION=dev
ARG COMMIT=none
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
    go build \
      -trimpath \
      -ldflags="-s -w -X main.version=${VERSION} -X main.commit=${COMMIT}" \
      -o /out/kubecause \
      ./cmd/kubecause

FROM gcr.io/distroless/static-debian12:nonroot

COPY --from=builder /out/kubecause /kubecause

USER nonroot:nonroot
EXPOSE 8080
ENTRYPOINT ["/kubecause"]
