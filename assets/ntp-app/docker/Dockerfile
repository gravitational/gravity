FROM debian:jessie
RUN apt-get update && \
    apt-get -y install ntp ntpstat ntpdate && \
    apt-get autoremove && \
    apt-get clean && \
    apt-get autoclean
COPY client.conf server.conf /etc/ntp/
COPY start-client.sh /root
