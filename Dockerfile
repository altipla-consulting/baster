
FROM google/debian:wheezy
MAINTAINER Ernesto Alejo <ernesto@altiplaconsulting.com>

RUN apt-get update && \
    apt-get install -y ca-certificates

COPY baster /opt/ac/baster

WORKDIR /opt/ac
CMD ["/opt/ac/baster"]
