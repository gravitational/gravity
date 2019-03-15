FROM quay.io/gravitational/rig:5.5.2

ARG CHANGESET
ENV RIG_CHANGESET $CHANGESET

ADD entrypoint.sh /

ENTRYPOINT ["dumb-init", "/entrypoint.sh"]
