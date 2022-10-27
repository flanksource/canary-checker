# Getting started with config-db

`config-db` is a straightforward JSON-based configuration management database. It enables you to scrape configuration from several sources on an ongoing basis and navigate that configuration in an easy-to-navigate and search JSON tree.

By doing this, `config-db` enables you to view and search the change history of your configuration across multiple dimensions (node, zone, environment, application, technology, etc). As well as compare and view the differences between configurations across environments.

In this guide, we will illustrate how to set up `config-db` and configure it to scrape configuration from a Git repository, in line with the GitOps philosophy.

The scraped configuration will be branch and environment aware, enabling us to see the differences between those dimensions.

Additionally - we will illustrate how `config-db` can keep track of configuration changes should we modify the configuration in our Git repository.
## Installation
### Database Configuration

`config-db` needs a backing PostgreSQL database to run its migrations against. 

We will use the PostgreSQL command line utility `createdb` to create our database.

Run the following command in your terminal:

```
createdb -h localhost -p 5432 -U postgres config
```

Where `config` is the name of the database we’re creating.

We can then simply export the connection URL for the database as an environment variable for `config-db` to use by running the following in our terminal:

```
export DB_URL=postgres://postgres@localhost:5432/config
```
## Scraping

Before getting started with scraping, we will need to clone and build the `config-db` project.

You can find the repository here.

Clone the repository and run the following command in the project's root.

```
make build
```

This will build the project and prepare us to start scraping.

Once the build is complete, we can ensure everything is working by running `config-db` with the default configuration for scraping.

```
% ./.bin/config-db serve
INFO[0000] Loaded 7 config rules
2022-10-12T19:08:14.962+0200	INFO	Initialized DB: localhost:5432/config (7503 kB)
2022-10-12 19:08:14.984859 I | goose: no migrations to run. current version: 99

   ____    __
  / __/___/ /  ___
 / _// __/ _ \/ _ \
/___/\__/_//_/\___/ v4.6.3
High performance, minimalist Go web framework
https://echo.labstack.com
____________________________________O/_______
                                    O\
⇨ http server started on [::]:8080
12/Oct/2022:19:08:15 +0200: Attempting to connect to the database...
12/Oct/2022:19:08:15 +0200: Connection successful
12/Oct/2022:19:08:15 +0200: Listening on port 3000
12/Oct/2022:19:08:15 +0200: Listening for notifications on the pgrst channel
12/Oct/2022:19:08:15 +0200: Config re-loaded
12/Oct/2022:19:08:15 +0200: Schema cache loaded
```

Once we’ve verified that we can build and start `config-db`, we can move on to scraping configuration from a Git repository.
Scraping configuration from Git

As an example, we will scrape the configuration from a sample repository. We recommend forking this repository so that you can modify it and play with the different features that `config-db` provides.

This repository contains a simple YAML definition for a canary that can be used by canary-checker.

```
apiVersion: canaries.flanksource.com/v1
kind: Canary
metadata:
 name: http-pass-single
 labels:
   canary: http
spec:
 interval: 40
 http:
   - endpoint: http://status.savanttools.com/?code=200
     name: sample-check
     thresholdMillis: 3000
     responseCodes: [201, 200, 301]
     responseContent: ""
     maxSSLExpiry: 7
     test:
       expr: 'code == 200'
```

To get started, let’s create a simple scraping configuration to let `config-db` scrape the configuration from our repository.


```
file:
 - type: $.kind
   id: $.metadata.name
   url: github.com/cishiv/sample-configuration
   paths:
     - simple-config.yaml

```
We can save this configuration as `scrape-git.yaml`. You will notice that for `type` and `id` we use the `$.` syntax. This lets `config-db` know to look for those fields in the scraped configuration. In our case, those fields should evaluate as follows:

`type` should be equal to “Canary”
`id` should be equal to “http-pass-single”

We can now have `config-db` run this scraper on a specified schedule. If we don’t specify a schedule, the scraper will run every 60 minutes by default.

To start `config-db` with this scraper configuration, run the following command in your terminal:

```
./.bin/config-db serve scrape-git.yaml –default-schedule=’@every 20s’
``` 

This will start `config-db` and run the scraper we’ve defined every 20 seconds.



We can now make a change to our configuration and push it to remote, and `config-db` will detect that configuration change and reflect it on the next scraper run.

Let’s change the `interval` field in our configuration from 40 to 30.

```
...
   canary: http
spec:
 interval: 30
...
```

Once `config-db` detects your change, you should see log output as follows:



We can easily view the output of the configuration changes using the HTTP API provided by `config-db`.

We can access the API for configuration changes by executing the following curl request:

```
% curl -s http://localhost:3000/config_changes | jq
[
  {
    "id": "0183cd72-c66f-6c48-f066-709ab9a8725a",
    "config_id": "0183cd5e-b709-d6e7-2478-5b0c6a89d51e",
    "external_change_id": "",
    "external_created_by": null,
    "change_type": "diff",
    "severity": "",
    "source": "",
    "summary": null,
    "patches": {
      "spec": {
        "interval": 30
      }
    },
    "details": {},
    "created_by": null,
    "created_at": "2022-10-12T20:26:34.735881"
  },
  {
    "id": "0183cd85-4cd9-aeca-8c4b-dc7663dbf9da",
    "config_id": "0183cd5e-b709-d6e7-2478-5b0c6a89d51e",
    "external_change_id": "",
    "external_created_by": null,
    "change_type": "diff",
    "severity": "",
    "source": "",
    "summary": null,
    "patches": {
      "spec": {
        "interval": 40
      }
    },
    "details": {},
    "created_by": null,
    "created_at": "2022-10-12T20:46:48.793886"
  }
]
```

We can see that all changes to our configuration have been detected and stored as patches by `config-db`.

Additionally, we can view our full configuration via the `config_items` API. Accessible via `http://localhost:3000/config_items`.

## Scraping using the CLI

So far, we’ve illustrated how to get `config-db` running and scrape on an ongoing basis. But that isn’t always necessary. In cases where you just want to scrape once-off, you can use the CLI.

We will continue using our `scrape-git.yaml` scraper configuration, but instead of using the `serve` command, we will use the `run` command as follows:

```
% ./.bin/config-db run scrape-git.yaml
INFO[0000] Loaded 7 config rules
2022-10-12T20:53:44.453+0200	INFO	Scrapping [scrape-git.yaml]
2022-10-12T20:53:44.496+0200	INFO	Initialized DB: localhost:5432/config (7959 kB)
2022-10-12 20:53:44.513877 I | goose: no migrations to run. current version: 99
2022-10-12T20:53:44.535+0200	INFO	Scraping files from (PWD: /Users/shiv/personal/flanksource/config-db)
2022-10-12T20:53:47.341+0200	INFO	Exporting 1 resources to DB
```
We can see that our scraper executed once, fetching our configuration from our repository, and then exited.

This can be useful for quickly updating configuration and verifying diffs.
## Next steps

In this guide, you’ve learnt what `config-db` is and how it can be useful to you. Additionally, you’ve seen how it can be used to scrape configuration from a Git repository.

We’ve also illustrated the usage of the HTTP API that is built into `config-db` to interrogate configuration items and their patches easily.

In our next guide - we illustrate how to install `config-db` via Helm into your Kubernetes cluster, as well as how to set it up for other types of configuration.


