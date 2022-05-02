FROM golang:latest AS builder

WORKDIR /fluent-bit-go

COPY go.mod .
COPY go.sum .

RUN go mod download
RUN go mod verify

COPY . .

RUN go build -trimpath -buildmode c-shared -o ./bin/go-test-input-plugin.so ./plugin/testdata/input/input.go
RUN go build -trimpath -buildmode c-shared -o ./bin/go-test-output-plugin.so ./plugin/testdata/output/output.go

FROM ghcr.io/calyptia/calyptia-fluent-bit/advanced:main

COPY --from=builder /fluent-bit-go/bin/go-test-input-plugin.so /fluent-bit/etc/
COPY --from=builder /fluent-bit-go/bin/go-test-output-plugin.so /fluent-bit/etc/

COPY ./plugin/testdata/fluent-bit.conf /fluent-bit/etc/
COPY ./plugin/testdata/plugins.conf /fluent-bit/etc/

ENTRYPOINT [ "/fluent-bit/bin/fluent-bit" ]
CMD [ "/fluent-bit/bin/fluent-bit", "-c", "/fluent-bit/etc/fluent-bit.conf" ]
