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
COPY ./.release/docker/entrypoint.sh  /
RUN chmod +x /entrypoint.sh

# Include compliance details about the container and what it contains.
COPY Dockerfile /
COPY NOTICE     /
COPY LICENSE    /

# Make the location for the configuration file that will be used.
RUN     mkdir /etc/xmidt-agent/
COPY ./.release/docker/config/xmidt_agent.yaml  /etc/xmidt-agent/xmidt_agent.yaml
COPY ./.release/docker/certs/  /certs
COPY ./internal/wrphandlers/mocktr181/mock_tr181.json /

USER root

#ENTRYPOINT ["tail", "-f", "/dev/null"]
#ENTRYPOINT ["/entrypoint.sh"]
ENTRYPOINT ["/xmidt-agent"]

EXPOSE 6666

CMD ["-f", "/etc/xmidt-agent/xmidt_agent.yaml", "--dev"]
