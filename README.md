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
1. cp .release/docker/config/config_template.yaml to .release/docker/config/xmidt_agent.yaml
2. replace field values (in caps) with desired values (TODO need more of an explanation of the jwt pem stuff)
3. cd cmd/xmidt-agent (TODO build from root directory)
4. build for alpine
    ```env GOOS=linux GOARCH=arm64 go build .```
5. mv xmidt-agent ../..  (TODO)
6. cd back to root of repository
7. docker build -t xmidt-agent .
8. docker run xmidt-agent
