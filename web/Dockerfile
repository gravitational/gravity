FROM node:12.18.3-stretch-slim

RUN apt-get update && apt-get install git -y

# prepare to install only package.json dependencies
RUN mkdir -p /app
COPY package*.json /app/

# copy the rest of the app files
WORKDIR /app

RUN npm set audit false ;\
    npm install --silent

COPY . /app

# and build
RUN npm run build

# tar up the resulting build
RUN tar -C /app/dist -czf /web-assets.tar.gz .