FROM scratch

COPY bashrc /root/.bashrc
COPY build/webhead /root/webhead

WORKDIR /root
ENV HOME=/root

# to save on space we mount parent host's userspace tools instead
# of packaging our own
VOLUME ["/bin", \
        "/lib", \
        "/etc", \
        "/lib64", \
        "/usr", \
        "/sbin", \
        "/data"]

CMD ["/root/webhead"]
