Building MailHog
================

MailHog is built using `make`, and using [this Makefile](../Makefile).

You can install MailHog using:
`go get github.com/doctolib/MailHog`

### Why do I need a Makefile?

MailHog has HTML, CSS and Javascript assets which need to be converted
to a go source file using [go-bindata](https://github.com/jteeuwen/go-bindata).

This must happen before running `go build` or `go install` to avoid compilation
errors (e.g., `no buildable Go source files in MailHog-UI/assets`).

### go generate

The build should be updated to use `go generate` (added in Go 1.4) to
preprocess static assets into go source files.

However, this will break backwards compatibility with Go 1.2/1.3.

### Building a release

Releases are built using [gox](https://github.com/mitchellh/gox).

Run `make release` to cross-compile for all available platforms.
