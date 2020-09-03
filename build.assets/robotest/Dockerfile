ARG BASE

FROM $BASE

ARG UID
ARG GID

RUN (groupadd robotest --gid=$GID && useradd robotest --uid=$UID --gid=$GID)

USER robotest
