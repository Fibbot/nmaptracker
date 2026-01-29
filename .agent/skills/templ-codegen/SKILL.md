---
name: Templ Code Generator
description: Standardizes the generation of Templ components.
---

# Templ Code Generator

This skill ensures that Templ components are generated correctly using the `templ` CLI.

## Usage

To generate Go code from `.templ` files, run:

```bash
templ generate
```

or use the provided `generate.sh` script if you want a consistent wrapper.

## Prerequisites

Ensure `templ` is installed:
```bash
go install github.com/a-h/templ/cmd/templ@latest
```
