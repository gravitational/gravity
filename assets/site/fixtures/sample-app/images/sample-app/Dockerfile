# Docker image for the sample app
FROM busybox:latest

LABEL Description="Sample app" \
      Vendor="Gravitational Inc" \
      Version="1.0.2"

ENV PATH="$PATH:/opt/sample"

ADD sample /opt/sample/sample

WORKDIR /opt/sample
EXPOSE 5000

CMD ["/opt/sample/sample"]
