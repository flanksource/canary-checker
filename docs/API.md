---
title: Canary Types
hide:
  - toc
---

## Canary

Canary is the Schema for the canaries API

| Field | Description | Scheme | Required |
| ----- | ----------- | ------ | -------- |
| **ObjectMeta** |  | [metav1.ObjectMeta](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.20/#objectmeta-v1-meta) | Yes |
| **Spec** |  | [CanarySpec](#canaryspec) | Yes |
| **Status** |  | [CanaryStatus](#canarystatus) | Yes |
| **TypeMeta** |  | metav1.TypeMeta | Yes |


## CanarySpec

CanarySpec defines the desired state of Canary

| Field | Description | Scheme | Required |
| ----- | ----------- | ------ | -------- |
| cloudwatch |  | \[\][CloudWatchCheck](#cloudwatchcheck) |  |
| containerd |  | \[\][ContainerdPullCheck](#containerdpullcheck) |  |
| containerdPush |  | \[\][ContainerdPushCheck](#containerdpushcheck) |  |
| dns |  | \[\][DNSCheck](#dnscheck) |  |
| docker |  | \[\][DockerPullCheck](#dockerpullcheck) |  |
| dockerPush |  | \[\][DockerPushCheck](#dockerpushcheck) |  |
| ec2 |  | \[\][EC2Check](#ec2check) |  |
| env |  | map[string][VarSource](#varsource) |  |
| gcsBucket |  | \[\][GCSBucketCheck](#gcsbucketcheck) |  |
| helm |  | \[\][HelmCheck](#helmcheck) |  |
| http |  | \[\][HTTPCheck](#httpcheck) |  |
| icmp |  | \[\][ICMPCheck](#icmpcheck) |  |
| icon |  | string |  |
| interval | interval (in seconds) to run checks on Deprecated in favor of Schedule | uint64 |  |
| jmeter |  | \[\][JmeterCheck](#jmetercheck) |  |
| junit |  | \[\][JunitCheck](#junitcheck) |  |
| ldap |  | \[\][LDAPCheck](#ldapcheck) |  |
| mongodb |  | \[\][MongoDBCheck](#mongodbcheck) |  |
| mssql |  | \[\][MssqlCheck](#mssqlcheck) |  |
| namespace |  | \[\][NamespaceCheck](#namespacecheck) |  |
| owner |  | string |  |
| pod |  | \[\][PodCheck](#podcheck) |  |
| postgres |  | \[\][PostgresCheck](#postgrescheck) |  |
| prometheus |  | \[\][PrometheusCheck](#prometheuscheck) |  |
| redis |  | \[\][RedisCheck](#redischeck) |  |
| restic |  | \[\][ResticCheck](#resticcheck) |  |
| s3 |  | \[\][S3Check](#s3check) |  |
| s3Bucket |  | \[\][S3BucketCheck](#s3bucketcheck) |  |
| schedule | Schedule to run checks on. Supports all cron expression, example: '30 3-6,20-23 * * *'. For more info about cron expression syntax see https://en.wikipedia.org/wiki/Cron
 Also supports golang duration, can be set as '@every 1m30s' which runs the check every 1 minute and 30 seconds. | string |  |
| severity |  | string |  |
| smb |  | \[\][SmbCheck](#smbcheck) |  |
| tcp |  | \[\][TCPCheck](#tcpcheck) |  |


## CanaryStatus

CanaryStatus defines the observed state of Canary

| Field | Description | Scheme | Required |
| ----- | ----------- | ------ | -------- |
| **ChecksStatus** |  | map[string]*[CheckStatus](#checkstatus) | Yes |
| **ErrorMessage** |  | *string | Yes |
| **LastCheck** |  | *metav1.Time | Yes |
| **LastTransitionedTime** |  | *metav1.Time | Yes |
| **Latency1H** | Average latency to complete all checks | string | Yes |
| **Message** |  | *string | Yes |
| **ObservedGeneration** |  | int64 | Yes |
| **Status** |  | *CanaryStatusCondition | Yes |
| **Uptime1H** | Availibility over a rolling 1h period | string | Yes |


## CanaryList

CanaryList contains a list of Canary

| Field | Description | Scheme | Required |
| ----- | ----------- | ------ | -------- |
| **Items** |  | \[\][Canary](#canary) | Yes |
| **ListMeta** |  | [metav1.ListMeta](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.20/#listmeta-v1-meta) | Yes |
| **TypeMeta** |  | metav1.TypeMeta | Yes |


## <img src='https://raw.githubusercontent.com/flanksource/flanksource-ui/main/src/icons/cloudwatch.svg' style='height: 32px'/> CloudWatch

This checks the cloudwatch for all the Active alarm and response with the reason
??? example
     ```yaml
     cloudwatch:
       - accessKey:
           valueFrom:
         secretKeyRef:
         key: aws
         name: access-key
         secretKey:
           valueFrom:
         secretKeyRef:
         key: aws
         name: secrey-key
         region: "us-east-1"
         #skipTLSVerify: true
     
     ```

| Field | Description | Scheme | Required |
| ----- | ----------- | ------ | -------- |
| **accessKey** |  | [kommons.EnvVar](https://pkg.go.dev/github.com/flanksource/kommons#EnvVar) | Yes |
| description | Description for the check | string |  |
| display |  | [Template](#template) |  |
| endpoint |  | string |  |
| filter |  | [CloudWatchFilter](#cloudwatchfilter) |  |
| icon | Icon for overwriting default icon on the dashboard | string |  |
| name | Name of the check | string |  |
| region |  | string |  |
| **secretKey** |  | [kommons.EnvVar](https://pkg.go.dev/github.com/flanksource/kommons#EnvVar) | Yes |
| skipTLSVerify | Skip TLS verify when connecting to aws | bool |  |
| test |  | [Template](#template) |  |


## <img src='https://raw.githubusercontent.com/flanksource/flanksource-ui/main/src/icons/containerdPull.svg' style='height: 32px'/> ContainerdPull

??? example
     ```yaml
     apiVersion: canaries.flanksource.com/v1
     kind: Canary
     metadata:
       name: containerd-pull-pass
     spec:
       interval: 30
       containerd:
         - image: docker.io/library/busybox:1.31.1
           expectedDigest: sha256:95cf004f559831017cdf4628aaf1bb30133677be8702a8c5f2994629f637a209
           expectedSize: 764556
     
     ```

| Field | Description | Scheme | Required |
| ----- | ----------- | ------ | -------- |
| auth |  | [Authentication](#authentication) |  |
| description | Description for the check | string |  |
| expectedDigest |  | string |  |
| expectedSize |  | int64 |  |
| icon | Icon for overwriting default icon on the dashboard | string |  |
| **image** |  | string | Yes |
| name | Name of the check | string |  |


## <img src='https://raw.githubusercontent.com/flanksource/flanksource-ui/main/src/icons/containerdPush.svg' style='height: 32px'/> ContainerdPush

??? example
     ```yaml
     
     ```

| Field | Description | Scheme | Required |
| ----- | ----------- | ------ | -------- |
| description | Description for the check | string |  |
| icon | Icon for overwriting default icon on the dashboard | string |  |
| **image** |  | string | Yes |
| name | Name of the check | string |  |
| **password** |  | string | Yes |
| **username** |  | string | Yes |


## <img src='https://raw.githubusercontent.com/flanksource/flanksource-ui/main/src/icons/dns.svg' style='height: 32px'/> DNS

??? example
     ```yaml
     apiVersion: canaries.flanksource.com/v1
     kind: Canary
     metadata:
       name: dns-pass
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


## <img src='https://raw.githubusercontent.com/flanksource/flanksource-ui/main/src/icons/dockerPull.svg' style='height: 32px'/> DockerPull

??? example
     ```yaml
     apiVersion: canaries.flanksource.com/v1
     kind: Canary
     metadata:
       name: docker-pass
     spec:
       interval: 30
       docker:
         - image: docker.io/library/busybox:1.31.1@sha256:b20c55f6bfac8828690ec2f4e2da29790c80aa3d7801a119f0ea6b045d2d2da1
           expectedDigest: sha256:b20c55f6bfac8828690ec2f4e2da29790c80aa3d7801a119f0ea6b045d2d2da1
     
     ```

| Field | Description | Scheme | Required |
| ----- | ----------- | ------ | -------- |
| auth |  | *[Authentication](#authentication) |  |
| description | Description for the check | string |  |
| **expectedDigest** |  | string | Yes |
| **expectedSize** |  | int64 | Yes |
| icon | Icon for overwriting default icon on the dashboard | string |  |
| **image** |  | string | Yes |
| name | Name of the check | string |  |


## <img src='https://raw.githubusercontent.com/flanksource/flanksource-ui/main/src/icons/dockerPush.svg' style='height: 32px'/> DockerPush

DockerPush check will try to push a Docker image to specified registry.
/*
??? example
     ```yaml
     apiVersion: canaries.flanksource.com/v1
     kind: Canary
     metadata:
       name: docker-push0-pass
     spec:
       interval: 30
       dockerPush:
         - image: ttl.sh/flanksource-busybox:1.30
           auth:
             username:
               value: test
             password:
               value: pass
     
     ```

| Field | Description | Scheme | Required |
| ----- | ----------- | ------ | -------- |
| auth |  | *[Authentication](#authentication) |  |
| description | Description for the check | string |  |
| icon | Icon for overwriting default icon on the dashboard | string |  |
| **image** |  | string | Yes |
| name | Name of the check | string |  |


## <img src='https://raw.githubusercontent.com/flanksource/flanksource-ui/main/src/icons/ec2.svg' style='height: 32px'/> EC2

??? example
     ```yaml
     apiVersion: canaries.flanksource.com/v1
     kind: Canary
     metadata:
       name: ec2-pass
     spec:
       interval: 30
       spec:
         ec2:
           - description: test instance
             accessKeyID:
               valueFrom:
                 secretKeyRef:
                   name: aws-credentials
                   key: AWS_ACCESS_KEY_ID
             secretKey:
               valueFrom:
                 secretKeyRef:
                   name: aws-credentials
                   key: AWS_SECRET_ACCESS_KEY
             region: af-south-1
             userData: |
               #!/bin/bash
               yum install -y httpd
               systemctl start httpd
               systemctl enable httpd
               usermod -a -G apache ec2-user
               chown -R ec2-user:apache /var/www
               chmod 2775 /var/www
               find /var/www -type d -exec chmod 2775 {} \;
               find /var/www -type f -exec chmod 0664 {} \;
             securityGroup: WebAccess
     
     ```

| Field | Description | Scheme | Required |
| ----- | ----------- | ------ | -------- |
| **accessKey** |  | [kommons.EnvVar](https://pkg.go.dev/github.com/flanksource/kommons#EnvVar) | Yes |
| ami |  | string |  |
| canaryRef |  | \[\][v1.LocalObjectReference](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.20/#localobjectreference-v1-core) |  |
| description | Description for the check | string |  |
| endpoint |  | string |  |
| icon | Icon for overwriting default icon on the dashboard | string |  |
| keepAlive |  | bool |  |
| name | Name of the check | string |  |
| region |  | string |  |
| **secretKey** |  | [kommons.EnvVar](https://pkg.go.dev/github.com/flanksource/kommons#EnvVar) | Yes |
| securityGroup |  | string |  |
| skipTLSVerify | Skip TLS verify when connecting to aws | bool |  |
| timeOut |  | int |  |
| userData |  | string |  |
| waitTime |  | int |  |


## <img src='https://raw.githubusercontent.com/flanksource/flanksource-ui/main/src/icons/gcsBucket.svg' style='height: 32px'/> GCSBucket

??? example
     ```yaml
     
     ```

| Field | Description | Scheme | Required |
| ----- | ----------- | ------ | -------- |
| **bucket** |  | string | Yes |
| **credentials** |  | *[kommons.EnvVar](https://pkg.go.dev/github.com/flanksource/kommons#EnvVar) | Yes |
| description | Description for the check | string |  |
| display |  | [Template](#template) |  |
| **endpoint** |  | string | Yes |
| filter |  | [FolderFilter](#folderfilter) |  |
| icon | Icon for overwriting default icon on the dashboard | string |  |
| maxAge | MaxAge the latest object should be younger than defined age | Duration |  |
| maxCount | MinCount the minimum number of files inside the searchPath | *int |  |
| maxSize | MaxSize of the files inside the searchPath | Size |  |
| minAge | MinAge the latest object should be older than defined age | Duration |  |
| minCount | MinCount the minimum number of files inside the searchPath | *int |  |
| minSize | MinSize of the files inside the searchPath | Size |  |
| name | Name of the check | string |  |
| test |  | [Template](#template) |  |


## <img src='https://raw.githubusercontent.com/flanksource/flanksource-ui/main/src/icons/http.svg' style='height: 32px'/> HTTP

??? example
     ```yaml
     apiVersion: canaries.flanksource.com/v1
     kind: Canary
     metadata:
       name: http-pass
     spec:
       interval: 30
       http:
         - endpoint: http://status.savanttools.com/?code=200
           thresholdMillis: 3000
           responseCodes: [201, 200, 301]
           responseContent: ""
           maxSSLExpiry: 7
         - endpoint: http://status.savanttools.com/?code=404
           thresholdMillis: 3000
           responseCodes: [404]
           responseContent: ""
           maxSSLExpiry: 7
         - endpoint: http://status.savanttools.com/?code=500
           thresholdMillis: 3000
           responseCodes: [500]
           responseContent: ""
           maxSSLExpiry: 7
     
     ```

| Field | Description | Scheme | Required |
| ----- | ----------- | ------ | -------- |
| authentication | Credentials for authentication headers | *[Authentication](#authentication) |  |
| body | Request Body Contents | string |  |
| description | Description for the check | string |  |
| display |  | [Template](#template) |  |
| **endpoint** | HTTP endpoint to check.  Mutually exclusive with Namespace | string | Yes |
| headers | Header fields to be used in the query | \[\][kommons.EnvVar](https://pkg.go.dev/github.com/flanksource/kommons#EnvVar) |  |
| icon | Icon for overwriting default icon on the dashboard | string |  |
| maxSSLExpiry | Maximum number of days until the SSL Certificate expires. | int |  |
| method | Method to use - defaults to GET | string |  |
| name | Name of the check | string |  |
| namespace | Namespace to crawl for TLS endpoints.  Mutually exclusive with Endpoint | string |  |
| ntlm | NTLM when set to true will do authentication using NTLM v1 protocol | bool |  |
| ntlmv2 | NTLM when set to true will do authentication using NTLM v2 protocol | bool |  |
| responseCodes | Expected response codes for the HTTP Request. | \[\]int |  |
| responseContent | Exact response content expected to be returned by the endpoint. | string |  |
| responseJSONContent | Path and value to of expect JSON response by the endpoint | [JSONCheck](#jsoncheck) |  |
| test |  | [Template](#template) |  |
| thresholdMillis | Maximum duration in milliseconds for the HTTP request. It will fail the check if it takes longer. | int |  |


## <img src='https://raw.githubusercontent.com/flanksource/flanksource-ui/main/src/icons/helm.svg' style='height: 32px'/> Helm

??? example
     ```yaml
     apiVersion: canaries.flanksource.com/v1
     kind: Canary
     metadata:
       name: helm-pass
     spec:
       interval: 30
       helm:
         - chartmuseum: http://chartmuseum.default:8080
           project: library
           auth:
             username:
               value: admin
             password:
               value: passwd
     
     ```

| Field | Description | Scheme | Required |
| ----- | ----------- | ------ | -------- |
| auth |  | *[Authentication](#authentication) |  |
| cafile |  | string |  |
| **chartmuseum** |  | string | Yes |
| description | Description for the check | string |  |
| icon | Icon for overwriting default icon on the dashboard | string |  |
| name | Name of the check | string |  |
| project |  | string |  |


## <img src='https://raw.githubusercontent.com/flanksource/flanksource-ui/main/src/icons/icmp.svg' style='height: 32px'/> ICMP

This test will check ICMP packet loss and duration.

??? example
     ```yaml
     apiVersion: canaries.flanksource.com/v1
     kind: Canary
     metadata:
       name: icmp
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


## <img src='https://raw.githubusercontent.com/flanksource/flanksource-ui/main/src/icons/jmeter.svg' style='height: 32px'/> Jmeter

Jmeter check will run jmeter cli against the supplied host
??? example
     ```yaml
     
     ```

| Field | Description | Scheme | Required |
| ----- | ----------- | ------ | -------- |
| description | Description for the check | string |  |
| host | Host is the server against which test plan needs to be executed | string |  |
| icon | Icon for overwriting default icon on the dashboard | string |  |
| **jmx** | Jmx defines tge ConfigMap or Secret reference to get the JMX test plan | [kommons.EnvVar](https://pkg.go.dev/github.com/flanksource/kommons#EnvVar) | Yes |
| name | Name of the check | string |  |
| port | Port on which the server is running | int32 |  |
| properties | Properties defines the local Jmeter properties | \[\]string |  |
| responseDuration | ResponseDuration under which the all the test should pass | string |  |
| systemProperties | SystemProperties defines the java system property | \[\]string |  |


## <img src='https://raw.githubusercontent.com/flanksource/flanksource-ui/main/src/icons/junit.svg' style='height: 32px'/> Junit

Junit check will wait for the given pod to be completed than parses all the xml files present in the defined testResults directory

??? example
     ```yaml
     apiVersion: canaries.flanksource.com/v1
     kind: Canary
     metadata:
       name: junit-pass
       annotations:
         trace: "true"
     spec:
       interval: 120
       owner: DBAdmin
       severity: high
       junit:
         - testResults: "/tmp/junit-results/"
           display:
             template: |
               ✅ {{.results.passed}} ❌ {{.results.failed}} in 🕑 {{.results.duration}}
               {{  range $r := .results.suites}}
               {{- if gt (conv.ToInt $r.failed)  0 }}
                 {{$r.name}} ✅ {{$r.passed}} ❌ {{$r.failed}} in 🕑 {{$r.duration}}
               {{- end }}
               {{- end }}
           spec:
             containers:
               - name: jes
                 image: docker.io/tarun18/junit-test-pass
                 command: ["/start.sh"]
     
     ```

| Field | Description | Scheme | Required |
| ----- | ----------- | ------ | -------- |
| description | Description for the check | string |  |
| display |  | [Template](#template) |  |
| icon | Icon for overwriting default icon on the dashboard | string |  |
| name | Name of the check | string |  |
| **spec** |  | [v1.PodSpec](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.20/#podspec-v1-core) | Yes |
| test |  | [Template](#template) |  |
| **testResults** |  | string | Yes |
| timeout | Timeout in minutes to wait for specified container to finish its job. Defaults to 5 minutes | int |  |


## <img src='https://raw.githubusercontent.com/flanksource/flanksource-ui/main/src/icons/ldap.svg' style='height: 32px'/> LDAP

The LDAP check will:

* bind using provided user/password to the ldap host. Supports ldap/ldaps protocols.
* search an object type in the provided bind DN.s

??? example
     ```yaml
     apiVersion: canaries.flanksource.com/v1
     kind: Canary
     metadata:
       name: ldap-pass
     spec:
       interval: 30
       ldap:
         - host: ldap://apacheds.ldap.svc:10389
           auth:
             username:
               value: uid=admin,ou=system
             password:
               value: secret
           bindDN: ou=users,dc=example,dc=com
           userSearch: "(&(objectClass=organizationalPerson))"
         - host: ldap://apacheds.ldap.svc:10389
           auth:
             username:
               value: uid=admin,ou=system
             password:
               value: secret
           bindDN: ou=groups,dc=example,dc=com
           userSearch: "(&(objectClass=groupOfNames))"
     
     ```

| Field | Description | Scheme | Required |
| ----- | ----------- | ------ | -------- |
| **auth** |  | *[Authentication](#authentication) | Yes |
| **bindDN** |  | string | Yes |
| description | Description for the check | string |  |
| **host** |  | string | Yes |
| icon | Icon for overwriting default icon on the dashboard | string |  |
| name | Name of the check | string |  |
| skipTLSVerify |  | bool |  |
| userSearch |  | string |  |


## <img src='https://raw.githubusercontent.com/flanksource/flanksource-ui/main/src/icons/mongodb.svg' style='height: 32px'/> Mongo

??? example
     ```yaml
     apiVersion: canaries.flanksource.com/v1
     kind: Canary
     metadata:
       name: mongo
     spec:
       interval: 30
       mongodb:
         - connection: mongodb://$(username):$(password)@mongo.default.svc:27017/?authSource=admin
           description: mongo ping
           auth:
             username:
               value: mongoadmin
             password:
               value: secret
       dns:
         - query: mongo.default.svc
     
     ```

| Field | Description | Scheme | Required |
| ----- | ----------- | ------ | -------- |
| auth |  | [Authentication](#authentication) |  |
| **connection** |  | string | Yes |
| description | Description for the check | string |  |
| icon | Icon for overwriting default icon on the dashboard | string |  |
| name | Name of the check | string |  |


## MsSQL

This check will try to connect to a specified MsSQL database, run a query against it and verify the results.

??? example
     ```yaml
     apiVersion: canaries.flanksource.com/v1
     kind: Canary
     metadata:
       name: mssql-pass
     spec:
       interval: 30
       mssql:
         - connection: "server=mssql.default.svc;user id=$(username);password=$(password);port=1433;database=master"
           auth:
             username:
               value: sa
             password:
               value: S0m3p@sswd
           query: "SELECT 1"
           results: 1
     
     ```

| Field | Description | Scheme | Required |
| ----- | ----------- | ------ | -------- |
| auth |  | [Authentication](#authentication) |  |
| **connection** |  | string | Yes |
| description | Description for the check | string |  |
| display |  | [Template](#template) |  |
| icon | Icon for overwriting default icon on the dashboard | string |  |
| name | Name of the check | string |  |
| **query** |  | string | Yes |
| **results** | Number rows to check for | int | Yes |
| test |  | [Template](#template) |  |


## <img src='https://raw.githubusercontent.com/flanksource/flanksource-ui/main/src/icons/namespace.svg' style='height: 32px'/> Namespace

The Namespace check will:

* create a new namespace using the labels/annotations provided

??? example
     ```yaml
     apiVersion: canaries.flanksource.com/v1
     kind: Canary
     metadata:
       name: namespace-pass
     spec:
       interval: 30
       namespace:
         - checkName: check
           namespaceNamePrefix: "test-foo-"
           podSpec: |
             apiVersion: v1
             kind: Pod
             metadata:
               name: test-namespace
               namespace: default
               labels:
                 app: hello-world-golang
             spec:
               containers:
                 - name: hello
                   image: quay.io/toni0/hello-webserver-golang:latest
           port: 8080
           path: /foo/bar
           ingressName: test-namespace-pod
           ingressHost: "test-namespace-pod.127.0.0.1.nip.io"
           readyTimeout: 5000
           httpTimeout: 15000
           deleteTimeout: 12000
           ingressTimeout: 20000
           deadline: 29000
           httpRetryInterval: 200
           expectedContent: bar
           expectedHttpStatuses: [200, 201, 202]
     
     ```

| Field | Description | Scheme | Required |
| ----- | ----------- | ------ | -------- |
| **checkName** |  | string | Yes |
| deadline |  | int64 |  |
| deleteTimeout |  | int64 |  |
| description | Description for the check | string |  |
| expectedContent |  | string |  |
| expectedHttpStatuses |  | \[\]int64 |  |
| httpRetryInterval |  | int64 |  |
| httpTimeout |  | int64 |  |
| icon | Icon for overwriting default icon on the dashboard | string |  |
| ingressHost |  | string |  |
| ingressName |  | string |  |
| ingressTimeout |  | int64 |  |
| name | Name of the check | string |  |
| namespaceAnnotations |  | map[string]string |  |
| namespaceLabels |  | map[string]string |  |
| namespaceNamePrefix |  | string |  |
| path |  | string |  |
| **podSpec** |  | string | Yes |
| port |  | int64 |  |
| priorityClass |  | string |  |
| readyTimeout |  | int64 |  |
| scheduleTimeout |  | int64 |  |


## <img src='https://raw.githubusercontent.com/flanksource/flanksource-ui/main/src/icons/pod.svg' style='height: 32px'/> Pod

??? example
     ```yaml
     apiVersion: canaries.flanksource.com/v1
     kind: Canary
     metadata:
       name: pod-pass
     spec:
       interval: 30
       pod:
         - name: golang
           namespace: default
           spec: |
             apiVersion: v1
             kind: Pod
             metadata:
               name: hello-world-golang
               namespace: default
               labels:
                 app: hello-world-golang
             spec:
               containers:
                 - name: hello
                   image: quay.io/toni0/hello-webserver-golang:latest
           port: 8080
           path: /foo/bar
           ingressName: hello-world-golang
           ingressHost: "hello-world-golang.127.0.0.1.nip.io"
           scheduleTimeout: 20000
           readyTimeout: 10000
           httpTimeout: 7000
           deleteTimeout: 12000
           ingressTimeout: 10000
           deadline: 60000
           httpRetryInterval: 200
           expectedContent: bar
           expectedHttpStatuses: [200, 201, 202]
           priorityClass: canary-checker-priority
         - name: ruby
           namespace: default
           spec: |
             apiVersion: v1
             kind: Pod
             metadata:
               name: hello-world-ruby
               namespace: default
               labels:
                 app: hello-world-ruby
             spec:
               containers:
                 - name: hello
                   image: quay.io/toni0/hello-webserver-ruby:latest
                   imagePullPolicy: Always
           port: 8080
           path: /foo/bar
           ingressName: hello-world-ruby
           ingressHost: "hello-world-ruby.127.0.0.1.nip.io"
           scheduleTimeout: 30000
           readyTimeout: 12000
           httpTimeout: 7000
           deleteTimeout: 12000
           ingressTimeout: 10000
           deadline: 29000
           httpRetryInterval: 200
           expectedContent: hello, you've hit /foo/bar
           expectedHttpStatuses: [200, 201, 202]
     
     ```

| Field | Description | Scheme | Required |
| ----- | ----------- | ------ | -------- |
| deadline |  | int64 |  |
| deleteTimeout |  | int64 |  |
| description | Description for the check | string |  |
| expectedContent |  | string |  |
| expectedHttpStatuses |  | \[\]int |  |
| httpRetryInterval |  | int64 |  |
| httpTimeout |  | int64 |  |
| icon | Icon for overwriting default icon on the dashboard | string |  |
| **ingressHost** |  | string | Yes |
| **ingressName** |  | string | Yes |
| ingressTimeout |  | int64 |  |
| name | Name of the check | string |  |
| **namespace** |  | string | Yes |
| path |  | string |  |
| port |  | int64 |  |
| priorityClass |  | string |  |
| readyTimeout |  | int64 |  |
| scheduleTimeout |  | int64 |  |
| **spec** |  | string | Yes |


## <img src='https://raw.githubusercontent.com/flanksource/flanksource-ui/main/src/icons/postgres.svg' style='height: 32px'/> Postgres

This check will try to connect to a specified Postgresql database, run a query against it and verify the results.

??? example
     ```yaml
     apiVersion: canaries.flanksource.com/v1
     kind: Canary
     metadata:
       name: postgres-succeed
     spec:
       interval: 30
       postgres:
         - connection: "postgres://$(username):$(password)@postgres.default.svc:5432/postgres?sslmode=disable"
           auth:
             username:
               value: postgresadmin
             password:
               value: admin123
           query: SELECT current_schemas(true)
           display:
             template: |
               {{- range $r := .results.rows }}
               {{- $r.current_schemas}}
               {{- end}}
           results: 1
     
     ```

| Field | Description | Scheme | Required |
| ----- | ----------- | ------ | -------- |
| auth |  | [Authentication](#authentication) |  |
| **connection** |  | string | Yes |
| description | Description for the check | string |  |
| display |  | [Template](#template) |  |
| icon | Icon for overwriting default icon on the dashboard | string |  |
| name | Name of the check | string |  |
| **query** |  | string | Yes |
| **results** | Number rows to check for | int | Yes |
| test |  | [Template](#template) |  |


## <img src='https://raw.githubusercontent.com/flanksource/flanksource-ui/main/src/icons/prometheus.svg' style='height: 32px'/> Prometheus

??? example
     ```yaml
     apiVersion: canaries.flanksource.com/v1
     kind: Canary
     metadata:
       name: prometheus
     spec:
       interval: 30
       prometheus:
         - host: http://prometheus-k8s.monitoring.svc:9090
           query: kubernetes_build_info{job!~"kube-dns|coredns"}
           display:
             template: "{{ (index .results 0).git_version }}"
           test:
             template: "true"
     
     ```

| Field | Description | Scheme | Required |
| ----- | ----------- | ------ | -------- |
| description | Description for the check | string |  |
| display |  | [Template](#template) |  |
| **host** | Address of the prometheus server | string | Yes |
| icon | Icon for overwriting default icon on the dashboard | string |  |
| name | Name of the check | string |  |
| **query** | PromQL query | string | Yes |
| test |  | [Template](#template) |  |


## <img src='https://raw.githubusercontent.com/flanksource/flanksource-ui/main/src/icons/redis.svg' style='height: 32px'/> Redis



| Field | Description | Scheme | Required |
| ----- | ----------- | ------ | -------- |
| **addr** |  | string | Yes |
| auth |  | *[Authentication](#authentication) |  |
| **db** |  | int | Yes |
| description | Description for the check | string |  |
| icon | Icon for overwriting default icon on the dashboard | string |  |
| name | Name of the check | string |  |


## <img src='https://raw.githubusercontent.com/flanksource/flanksource-ui/main/src/icons/restic.svg' style='height: 32px'/> Restic



| Field | Description | Scheme | Required |
| ----- | ----------- | ------ | -------- |
| accessKey | AccessKey access key id for connection with aws s3, minio, wasabi, alibaba oss | *[kommons.EnvVar](https://pkg.go.dev/github.com/flanksource/kommons#EnvVar) |  |
| caCert | CaCert path to the root cert. In case of self-signed certificates | string |  |
| checkIntegrity | CheckIntegrity when enabled will check the Integrity and consistency of the restic reposiotry | bool |  |
| description | Description for the check | string |  |
| icon | Icon for overwriting default icon on the dashboard | string |  |
| **maxAge** | MaxAge for backup freshness | string | Yes |
| name | Name of the check | string |  |
| **password** | Password for the restic repository | *[kommons.EnvVar](https://pkg.go.dev/github.com/flanksource/kommons#EnvVar) | Yes |
| **repository** | Repository The restic repository path eg: rest:https://user:pass@host:8000/ or rest:https://host:8000/ or s3:s3.amazonaws.com/bucket_name | string | Yes |
| secretKey | SecretKey secret access key for connection with aws s3, minio, wasabi, alibaba oss | *[kommons.EnvVar](https://pkg.go.dev/github.com/flanksource/kommons#EnvVar) |  |


## <img src='https://raw.githubusercontent.com/flanksource/flanksource-ui/main/src/icons/s3Bucket.svg' style='height: 32px'/> S3

S3 check will:

* list objects in the bucket to check for Read permissions
* PUT an object into the bucket for Write permissions
* download previous uploaded object to check for Get permissions

??? example
     ```yaml
     apiVersion: canaries.flanksource.com/v1
     kind: Canary
     metadata:
       name: s3-bucket-pass
       annotations:
         trace: "false"
     spec:
       interval: 30
       s3Bucket:
         # Check for any backup not older than 7 days and min size 25 bytes
         - bucket: flanksource-public
           region: eu-central-1
           minSize: 50M
           maxAge: 10d
           filter:
             regex: .*.ova
             minSize: 100M
             # maxAge: 18760h
           display:
             template: |
               {{-  range $f := .results.Files   }}
               {{- if gt $f.Size 0 }}
                 Name: {{$f.Name}} {{$f.ModTime | humanizeTime }} {{ $f.Size | humanizeBytes}}
               {{- end}}
               {{- end  }}
     
     ```

| Field | Description | Scheme | Required |
| ----- | ----------- | ------ | -------- |
| **accessKey** |  | string | Yes |
| **bucket** |  | [Bucket](#bucket) | Yes |
| description | Description for the check | string |  |
| icon | Icon for overwriting default icon on the dashboard | string |  |
| name | Name of the check | string |  |
| **objectPath** |  | string | Yes |
| **secretKey** |  | string | Yes |
| skipTLSVerify | Skip TLS verify when connecting to s3 | bool |  |


## <img src='https://raw.githubusercontent.com/flanksource/flanksource-ui/main/src/icons/s3Bucket.svg' style='height: 32px'/> S3Bucket

This check will

- search objects matching the provided object path pattern
- check that latest object is no older than provided MaxAge value in seconds
- check that latest object size is not smaller than provided MinSize value in bytes.

??? example
     ```yaml
     apiVersion: canaries.flanksource.com/v1
     kind: Canary
     metadata:
       name: s3-bucket-pass
     spec:
       interval: 30
       s3Bucket:
         # Check for any backup not older than 7 days and min size 25 bytes
         - bucket: tests-e2e-1
           accessKey:
             valueFrom:
               secretKeyRef:
                 name: aws-credentials
                 key: AWS_ACCESS_KEY_ID
           secretKey:
             valueFrom:
               secretKeyRef:
                 name: aws-credentials
                 key: AWS_SECRET_ACCESS_KEY
           region: "minio"
           endpoint: "http://minio.minio:9000"
           filter:
             regex: "(.*)backup.zip$"
           maxAge: 7d
           minSize: 25b
           usePathStyle: true
           skipTLSVerify: true
         # Check for any mysql backup not older than 7 days and min size 25 bytes
         - bucket: tests-e2e-1
           accessKey:
             valueFrom:
               secretKeyRef:
                 name: aws-credentials
                 key: AWS_ACCESS_KEY_ID
           secretKey:
             valueFrom:
               secretKeyRef:
                 name: aws-credentials
                 key: AWS_SECRET_ACCESS_KEY
           region: "minio"
           endpoint: "http://minio.minio:9000"
           filter:
             regex: "mysql\\/backups\\/(.*)\\/mysql.zip$"
           maxAge: 7d
           minSize: 25b
           usePathStyle: true
           skipTLSVerify: true
         # Check for any pg backup not older than 7 days and min size 50 bytes
         - bucket: tests-e2e-1
           accessKey:
             valueFrom:
               secretKeyRef:
                 name: aws-credentials
                 key: AWS_ACCESS_KEY_ID
           secretKey:
             valueFrom:
               secretKeyRef:
                 name: aws-credentials
                 key: AWS_SECRET_ACCESS_KEY
           region: "minio"
           endpoint: "http://minio.minio:9000"
           filter:
             regex: "pg\\/backups\\/(.*)\\/backup.zip$"
           maxAge: 7d
           minSize: 25b
           usePathStyle: true
           skipTLSVerify: true
     
     ```

| Field | Description | Scheme | Required |
| ----- | ----------- | ------ | -------- |
| **accessKey** |  | [kommons.EnvVar](https://pkg.go.dev/github.com/flanksource/kommons#EnvVar) | Yes |
| **bucket** |  | string | Yes |
| description | Description for the check | string |  |
| display |  | [Template](#template) |  |
| endpoint |  | string |  |
| filter |  | [FolderFilter](#folderfilter) |  |
| icon | Icon for overwriting default icon on the dashboard | string |  |
| maxAge | MaxAge the latest object should be younger than defined age | Duration |  |
| maxCount | MinCount the minimum number of files inside the searchPath | *int |  |
| maxSize | MaxSize of the files inside the searchPath | Size |  |
| minAge | MinAge the latest object should be older than defined age | Duration |  |
| minCount | MinCount the minimum number of files inside the searchPath | *int |  |
| minSize | MinSize of the files inside the searchPath | Size |  |
| name | Name of the check | string |  |
| objectPath | glob path to restrict matches to a subset | string |  |
| region |  | string |  |
| **secretKey** |  | [kommons.EnvVar](https://pkg.go.dev/github.com/flanksource/kommons#EnvVar) | Yes |
| skipTLSVerify | Skip TLS verify when connecting to aws | bool |  |
| test |  | [Template](#template) |  |
| usePathStyle | Use path style path: http://s3.amazonaws.com/BUCKET/KEY instead of http://BUCKET.s3.amazonaws.com/KEY | bool |  |


## <img src='https://raw.githubusercontent.com/flanksource/flanksource-ui/main/src/icons/smb.svg' style='height: 32px'/> Smb

Smb check will connect to the given samba server with given credentials
find the age of the latest updated file and compare it with minAge
count the number of file present and compare with minCount if defined

??? example
     ```yaml
     
     ```

| Field | Description | Scheme | Required |
| ----- | ----------- | ------ | -------- |
| **auth** |  | *[Authentication](#authentication) | Yes |
| description | Description for the check | string |  |
| display |  | [Template](#template) |  |
| domain | Domain... | string |  |
| filter |  | [FolderFilter](#folderfilter) |  |
| icon | Icon for overwriting default icon on the dashboard | string |  |
| maxAge | MaxAge the latest object should be younger than defined age | Duration |  |
| maxCount | MinCount the minimum number of files inside the searchPath | *int |  |
| maxSize | MaxSize of the files inside the searchPath | Size |  |
| minAge | MinAge the latest object should be older than defined age | Duration |  |
| minCount | MinCount the minimum number of files inside the searchPath | *int |  |
| minSize | MinSize of the files inside the searchPath | Size |  |
| name | Name of the check | string |  |
| port | Port on which smb server is running. Defaults to 445 | int |  |
| searchPath | SearchPath sub-path inside the mount location | string |  |
| **server** | Server location of smb server. Can be `hostname/ip` or in `\\server\e$\a\b\c` syntax
Where server is the `hostname` `e$`` is the sharename and `a/b/c` is the searchPath location | string | Yes |
| sharename | Sharename to mount from the samba server | string |  |
| test |  | [Template](#template) |  |
| workstation | Workstation... | string |  |


## Template



| Field | Description | Scheme | Required |
| ----- | ----------- | ------ | -------- |
| jsonPath |  | string |  |
| template |  | string |  |


## Connection



| Field | Description | Scheme | Required |
| ----- | ----------- | ------ | -------- |
| auth |  | [Authentication](#authentication) |  |
| **connection** |  | string | Yes |


## AWSConnection



| Field | Description | Scheme | Required |
| ----- | ----------- | ------ | -------- |
| **accessKey** |  | [kommons.EnvVar](https://pkg.go.dev/github.com/flanksource/kommons#EnvVar) | Yes |
| endpoint |  | string |  |
| region |  | string |  |
| **secretKey** |  | [kommons.EnvVar](https://pkg.go.dev/github.com/flanksource/kommons#EnvVar) | Yes |
| skipTLSVerify | Skip TLS verify when connecting to aws | bool |  |


## Bucket



| Field | Description | Scheme | Required |
| ----- | ----------- | ------ | -------- |
| **endpoint** |  | string | Yes |
| **name** |  | string | Yes |
| **region** |  | string | Yes |


## FolderFilter



| Field | Description | Scheme | Required |
| ----- | ----------- | ------ | -------- |
| maxAge |  | Duration |  |
| maxSize |  | Size |  |
| minAge |  | Duration |  |
| minSize |  | Size |  |
| regex |  | string |  |


## GCPConnection



| Field | Description | Scheme | Required |
| ----- | ----------- | ------ | -------- |
| **credentials** |  | *[kommons.EnvVar](https://pkg.go.dev/github.com/flanksource/kommons#EnvVar) | Yes |
| **endpoint** |  | string | Yes |


## Authentication



| Field | Description | Scheme | Required |
| ----- | ----------- | ------ | -------- |
| **password** |  | [kommons.EnvVar](https://pkg.go.dev/github.com/flanksource/kommons#EnvVar) | Yes |
| **username** |  | [kommons.EnvVar](https://pkg.go.dev/github.com/flanksource/kommons#EnvVar) | Yes |


## Display



| Field | Description | Scheme | Required |
| ----- | ----------- | ------ | -------- |


## VarSource

VarSource represents a source for a value

| Field | Description | Scheme | Required |
| ----- | ----------- | ------ | -------- |
| **ConfigMapKeyRef** | Selects a key of a ConfigMap. | *corev1.ConfigMapKeySelector | Yes |
| **FieldRef** | Selects a field of the pod: supports metadata.name, metadata.namespace, metadata.labels, metadata.annotations,
spec.nodeName, spec.serviceAccountName, status.hostIP, status.podIP, status.podIPs. | *corev1.ObjectFieldSelector | Yes |
| **SecretKeyRef** | Selects a key of a secret in the pod's namespace | *corev1.SecretKeySelector | Yes |
| **Value** |  | string | Yes |


## CloudWatchFilter



| Field | Description | Scheme | Required |
| ----- | ----------- | ------ | -------- |
| actionPrefix |  | *string |  |
| alarmPrefix |  | *string |  |
| alarms |  | \[\]string |  |
| state |  | string |  |

