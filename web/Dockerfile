# syntax=docker/dockerfile:1.1.7-experimental

FROM node:8.11.2-slim

ENV npm_config_cache=/.cache/npm

# Fixes jessie repository
RUN sed -i '/jessie-updates/d' /etc/apt/sources.list

# RUN --mount=type=cache,sharing=locked,id=aptlib,target=/var/lib/apt \
#    --mount=type=cache,sharing=locked,id=aptcache,target=/var/cache/apt \
    apt-get update && apt-get install git -y

RUN --mount=type=cache,sharing=locked,id=npm,target=/.cache/npm \
    npm i npm@6.4.1 -g

# prepare to install only package.json dependencies
RUN mkdir -p /app
COPY package*.json /app/

# copy the rest of the app files
WORKDIR /app

RUN --mount=type=cache,sharing=locked,id=npm,target=/.cache/npm \
    npm set audit false ;\
    npm install --silent

COPY . /app

# and build
RUN --mount=type=cache,sharing=locked,id=npm,target=/.cache/npm \
    npm run build

# tar up the resulting build
RUN tar -C /app/dist -czf /web-assets.tar.gz .