
FROM node:22-alpine AS node
FROM golang:1.23.10-alpine AS golang

FROM alpine

COPY --from=node /usr/lib /usr/lib
COPY --from=node /usr/local/share /usr/local/share
COPY --from=node /usr/local/lib /usr/local/lib
COPY --from=node /usr/local/include /usr/local/include
COPY --from=node /usr/local/bin /usr/local/bin
COPY --from=golang /usr/local/go/ /usr/local/go/

RUN apk add --no-cache --update aws-cli docker-cli jq
RUN npm install -g npm && npm install -g aws-cdk@2 esbuild

ENV GOROOT=/usr/local/go
ENV PATH=$GOROOT/bin:$PATH