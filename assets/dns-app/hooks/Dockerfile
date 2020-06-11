FROM quay.io/gravitational/rig:5.5.6-1-g9ad0b17b

ARG CHANGESET
ENV RIG_CHANGESET $CHANGESET

ADD entrypoint.sh /

ENTRYPOINT ["dumb-init", "/entrypoint.sh"]
