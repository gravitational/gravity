# Copyright 2021 Gravitational Inc.
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#      http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

FROM node:8.11.2-slim

# Fixes jessie repository
RUN sed -i '/jessie-updates/d' /etc/apt/sources.list

RUN apt-get update && apt-get install git -y

RUN npm i npm@6.4.1 -g

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