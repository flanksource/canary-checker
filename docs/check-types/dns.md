## <img src='https://raw.githubusercontent.com/flanksource/flanksource-ui/main/src/icons/dns.svg' style='height: 32px'/> DNS

??? example
     ```yaml
     apiVersion: canaries.flanksource.com/v1
     kind: Canary
     metadata:
       name: dns-check
     spec:
       interval: 30
       dns:
         - server: 8.8.8.8
           port: 53
           query: "1.2.3.4.nip.io"
           querytype: "A"
           minrecords: 1
           exactreply: ["1.2.3.4"]
           timeout: 10
           thresholdMillis: 1000
         - server: 8.8.8.8
           port: 53
           query: "8.8.8.8"
           querytype: "PTR"
           minrecords: 1
           exactreply: ["dns.google."]
           timeout: 10
           thresholdMillis: 100
         - server: 8.8.8.8
           port: 53
           query: "dns.google"
           querytype: "CNAME"
           minrecords: 1
           exactreply: ["dns.google."]
           timeout: 10
           thresholdMillis: 1000
         - server: 8.8.8.8
           port: 53
           query: "flanksource.com"
           querytype: "MX"
           minrecords: 1
           exactreply:
             - "aspmx.l.google.com. 1"
             - "alt1.aspmx.l.google.com. 5"
             - "alt2.aspmx.l.google.com. 5"
             - "aspmx3.googlemail.com. 10"
             - "aspmx2.googlemail.com. 10"
           timeout: 10
           thresholdMillis: 1000
         - server: 8.8.8.8
           port: 53
           query: "flanksource.com"
           querytype: "TXT"
           minrecords: 1
           exactreply: ["google-site-verification=IIE1aJuvqseLUKSXSIhu2O2lgdU_d8csfJjjIQVc-q0"]
           timeout: 10
           thresholdMillis: 1000
         - server: 8.8.8.8
           port: 53
           query: "flanksource.com"
           querytype: "NS"
           minrecords: 1
           exactreply:
             - "ns-91.awsdns-11.com."
             - "ns-908.awsdns-49.net."
             - "ns-1450.awsdns-53.org."
             - "ns-1896.awsdns-45.co.uk."
           timeout: 10
           thresholdMillis: 1000
       #  - server: 8.8.8.8
       #    port: 53
       #    querytype: "SRV"
       #    query: "_test._tcp.test"
       #    timeout: 10
       #    srvReply:
       #      target: ""
       #      port: 0
       #      priority: 0
       #      weight: 0*
     
     ```

| Field | Description | Scheme | Required |
| ----- | ----------- | ------ | -------- |
| description | Description for the check | string |  |
| exactreply |  | \[\]string |  |
| icon | Icon for overwriting default icon on the dashboard | string |  |
| minrecords |  | int |  |
| name | Name of the check | string |  |
| port |  | int |  |
| query |  | string |  |
| querytype |  | string |  |
| **server** |  | string | Yes |
| thresholdMillis |  | int |  |
| timeout |  | int |  |
