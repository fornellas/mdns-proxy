[![Latest Release](https://img.shields.io/github/v/release/fornellas/mdns-proxy)](https://github.com/fornellas/mdns-proxy/releases)
[![Push](https://github.com/fornellas/mdns-proxy/actions/workflows/push.yaml/badge.svg)](https://github.com/fornellas/mdns-proxy/actions/workflows/push.yaml)
[![Go Report Card](https://goreportcard.com/badge/github.com/fornellas/mdns-proxy)](https://goreportcard.com/report/github.com/fornellas/mdns-proxy)
[![Go Reference](https://pkg.go.dev/badge/github.com/fornellas/mdns-proxy.svg)](https://pkg.go.dev/github.com/fornellas/mdns-proxy)
[![License: GPL v3](https://img.shields.io/badge/License-GPLv3-blue.svg)](https://www.gnu.org/licenses/gpl-3.0)
[![Buy me a beer: donate](https://img.shields.io/badge/Donate-Buy%20me%20a%20beer-yellow)](https://www.paypal.com/donate?hosted_button_id=AX26JVRT2GS2Q)

# mDNS Proxy

This is a HTTP proxy that forwards requests to mDNS hosts:

- Requests to base domain (eg: `example.com`) shows a list of links to discovered mDNS hosts.
- Each mDNS host (eg: `foo.local`) is accessible via a subdomain (eg: `foo.example.com`).

This is useful is situations where you have a secure network with mDNS hosts (eg: ESPHome IoT devices, which may lack strong security) and want to access control to its hosts.

An example setup would be to run this at a server that's connected to both the client network and the mDNS servers secure network:

```
Client network < Server > mDNS secure network
```

The server can restrict traffic to the mDNS secure network, and a Nginx can be setup to take care of access control:

```
Browser > Nginx > mDNS proxy > mDNS host
```

## Install

Pick the [latest release](https://github.com/fornellas/mdns-proxy/releases) with:

```bash
GOARCH=$(case $(uname -m) in i[23456]86) echo 386;; x86_64) echo amd64;; armv6l|armv7l) echo arm;; aarch64) echo arm64;; *) echo Unknown machine $(uname -m) 1>&2 ; exit 1 ;; esac) && wget -O- https://github.com/fornellas/mdns-proxy/releases/latest/download/mdns-proxy.linux.$GOARCH.gz | gunzip > mdns-proxy && chmod 755 mdns-proxy
./mdns-proxy server --base-domain example.com
```

## Development

[Docker](https://www.docker.com/) is used to create a reproducible development environment on any machine:

```bash
git clone git@github.com:fornellas/mdns-proxy.git
cd mdns-proxy/
./builld.sh
```

Typically you'll want to stick to `./builld.sh rrb`, as it enables you to edit files as preferred, and the build will automatically be triggered on any file changes.