# SockScrape

Scrape package source code from [Socket](https://socket.dev/).

## Installation

```shell
# OPTIONAL
go install github.com/ericcornelissen/sockscrape@latest
```

```shell
go run github.com/ericcornelissen/sockscrape@latest -install
```

## Usage

```shell
mkdir out
go run github.com/ericcornelissen/sockscrape@latest \
  -ecosystem <cargo|npm|etc...> -module <name>
```


### Examples

```shell
mkdir out
go run github.com/ericcornelissen/sockscrape@latest \
  -ecosystem cargo -module simple-regex
```

```shell
mkdir out
go run github.com/ericcornelissen/sockscrape@latest \
  -ecosystem cargo -module simple-regex -version 1.0.1
```
