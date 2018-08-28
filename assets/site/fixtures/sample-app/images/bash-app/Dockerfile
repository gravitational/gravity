# This is /bin/bash wrapped into a busybox-based container
FROM busybox:latest

LABEL Description="Bash app" \
      Vendor="Gravitational Inc" \
      Version="1.0.2"

ADD Dockerfile /Dockerfile

WORKDIR /

CMD ["/bin/bash", "-c", "while true : ; do sleep 1; done"]
