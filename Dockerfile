# Stage 1. Build the binary.
FROM golang:1.22-alpine as build

# Git is required for fetching the dependencies.
RUN apk add --no-cache git

# Certificates.
RUN apk --no-cache add ca-certificates

# Add users here, addgroup & adduser are not available in scratch.
RUN addgroup -S househunt && adduser -S -u 10000 -g househunt househunt

WORKDIR /src

# Copy go.mod/go.sum and download dependencies.
COPY ./go.mod ./go.sum ./
RUN go mod download

# Copy source code.
COPY ./ ./

# Build the binary.
RUN CGO_ENABLED=0 go build -o /out/server cmd/server/*.go

# Stage 2. Run the binary.
FROM scratch AS final

# Copy binary 
COPY --from=build /out/server /server

# Copy certificates
COPY --from=build /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/

# Copy users
COPY --from=build /etc/passwd /etc/passwd

USER househunt

# Run the binary.
ENTRYPOINT ["/server"]