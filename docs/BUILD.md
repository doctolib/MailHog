Building MailHog
================

MailHog is built using `make`, and using [this Makefile](../Makefile).

You can install MailHog using:
`go install github.com/doctolib/MailHog@latest`

### Static assets

MailHog's HTML, CSS, Javascript and SQL assets are embedded into the binary
at compile time using [`go:embed`](https://pkg.go.dev/embed) (see
[assets/assets.go](../assets/assets.go) and [queries/queries.go](../queries/queries.go)).

No code generation step is required: a plain `go build` on a fresh checkout works.

### Building a release

Releases are built using [gox](https://github.com/mitchellh/gox).

Run `make release` to cross-compile for all available platforms.
