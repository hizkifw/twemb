FROM --platform=$BUILDPLATFORM golang:1.26-alpine AS build
ARG TARGETOS TARGETARCH
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY *.go ./
RUN CGO_ENABLED=0 GOOS=$TARGETOS GOARCH=$TARGETARCH go build -trimpath -ldflags="-s -w" -o /out/twemb . \
 && mkdir /out/data

FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=build /out/twemb /twemb
# exclusions.json is written to the working directory
COPY --from=build --chown=nonroot:nonroot /out/data /data
WORKDIR /data
VOLUME /data
ENTRYPOINT ["/twemb"]
