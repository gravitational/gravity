FROM quay.io/gravitational/debian-tall

COPY ./start.sh /opt/
COPY ./init.sh /opt/
RUN mkdir /opt/gravity
COPY ./gravity /opt/gravity/gravity
