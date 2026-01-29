---
name: Test Suite Runner
description: a centralized test runner for the project.
---

# Test Suite Runner

This skill provides a consistent way to run tests for the project, ensuring all necessary flags and environment variables are set.

## Usage

To run all tests:

```bash
./.agent/skills/test-suite/run_tests.sh
```

To run a specific package:

```bash
./.agent/skills/test-suite/run_tests.sh ./internal/db/...
```

## Features

- Enables race detection (`-race`).
- Vetoes packages without tests.
- Defaults to running all tests if no arguments are provided.
