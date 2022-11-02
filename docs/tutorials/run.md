---
hide:
- toc
---


# Installation

=== "Linux (amd64)"
    ```bash
    wget  https://github.com/flanksource/canary-checker/releases/latest/download/canary-checker_linux_amd64   \
      -O /usr/bin/canary-checker && \
      chmod +x /usr/bin/canary-checker
    ```

=== "MacOSX (amd64)"
    ```bash
    wget  https://github.com/flanksource/canary-checker/releases/latest/download/canary-checker_darwin_amd64  \
      -O /usr/local/bin/canary-checker && \
      chmod +x /usr/local/bin/canary-checker
    ```

=== "Makefile"
    ```Makefile
    OS = $(shell uname -s | tr '[:upper:]' '[:lower:]')
    ARCH = $(shell uname -m | sed 's/x86_64/amd64/')
    wget -nv -nc https://github.com/flanksource/canary-checker/releases/latest/download/canary-checker_$(OS)_$(ARCH)  \
      -O /usr/local/bin/canary-checker && \
      chmod +x /usr/local/bin/canary-checker

    ```
=== "Windows"
    ```bash
    wget -nv -nc -O https://github.com/flanksource/canary-checker/releases/latest/download/canary-checker.exe
    ```





# Running
Execute checks and return

```bash
canary-checker run <canary.yaml> [flags]
```

### Options

```
  -h, --help               help for run
  -j, --junit string       Export JUnit XML formatted results to this file e.g: junit.xml
  -n, --namespace string   Specify namespace
      --expose-env       Expose environment variables for use in all templates. Note this has serious security implications with untrusted canaries
      --json-logs        Print logs in json format to stderr
  -v, --loglevel count   Increase logging level
```
