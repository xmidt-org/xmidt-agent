# SPDX-FileCopyrightText: 2024 Comcast Cable Communications Management, LLC
# SPDX-License-Identifier: Apache-2.0

FROM docker.io/library/golang:1.19-alpine as builder

WORKDIR /src

RUN apk add --no-cache --no-progress ca-certificates

COPY . .

##########################
# Build the final image.
##########################

FROM alpine:latest

# Copy over the standard things you'd expect.
COPY --from=builder /etc/ssl/certs/ca-certificates.crt  /etc/ssl/certs/
COPY xmidt-agent /

# Include compliance details about the container and what it contains.
COPY Dockerfile /
COPY NOTICE     /
COPY LICENSE    /
COPY mock_tr181.json /

# Make the location for the configuration file that will be used.
RUN     mkdir /etc/xmidt-agent/

COPY ./.release/docker/config/config.yaml  /etc/xmidt-agent/xmidt-agent.yaml

USER root

EXPOSE 6666

ENTRYPOINT ["/xmidt-agent"]