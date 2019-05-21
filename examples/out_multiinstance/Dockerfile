FROM golang:1.12 as gobuilder

WORKDIR /root

ENV GOOS=linux\
    GOARCH=amd64

COPY / /root/

RUN go build \
    -buildmode=c-shared \
    -o /out_multiinstance.so \
    github.com/fluent/fluent-bit-go/examples/out_multiinstance

FROM fluent/fluent-bit:1.1

COPY --from=gobuilder /out_multiinstance.so /fluent-bit/bin/
COPY --from=gobuilder /root/examples/out_multiinstance/fluent-bit.conf /fluent-bit/etc/
COPY --from=gobuilder /root/examples/out_multiinstance/plugins.conf /fluent-bit/etc/

EXPOSE 2020

# CMD ["/fluent-bit/bin/fluent-bit", "--plugin", "/fluent-bit/bin/out_multiinstance.so", "--config", "/fluent-bit/etc/fluent-bit.conf"]¬
CMD ["/fluent-bit/bin/fluent-bit", "--config", "/fluent-bit/etc/fluent-bit.conf"]¬
