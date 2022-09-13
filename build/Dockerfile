FROM registry.redhat.io/ubi8/go-toolset:latest as builder

WORKDIR /go/src/app
COPY . .

USER 0

# RUN go get -d ./... && \
#     go build -o widgets  server.go
RUN make build


FROM registry.redhat.io/ubi8/ubi-minimal:8.5

WORKDIR /

COPY --from=builder /go/src/app/release/kafka /widgets

USER 1001

CMD ["/widgets"]

