FROM golang:1.19-alpine AS build

WORKDIR /sampling
COPY . .
RUN    go env -w CGO_ENABLED=0 \
    && go build -v


FROM alpine:latest

RUN apk add --no-cache tzdata
ENTRYPOINT [ "/sampling" ]
COPY --from=build /sampling/sampling /sampling
