---
build: go build -o canary-checker .
cwd: ../..
codeBlocks: [bash]
---

# HTTP Logging Levels

## Default Verbosity

### command: no http logging at info level

```yaml
exitCode: 0
timeout: 30
```

```bash
./canary-checker run fixtures/minimal/http_single_pass.yaml 2>&1
```

- cel: stdout.contains('passed')
- not: contains: Accept
- not: contains: httpbin.flanksource.com/status/200 200

### command: no http logging at -v1

```yaml
exitCode: 0
timeout: 30
```

```bash
./canary-checker run fixtures/minimal/http_single_pass.yaml -v 2>&1
```

- cel: stdout.contains('passed')
- not: contains: Accept
- not: contains: httpbin.flanksource.com/status/200 200

### command: trace annotation at info level does not log headers

```yaml
exitCode: 0
timeout: 30
```

```bash
cat fixtures/minimal/http_single_pass.yaml | yq '.metadata.annotations.trace = "true"' | ./canary-checker run - 2>&1
```

- cel: stdout.contains('passed') && !stdout.contains('Accept')

## CLI Verbosity Levels

### command: -v2 prints access log

```yaml
exitCode: 0
timeout: 30
```

```bash
./canary-checker run fixtures/minimal/http_single_pass.yaml -vv 2>&1
```

- cel: stdout.contains('200')
- cel: stdout.contains('httpbin.flanksource.com')
- cel: stdout.contains('passed') && !stdout.contains('Host:')

### command: -v3 prints headers

```yaml
exitCode: 0
timeout: 30
```

```bash
./canary-checker run fixtures/minimal/http_single_pass.yaml -vvv 2>&1
```

- cel: stdout.contains('Accept')
- cel: stdout.contains('200')
- cel: stdout.contains('passed')

### command: -v4 prints headers and response

```yaml
exitCode: 0
timeout: 30
```

```bash
./canary-checker run fixtures/minimal/http_single_pass.yaml -vvvv 2>&1
```

- cel: stdout.contains('Accept')
- cel: stdout.contains('Content-Type')
- cel: stdout.contains('200')
- cel: stdout.contains('passed')

## http.log Property Levels (no -v flag needed)

### command: http.log=access prints single-line access log

```yaml
exitCode: 0
timeout: 30
```

```bash
./canary-checker run fixtures/minimal/http_single_pass.yaml -P http.log=access 2>&1
```

- cel: stdout.contains('200')
- cel: stdout.contains('httpbin.flanksource.com')
- cel: stdout.contains('passed') && !stdout.contains('Host:')

### command: http.log=headers prints headers without body

```yaml
exitCode: 0
timeout: 30
```

```bash
./canary-checker run fixtures/minimal/http_single_pass.yaml -P http.log=headers 2>&1
```

- cel: stdout.contains('Accept')
- cel: stdout.contains('200')
- cel: stdout.contains('passed')

### command: http.log=debug is equivalent to headers

```yaml
exitCode: 0
timeout: 30
```

```bash
./canary-checker run fixtures/minimal/http_single_pass.yaml -P http.log=debug 2>&1
```

- cel: stdout.contains('Accept')
- cel: stdout.contains('200')
- cel: stdout.contains('passed')

### command: http.log=all prints everything

```yaml
exitCode: 0
timeout: 30
```

```bash
./canary-checker run fixtures/minimal/http_single_pass.yaml -P http.log=all 2>&1
```

- cel: stdout.contains('Accept')
- cel: stdout.contains('Content-Type')
- cel: stdout.contains('200')
- cel: stdout.contains('passed')

### command: http.log=trace is equivalent to all

```yaml
exitCode: 0
timeout: 30
```

```bash
./canary-checker run fixtures/minimal/http_single_pass.yaml -P http.log=trace 2>&1
```

- cel: stdout.contains('Accept')
- cel: stdout.contains('Content-Type')
- cel: stdout.contains('200')
- cel: stdout.contains('passed')

## JSON Logging

### command: json access log has method and url fields

```yaml
exitCode: 0
timeout: 30
```

```bash
./canary-checker run fixtures/minimal/http_single_pass.yaml --json-logs -P http.log=access 2>&1
```

