# Golang application builder steps
FROM golang:1.11.5 as builder
WORKDIR /go/src/github.com/GoogleCloudPlatform/open-match
# Get only the necessary files from the repo.
# https://stackoverflow.com/a/39317180/3113674
# TODO: Set these to the final branch name after RC approval.
RUN svn export https://github.com/joeholley/open-match/branches/gdcdemo/internal
RUN svn export https://github.com/joeholley/open-match/branches/gdcdemo/config
WORKDIR /go/src/github.com/GoogleCloudPlatform/open-match/cmd/backendapi
COPY . .
RUN go get -d -v
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo .
RUN rm -f matchmaker_config.yaml
RUN ln -s /etc/config/matchmaker_config.yaml .

#FROM gcr.io/distroless/static  
#COPY --from=builder /go/src/github.com/GoogleCloudPlatform/open-match/cmd/backendapi/backendapi .

ENTRYPOINT ["./backendapi"]
