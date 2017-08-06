
FROM golang:1.8
MAINTAINER Ernesto Alejo <ernesto@altiplaconsulting.com>

COPY . /go/src/baster

RUN go install baster/cmd/baster

WORKDIR /go/src/baster
CMD baster
