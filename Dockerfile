FROM golang:1.17-alpine AS build
WORKDIR /sensor
COPY . .
RUN    go env -w CGO_ENABLED=0 \
    && go env -w GO111MODULE=on
RUN    go build -v

FROM alpine:latest
RUN apk add --no-cache tzdata
CMD [ "/sensor" ]
COPY --from=build /sensor/sensor /sensor
