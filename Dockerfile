
FROM golang:1.8
MAINTAINER Ernesto Alejo <ernesto@altiplaconsulting.com>

COPY ./cmd /go/src/cmd
COPY ./vendor /go/src/vendor

RUN go install baster/cmd/baster

WORKDIR /go/src/baster
CMD baster
