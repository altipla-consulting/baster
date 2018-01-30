
FROM golang:1.8
MAINTAINER Ernesto Alejo <ernesto@altiplaconsulting.com>

COPY . /go/src/github.com/altipla-consulting/baster

RUN go install github.com/altipla-consulting/baster/cmd/baster

CMD ["baster"]
