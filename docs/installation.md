# Installation


## Linux
    ```bash
    wget  https://github.com/flanksource/canary-checker/releases/latest/download/canary-checker   \
      -O /usr/bin/canary-checker && \
      chmod +x /usr/bin/canary-checker
    ```

## MacOSX
    ```bash
    wget  https://github.com/flanksource/canary-checker/releases/latest/download/canary-checker_osx  \
      -O /usr/local/bin/canary-checker && \
      chmod +x /usr/local/bin/canary-checker
    ```

## Windows
    ```bash
    wget -nv -nc -O https://github.com/flanksource/canary-checker/releases/latest/download/canary-checker.exe
    ```

### Windows Service

Canary Checker can be installed as a service in Windows environment using the `install-service.ps1` which is available at the root of the repo.
The script accepts the following parameters to define the service.

- configfile: Path to the config file with canaries. Defaults to "$pwd\canary-checker.yaml"
- httpPort: port to start the server on.
- metricsPort: port to expose the metrics on
- name: name of the server
- uninstall: A switch flag. Used to uninstall the service. For example: `.\install-service.ps1 -uninstall`
- pushServers: A comma separated list of servers to push the check data
