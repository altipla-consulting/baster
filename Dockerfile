
FROM google/debian:wheezy
MAINTAINER Ernesto Alejo <ernesto@altiplaconsulting.com>

COPY baster /opt/ac/baster

WORKDIR /opt/ac
CMD ["/opt/ac/baster"]
