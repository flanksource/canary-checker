# ICMP

This test will check ICMP packet loss and duration.

```yaml
icmp:
  - endpoints:
      - https://google.com
      - https://yahoo.com
    thresholdMillis: 400
    packetLossThreshold: 0.5
    packetCount: 2
```