- cel: stdout.contains('"method"')
- cel: stdout.contains('"url"')
- cel: stdout.contains('"status"')
- cel: stdout.contains('"duration"')
- cel: stdout.contains('passed')

### command: json headers log has request.headers

```yaml
exitCode: 0
timeout: 30
```

```bash
./canary-checker run fixtures/minimal/http_single_pass.yaml --json-logs -P http.log=headers 2>&1
```

- cel: stdout.contains('"headers"')
- cel: stdout.contains('"Accept"')
- cel: stdout.contains('"method"')
- cel: stdout.contains('passed')

### command: json response log has response.headers

```yaml
exitCode: 0
timeout: 30
```

```bash
./canary-checker run fixtures/minimal/http_single_pass.yaml --json-logs -P http.log=response 2>&1
```

- cel: stdout.contains('"responseHeaders"')
- cel: stdout.contains('"headers"')
- cel: stdout.contains('"Content-Type"')
- cel: stdout.contains('passed')

### command: json all log has all fields

```yaml
exitCode: 0
timeout: 30
```

```bash
./canary-checker run fixtures/minimal/http_single_pass.yaml --json-logs -P http.log=all 2>&1
```

- cel: stdout.contains('"headers"')
- cel: stdout.contains('"responseHeaders"')
- cel: stdout.contains('"method"')
- cel: stdout.contains('"status"')
- cel: stdout.contains('passed')

### command: json all log is valid json

```yaml
exitCode: 0
timeout: 30
```

```bash
./canary-checker run fixtures/minimal/http_single_pass.yaml --json-logs -Phttp.log=all 2>&1 | jq -e . > /dev/null && echo "valid-json"
```

- cel: stdout.contains('valid-json')

## Authorization Header Redaction

### command: auth headers are redacted in pretty output

```yaml
exitCode: 0
timeout: 30
```

```bash
./canary-checker run fixtures/minimal/http_auth_static_pass.yaml -P http.log=headers 2>&1
```

- cel: stdout.contains('Authorization')
- not: contains: aGVsbG86d29ybGQ
- cel: stdout.contains('passed')

### command: auth headers are redacted in json output

```yaml
exitCode: 0
timeout: 30
```

```bash
./canary-checker run fixtures/minimal/http_auth_static_pass.yaml --json-logs -P http.log=headers 2>&1
```

- cel: stdout.contains('"Authorization"')
- cel: stdout.contains('****')
- not: contains: aGVsbG86d29ybGQ
- cel: stdout.contains('passed')

## Request Body Redaction

### command: oauth client_secret is redacted in request body

```yaml
exitCode: 1
timeout: 30
```

```bash
./canary-checker run fixtures/minimal/http_oauth2_pass.yaml -P http.log=all 2>&1
```

- cel: stdout.contains('client_secret')
- not: contains: MJlO3binatD9jk1

### command: oauth client_secret is redacted in json request body

```yaml
exitCode: 1
timeout: 30
```

```bash
./canary-checker run fixtures/minimal/http_oauth2_pass.yaml --json-logs -P http.log=all 2>&1
```

- cel: stdout.contains('client_secret')
- not: contains: MJlO3binatD9jk1

## Property Overrides

### command: debug off property is acknowledged

```yaml
exitCode: 0
timeout: 30
```

```bash
cat fixtures/minimal/http_single_pass.yaml | yq '.metadata.annotations.debug = "true"' | ./canary-checker run - -P http.debug=off -vvvvv 2>&1
```

- cel: stdout.contains('passed')

## HAR Collection

### command: har property creates structured har file

```yaml
exitCode: 0
timeout: 30
```

```bash
rm -rf /tmp/canary-har-test
cat fixtures/minimal/http_single_pass.yaml | yq '.metadata.annotations.trace = "true"' | ./canary-checker run - -P http.har=true -P http.har.location=/tmp/canary-har-test -vvvvv 2>&1
cat /tmp/canary-har-test/*.har
```

- cel: stdout.contains('"request"')
- cel: stdout.contains('"response"')
- cel: stdout.contains('"method"')
- cel: stdout.contains('"headers"')
