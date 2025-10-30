# mtest:dummy

This package exists purely as a lightweight target for exercising the `mtest` CLI during development.

Running the package tests:

```bash
meteor test-packages ./test/dummy
```

or with the Go binary:

```bash
./mtest --package mtest:dummy --once
```
