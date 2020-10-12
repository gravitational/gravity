# resulting container image will contain `tele` tool and can be used
# to create Cluster images from within a container.

ARG BASE

FROM $BASE

RUN apt-get update && \
    apt-get -y install apt-transport-https ca-certificates curl gnupg software-properties-common

RUN curl -fsSL https://download.docker.com/linux/debian/gpg | sudo apt-key add - && \
    add-apt-repository "deb [arch=amd64] https://download.docker.com/linux/debian buster stable"

RUN apt-get update && \
    apt-get -y install docker-ce-cli
