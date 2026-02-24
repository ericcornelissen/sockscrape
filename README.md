# SockScrape

Scrape package source code from [Socket](https://socket.dev/).

## Installation

```shell
git clone https://github.com/ericcornelissen/sockscrape
cd sockscrape
go run . -install
```

## Usage

```shell
go run . -ecosystem <cargo|npm|etc...> -module <name>
```

### Examples

```shell
go run . -ecosystem cargo -module simple-regex
```

```shell
go run . -ecosystem cargo -module simple-regex -version 1.0.1
```
