# syntax=docker/dockerfile:1.1.7-experimental

ARG BUILD_BOX
FROM ${BUILD_BOX}

ARG FIO_BRANCH
RUN env
RUN mkdir -p /gopath/native/fio && \
	    git clone https://github.com/axboe/fio.git --branch ${FIO_BRANCH} --single-branch /gopath/native/fio

RUN cd /gopath/native/fio && ./configure --build-static && make

