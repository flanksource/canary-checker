---
hide:
- toc
---


# Installation

=== "Linux"
    ```bash
    wget  https://github.com/flanksource/canary-checker/releases/latest/download/canary-checker   \
      -O /usr/bin/canary-checker && \
      chmod +x /usr/bin/canary-checker
    ```

=== "MacOSX"
    ```bash
    wget  https://github.com/flanksource/canary-checker/releases/latest/download/canary-checker_osx  \
      -O /usr/local/bin/canary-checker && \
      chmod +x /usr/local/bin/canary-checker
    ```

=== "Windows"
    ```bash
    wget -nv -nc -O https://github.com/flanksource/canary-checker/releases/latest/download/canary-checker.exe
    ```


# Running
Start a server to execute checks

```
canary-checker serve [flags]
```

### Options

```
  -c, --configfile string         Specify configfile
      --dev                       Run in development mode
      --devGuiPort int            Port used by a local npm server in development mode (default 3004)
  -h, --help                      help for serve
      --httpPort int              Port to expose a health dashboard  (default 8080)
      --include-check string      Run matching canaries - useful for debugging
      --log-fail                  Log every failing check (default true)
      --log-pass                  Log every passing check
      --maxStatusCheckCount int   Maximum number of past checks in the status page (default 5)
      --metricsPort int           Port to expose a health dashboard  (default 8081)
      --name string               Server name shown in aggregate dashboard (default "local")
  -n, --namespace string          Watch only specified namespaces, otherwise watch all
      --prometheus string         URL of the prometheus server that is scraping this instance
      --pull-servers strings      push check results to multiple canary servers
      --push-servers strings      push check results to multiple canary servers
  -s, --schedule string           schedule to run checks on. Supports all cron expression and golang duration support in format: '@every duration'
      --expose-env       Expose environment variables for use in all templates. Note this has serious security implications with untrusted canaries
      --json-logs        Print logs in json format to stderr
  -v, --loglevel count   Increase logging level
```
