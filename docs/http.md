# HTTP

This example has multiple HTTP endpoints which will crawl periodically.

- Endpoints: HTTP endpoints to crawl
- ThresholdMillis: Maximum duration in milliseconds for the HTTP request. It will fail the check if it takes longer.
- ResponseCodes: Expected response codes for the HTTP Request.
- ResponseContent: Exact response content expected to be returned by the endpoint.
- MaxSSLExpiry: Maximum number of days until the SSL Certificate expires.

```yaml
http:
  - endpoints:
      - https://httpstat.us/200
      - https://httpstat.us/301
    thresholdMillis: 3000
    responseCodes: [201,200,301]
    responseContent: ""
    maxSSLExpiry: 60
  - endpoints:
      - https://httpstat.us/500
    thresholdMillis: 3000
    responseCodes: [500]
    responseContent: ""
    maxSSLExpiry: 60
  - endpoints:
      - https://httpstat.us/500
    thresholdMillis: 3000
    responseCodes: [302]
    responseContent: ""
    maxSSLExpiry: 60
```