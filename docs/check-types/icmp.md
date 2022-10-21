## <img src='https://raw.githubusercontent.com/flanksource/flanksource-ui/main/src/icons/icmp.svg' style='height: 32px'/> ICMP

This check performs ICMP requests for information on ICMP packet loss and duration.

??? example
     ```yaml
     apiVersion: canaries.flanksource.com/v1
     kind: Canary
     metadata:
       name: icmp-check
     spec:
       interval: 30
       icmp:
         - endpoint: https://api.github.com
           thresholdMillis: 600
           packetLossThreshold: 10
           packetCount: 2
     
     ```

| Field | Description | Scheme | Required |
| ----- | ----------- | ------ | -------- |
| description | Description for the check | string |  |
| **endpoint** |  | string | Yes |
| icon | Icon for overwriting default icon on the dashboard | string |  |
| name | Name of the check | string |  |
| packetCount |  | int |  |
| packetLossThreshold |  | int64 |  |
| thresholdMillis |  | int64 |  |