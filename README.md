# go-htpdate-win

go-htpdate-win is a lightweight Windows utility written in Go that queries HTTP(S) servers for the current time and optionally sets the local system clock. It can also run in daemon mode, serving time over NTP on localhost.

## Features

* Query multiple HTTP/HTTPS servers for their Date header.
* Display per-server time and offset relative to local system clock.
* Gives average offset across multiple servers.
* Optionally set the local system time to the average server time.
* Daemon mode: run a local NTP server on 127.0.0.1:123 using HTTP time sources.
* Minimal dependencies: single executable, portable on Windows.
* Packed with upx for smaller binary

## Usage

```
.\go-htpdate-win.exe [options] <URL> <URL2>  


Options
Flag	Description
-q	Query only, print server times and offsets (default)
-s	Set local system time to average of server times
-d	Debug output
-D	Daemon mode: serve NTP on localhost:123
-i	Interval in seconds for daemon mode polling (default: 1)
-h	Show help

```

### Notes

1. http(s) is no substitute for ntp over udp. It will be accurate +/- a second at best. 
2. Windows only, use htpdate on Linux
3. Daemon mode uses port 123 which like any priv port requires local Admin. Note that w32time service may be running on that port too which may need to be stopped. 

