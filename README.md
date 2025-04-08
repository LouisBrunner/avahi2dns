# avahi2dns [![Build](https://github.com/LouisBrunner/avahi2dns/actions/workflows/build.yml/badge.svg)](https://github.com/LouisBrunner/avahi2dns/actions/workflows/build.yml)

Linux DNS-resolution is done by the standard library (through functions like `getaddrinfo` or `gethostbyname`), unfortunately `musl` (used by Alpine Linux) doesn't support mdns out-of-the-box.

`avahi2dns` is a small DNS server which interfaces with avahi (through D-Bus) and forwards (perfect for Alpine Linux and musl)

## Building

Requires go 1.24 or later

```
go build
```

## Usage

```shell
$ ./avahi2dns -h
Usage: avahi2dns [--domains DOMAINS] [--addr ADDR] [--port PORT] [--debug] [--timeout TIMEOUT]

Options:
  --domains DOMAINS, -d DOMAINS
                         comma-separated list of domains to resolve
  --addr ADDR, -a ADDR   address to bind on [default: localhost]
  --port PORT, -p PORT   port to bind on [default: 53]
  --debug, -v            also include debug information [default: false]
  --timeout TIMEOUT, -t TIMEOUT
                         timeout for the Avahi request, 0 meaning none, see https://pkg.go.dev/time#ParseDuration for units and format [default: 0s]
  --help, -h             display this help and exit
```

> [!TIP]
> You can also configure the server through environment variables.

### Examples

By default the server will bind to port 53 on localhost (not accessible outside your computer) and resolve any domain with the following extensions: `home`, `internal`, `intranet`, `lan`, `local`, `private`, `test`

```shell
$ sudo ./avahi2dns
INFO[0000] starting DNS server                           addr="localhost:53"
...
```

Settings can be changed through command-line arguments or environment variables:

```shell
$ ./avahi2dns -p 5454 -a '0.0.0.0' -d 'local,home'
or
$ BIND='0.0.0.0' PORT=5454 ./avahi2dns -d 'local,home'
```

You can also use the debug flag if you need more information about what the server is doing (probably overkill):

```shell
$ ./avahi2dns -v -p 5454
DEBU[2025-04-07T23:27:41Z]/src/server.go:31 main.NewForwarder() connection to dbus...
DEBU[2025-04-07T23:27:41Z]/src/server.go:37 main.NewForwarder() connection to avahi through dbus...
DEBU[2025-04-07T23:27:41Z]/src/server.go:44 main.NewForwarder() adding dns handler                            domain=home
...
```

In this example, `avahi2dns` is run as a background job for the sake of demonstration, don't do that in production (use OpenRC, systemd or similar).

```shell
$ sudo ./avahi2dns &
INFO[0000] starting DNS server                           addr="localhost:53"
$ docker run --rm --entrypoint ash --dns=127.0.0.1 --net=host alpine -c 'apk add iputils && ping your-name-host.local'
fetch https://dl-cdn.alpinelinux.org/alpine/v3.14/main/armv7/APKINDEX.tar.gz
fetch https://dl-cdn.alpinelinux.org/alpine/v3.14/community/armv7/APKINDEX.tar.gz
(1/2) Installing libcap (2.50-r0)
(2/2) Installing iputils (20210202-r0)
Executing busybox-1.33.1-r3.trigger
OK: 4 MiB in 16 packages
INFO[0006] forwarding query to avahi                     component=main name=your-name-host.local. protocol=0 type=A
INFO[0006] forwarding query to avahi                     component=main name=your-name-host.local. protocol=1 type=AAAA
PING your-name-host.local (172.16.16.2) 56(84) bytes of data.
64 bytes from 172.16.16.2 (172.16.16.2): icmp_seq=1 ttl=64 time=0.456 ms
64 bytes from 172.16.16.2 (172.16.16.2): icmp_seq=2 ttl=64 time=0.426 ms
64 bytes from 172.16.16.2 (172.16.16.2): icmp_seq=3 ttl=64 time=0.429 ms
64 bytes from 172.16.16.2 (172.16.16.2): icmp_seq=4 ttl=64 time=0.418 ms
```

> [!IMPORTANT]
> If you are running `avahi2dns` within Docker, you will need to run your container with the `host` network mode (i.e. `--net=host`) instead of the `bridge` default.

## Installation

### Alpine (edge version)

Feel free to check the Alpine wiki directly for their recommended setup: https://wiki.alpinelinux.org/wiki/MDNS#Setup_avahi2dns

> [!NOTE]
> Thanks to [Willow Barraco](https://cv.willowbarraco.fr/en/), `avahi2dns` is available through the `testing` repository of Alpine.

```bash
# all those steps require you to be root

# add the testing repository, skip if you already have it enabled
echo "@testing https://dl-cdn.alpinelinux.org/alpine/edge/testing" >> /etc/apk/repositories

apk add avahi2dns@testing

# if you are using openrc
apk add avahi2dns-openrc@testing
# automatically starts avahi2dns at boot
rc-update add avahi2dns
# start it right now as well
rc-service avahi2dns start
```

### Any Linux

Checkout the latest release [here](https://github.com/LouisBrunner/avahi2dns/releases/latest) and download the binary corresponding to your architecture.

```bash
# make sure to match your system architecture
wget https://github.com/LouisBrunner/avahi2dns/releases/latest/download/avahi2dns-linux-arm64
mv avahi2dns-linux-arm64 /usr/bin/avahi2dns
```

You can then setup `avahi2dns` through OpenRC, systemd or similar. Check this [file](openrc/avahi2dns) for an example for the former.
