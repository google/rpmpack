# rpmpack (tar2rpm) - package rpms the easy way

## Disclaimer

This is not an official Google product, it is just code that happens to be owned
by Google.

## Overview

`tar2rpm` is a tool that takes a tar and outputs an rpm. `rpmpack` is a golang library to create rpms. Both are written in pure go, without using rpmbuild or spec files. API documentation for `rpmpack` can be found in [![GoDoc](https://godoc.org/github.com/google/rpmpack?status.svg)](https://godoc.org/github.com/google/rpmpack).

## Installation

```bash
$ go get -u github.com/google/rpmpack/...
```

This will make the `tar2rpm` tool available in `${GOPATH}/bin`, which by default means `~/go/bin`.

## Usage

`tar2rpm` takes a `tar` file (from `stdin` or a specified filename), and outputs an `rpm`.

```
Usage:
  tar2rpm [OPTION] [FILE]
Options:
  -file FILE
        write rpm to FILE instead of stdout
  -name string
        the package name
  -release string
        the rpm release
  -version string
        the package version
```

## Features

 - You put files into the rpm, so that rpm/yum will install them on a host.
 - Simple.
 - No `spec` files.
 - Does not build anything.
 - Does not try to auto-detect dependencies.
 - Does not try to magically deduce on which computer architecture you run.
 - Does not require any rpm database or other state, and does not use the
   filesystem.

## Downsides

 - Is not related to the team the builds rpmlib.
 - May easily wreak havoc on rpm based systems. It is surprisingly easy to cause
   rpm to segfault on corrupt rpm files.
 - Many features are missing.
 - All of the artifactes are stored in memory, sometimes more than once.
 - Less backwards compatible than `rpmbuild`.

## Philosophy

Sometimes you just want files to make it to hosts, and be managed by the package
manager. `rpmbuild` can use a `spec` file, together with a specific directory
layout and local database, to build/install/package your files. But you don't
need all that. You want something similar to tar.

As the project progresses, we must maintain the complexity/value ratio. This
includes both code complexity and interface complexity.

## Disclaimer

This is not an official Google product, it is just code that happens to be owned
by Google.
