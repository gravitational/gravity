FROM node:8.11.2-slim

RUN apt-get update && apt-get install git -y

RUN apt-get install -y python python-dev python-pip

RUN npm i npm@6.1.0 -g

# prepare to install only package.json dependencies
RUN mkdir -p /app
COPY package*.json /app/

# copy the rest of the app files
WORKDIR /app

RUN npm set audit false ;\
    npm install

COPY . /app

# and build
RUN npm run build