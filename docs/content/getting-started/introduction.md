---
title: "Introduction"
description: "What gdc is and how it is put together."
weight: 10
---

A command line for gdc.

gdc is a single binary. It speaks to gdc over plain HTTPS,
shapes the responses into clean records, and gets out of your way. There is
nothing to sign up for and nothing to run alongside it.

## How it is built

- A **library package** (`gdc`) holds the HTTP client and the typed
  data models. It paces requests, sets an honest User-Agent, and retries the
  transient failures any public site throws under load.
- A **domain** (`gdc/domain.go`) declares each operation once on the
  [any-cli/kit](https://github.com/tamnd/any-cli) framework. That single
  declaration becomes a CLI command, an HTTP route, an MCP tool, and a
  resource-URI dereference. It is the one place you add to the tool.
- A thin **`cmd/gdc`** hands the assembled app to `kit.Run`, which
  builds the command tree and the serve and mcp surfaces.

## One operation, four surfaces

Because an operation is surface-neutral, the same `page` you run on the command
line is also a route and a tool:

```bash
gdc page <path>                  # the command
gdc serve --addr :7777           # GET /v1/page/<path>
gdc mcp                          # the page tool, over stdio
ant get gdc://page/<path>        # the URI dereference (via a host)
```

You write the fetch and the record shape; the surfaces come for free.

## Scope

gdc is a read-only client over data gdc already serves
publicly. It reads that data and shapes it for you. That narrow scope keeps it a
single small binary with no database, no daemon, and no setup.

Next: [install it](/getting-started/installation/), then take the
[quick start](/getting-started/quick-start/).
