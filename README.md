# avahi2dns [![Build](https://github.com/LouisBrunner/avahi2dns/actions/workflows/build.yml/badge.svg)](https://github.com/LouisBrunner/avahi2dns/actions/workflows/build.yml)

Small DNS server which interface with avahi (perfect for Alpine Linux and musl)

## Building

Requires go 1.16 or later

```
go build
```

## Usage

```shell
$ ./avahi2dns -h
Usage: avahi2dns [--domains DOMAINS] [--addr ADDR] [--port PORT] [--debug]

Options:
  --domains DOMAINS, -d DOMAINS
                         comma-separated list of domains to resolve
  --addr ADDR, -a ADDR   address to bind on [default: localhost]
  --port PORT, -p PORT   port to bind on [default: 53]
  --debug, -v            also include debug information [default: false]
  --help, -h             display this help and exit
```

### Examples

By default the server will bind to port 53 on localhost (not accessible outside the computer) and resolve any domain with the following extensions: `home`, `internal`, `intranet`, `lan`, `local`, `private`, `test`

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
DEBU[0000]avahi2dns/server.go:14 main.runServer() connection to dbus...
DEBU[0000]avahi2dns/server.go:21 main.runServer() connection to avahi through dbus...
DEBU[0000]avahi2dns/server.go:37 main.runServer() adding dns handler                            domain=home
...
```

## Using with Alpine and Docker on Linux

Linux DNS-resolution is done by the standard library (through functions like `getaddrinfo` or `gethostbyname`), unfortunately `musl` used by Alpine Linux doesn't support mdns out-of-the-box.
However you can easily add this lookup through `avahi2dns`:

```shell
$ sudo ./avahi2dns &
INFO[0000] starting DNS server                           addr="localhost:53"
$ docker run --rm --entrypoint ash --net=host alpine -c 'apk add iputils && echo "nameserver 127.0.0.1" >> /etc/resolv.conf && ping your-name-host.local'
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

Note: this requires your Docker container to use `--net=host` instead of the default `--net=bridge`, but that it might be possible without using the latest Docker distributions

Note: `avahi2dns` is run as a background job for the sake of demonstration, you should really run it through `systemd` or similar
