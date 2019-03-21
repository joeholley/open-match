FROM golang:1.10.3 as builder
WORKDIR /go/src/github.com/GoogleCloudPlatform/open-match/test/cmd/clientloadgen/
COPY ./ ./
RUN go get -d -v 
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o clientloadgen . 

FROM gcr.io/distroless/static
COPY --from=builder /go/src/github.com/GoogleCloudPlatform/open-match/test/cmd/clientloadgen/clientloadgen .
COPY *.ping ./
COPY city.percent .

CMD ["./clientloadgen"]
