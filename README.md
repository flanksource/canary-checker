![canary-checker](images/canary-checker.png)
<p style="text-align: center;"><strong style="color: #326ce5;">A Kubernetes Native Multi-tenant Synthetic Monitoring System</strong></p>

---

Canary Checker is a Kubernetes native [Synthetic Monitoring](https://en.wikipedia.org/wiki/Synthetic_monitoring) system. It works with standard Kubernetes monitoring tools like [Prometheus](https://prometheus.io) to proactively monitor your applications across multi-tenant environments to ensure application availability.

Today many applications are written as microservices with service dependencies. This is especially true for applications that run on Kubernetes. For example, you may have a microservice running as a deployment that requires access to a database like [MongoDB](https://www.mongodb.com/kubernetes) or [Postgres](https://www.postgresql.org/). As an [SRE](https://en.wikipedia.org/wiki/Site_Reliability_Engineering) you can create a few canary checks that alert you if the system were about to experience any major issues: 

* You could create a check that ensures the PostgreSQL service responds within `3000` milliseconds 
* You could create a check that ensures you can run a query on Postgres and ensure the query result is as expected.

If any one of these checks fails, (e.g: the service doesn't respond after 3000 milliseconds) you would know that something is not performing as expected in your environment.  You could then take actions to remediate the issues before your end users are even aware there was an issue. 

## Features
<img src="images/ui01.png" style="border: 1px solid #326ce5; border-radius: 10px;" alt="Canary Check Dashboard">

* Built-in UI/Dashboard with multi-cluster aggregation
* CRD based configuration and status reporting
* Prometheus Integration
* Runnable as a CLI for once-off checks or as a standalone server outside kubernetes
* Many built-in check types


## Comparisons

<table style="border:1px solid #326ce5;border-radius: 10px">
    <tr>
        <th>App</th>
        <th>Comparison</th>
    </tr>
    <tr>
        <td>Prometheus</td>
        <td>canary-checker is not a replacement for prometheus, rather a companion. While prometheus provides persistent time series storage, canary-checker only has a small in-memory cache of recent checks. Canary-checker also exposes metrics via <code>/metrics</code> that are scraped by prometheus.</td>
    </tr>
    <tr>
        <td>Grafana</td>
        <td>The built-in UI provides a mechanism to display check results across 1 or more instances without a dependency on grafana/prometheus running. The UI will also display long-term graphs of check results by quering prometheus.</td>
    </tr>
    <tr>
        <td><a href="https://github.com/Comcast/kuberhealthy">Kuberhealthy</a></td>
        <td>Very similar goals, but Kuberhealthy relies on external containers to implement checks and does not provide a UI or multi-cluster/instance aggregation.</td>
    </tr>
    <tr>
        <td><a href="https://cloudprober.org/">Cloudprober</a></td>
        <td>Very similar goals, but Cloudprober is designed for very high scale, not multi-tenancy. Only has ICMP, DNS, HTTP, UDP built-in checks.</td>
    </tr>
</table>