# xmidt-agent
The client agent for the Xmidt service.

[![Build Status](https://github.com/xmidt-org/xmidt-agent/actions/workflows/ci.yml/badge.svg)](https://github.com/xmidt-org/xmidt-agent/actions/workflows/ci.yml)
[![codecov.io](http://codecov.io/github/xmidt-org/xmidt-agent/coverage.svg?branch=main)](http://codecov.io/github/xmidt-org/xmidt-agent?branch=main)
[![Go Report Card](https://goreportcard.com/badge/github.com/xmidt-org/xmidt-agent)](https://goreportcard.com/report/github.com/xmidt-org/xmidt-agent)
[![Apache V2 License](http://img.shields.io/badge/license-Apache%20V2-blue.svg)](https://github.com/xmidt-org/xmidt-agent/blob/main/LICENSE)
[![GitHub Release](https://img.shields.io/github/release/xmidt-org/xmidt-agent.svg)](CHANGELOG.md)


## Code of Conduct

This project and everyone participating in it are governed by the [XMiDT Code Of Conduct](https://xmidt.io/code_of_conduct/). 
By participating, you agree to this Code.


## Contributing

Refer to [CONTRIBUTING.md](CONTRIBUTING.md).

## Run xmidt-agent simulator as a docker container
1. build xmidt-agent for alpine
    ```cd cmd/xmidt-agent```
    ```env GOOS=linux GOARCH=arm64 go build .```
2. mv cmd/xmidt-agent/xmdidt-agent binary to the root directory 
3. from the root directory, build the docker container
    ```docker build -t xmdit-agent .```
4. run the container 
    ```docker run xmdit-agent --dev```
5. Note that you will see a connection error unless a websocket server is running at the default url specified by websocket -> back_up_url in cmd/xmidt-agent/default-config.yaml.
6. To override the default configuration, update the below config file OR bind a config file to target "/etc/xmidt-agent/xmidt-agent.yaml" at runtime:
```.release/docker/config/config.yml```
7. If using TLS, the Dockerfile expects the certificate and key file to be in a root directory called "certs" at build time.  Otherwise bind the directory at runtime. 

