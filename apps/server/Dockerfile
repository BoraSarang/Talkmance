# 톡맨스 서버 Dockerfile (T-27)
# 멀티 스테이지: 빌드(golang) → 런타임(distroless)
FROM golang:1.26-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/talkmance-server ./cmd/server

FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=build /out/talkmance-server /talkmance-server
EXPOSE 8080
USER nonroot
ENTRYPOINT ["/talkmance-server"]