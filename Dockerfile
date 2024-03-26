# Stage 1. Build the binaries.
FROM golang:1.22-alpine as build

# Git is required for fetching the dependencies.
RUN apk add --no-cache git gcc musl-dev

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

# Build the server binary.
RUN CGO_ENABLED=1 go build -o /out/server/server cmd/server/*.go

# Use ldd to list the dynamicly linked dependencies and copy them to the output directory.
RUN ldd /out/server/server | tr -s [:blank:] '\n' | grep ^/ | xargs -I % install -D % /out/server/%

# Build the dbmigrate binary.
RUN CGO_ENABLED=1 go build -o /out/dbmigrate/dbmigrate cmd/dbmigrate/*.go

# Use ldd to list the dynamicly linked dependencies and copy them to the output directory.
RUN ldd /out/dbmigrate/dbmigrate | tr -s [:blank:] '\n' | grep ^/ | xargs -I % install -D % /out/dbmigrate/%

# Stage 2. Run the binary.
FROM scratch AS final

# Copy binaries
COPY --from=build /out /

# Copy certificates
COPY --from=build /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/

# Copy users
COPY --from=build /etc/passwd /etc/passwd

USER househunt

# Run the binary.
ENTRYPOINT ["/out/server/server"]
