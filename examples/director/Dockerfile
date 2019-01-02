FROM gcr.io/open-match-222408/openmatch-base:dev as builder

WORKDIR /go/src/github.com/GoogleCloudPlatform/open-match/examples/director
COPY . .
RUN go get -d -v
# RUN go get -d -v agones.dev/agones

WORKDIR /go/src/github.com/GoogleCloudPlatform/open-match/vendor
RUN mv /go/src/agones.dev/agones/vendor/* ./
RUN mv /go/src/agones.dev ./

WORKDIR /go/src/github.com/GoogleCloudPlatform/open-match/examples/director
# RUN go get -d -v
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o director .

FROM alpine:3.8

RUN adduser -D director
COPY --from=builder \
    /go/src/github.com/GoogleCloudPlatform/open-match/examples/director/director \
    /home/director/director
# COPY --from=builder \
#     /go/src/github.com/GoogleCloudPlatform/open-match/examples/director/director_config.json \
#     /home/director/
# COPY --from=builder \
#     /go/src/github.com/GoogleCloudPlatform/open-match/examples/director/profiles/*.json \
#     /home/director/profiles/

RUN chown -R director /home/director && \
    chmod o+x /home/director/director

USER director
WORKDIR /home/director
ENTRYPOINT /home/director/director
