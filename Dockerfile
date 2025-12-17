FROM golang:1.22-alpine AS build
WORKDIR /src
COPY . .
RUN go build -o /gosubc ./cmd/gosubc
FROM scratch
ENV PATH=/usr/bin
COPY --from=build /gosubc /usr/bin
USER 1001
ENTRYPOINT ["/usr/bin/gosubc"]
