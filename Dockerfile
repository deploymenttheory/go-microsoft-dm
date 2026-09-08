# Build the reference MDM server (dmserver) as a static, pure-Go binary.
#
# The repository is two modules: the root library and server/ which replaces the
# library with ../, so both are copied before building. The build uses no cgo,
# so every SQL driver is the pure-Go implementation and the final image needs no
# system libraries.
FROM golang:1.27 AS build

WORKDIR /src

# Module metadata first, so dependency downloads cache across source changes.
COPY go.mod go.sum ./
COPY server/go.mod server/go.sum ./server/
RUN cd server && go mod download

# Then the sources for both modules.
COPY . .

# CGO_ENABLED=0 keeps the binary static; the drivers (modernc sqlite, pgx, mysql)
# are all pure Go.
RUN CGO_ENABLED=0 GOOS=linux go -C server build -trimpath -ldflags="-s -w" \
	-o /out/dmserver ./cmd/dmserver

FROM gcr.io/distroless/static-debian12:nonroot

COPY --from=build /out/dmserver /usr/local/bin/dmserver

# The server binds :8443 by default (DM_LISTEN); configuration is via DM_* env.
EXPOSE 8443
USER nonroot:nonroot
ENTRYPOINT ["/usr/local/bin/dmserver"]
