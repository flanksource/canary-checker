# StatusPage

The Vue project for the canary-checker front-end.

> **Requirements:**
> * node,
> * npm
 

## Rebuild and Package Front-End

To rebuild and pakage into main canary checker application:

```bash
# (In the main repo directory)
make vue-dist
```

## Project setup for local development

For local development, in the `statuspage` directory:


### Initialize:

Let npm download all required libraries:

```bash 
npm install
```

### Run a local development server

Run a local node front-end server that compiles and hot-reloads for development:
```
npm run serve
```

Run a local canary-checker back-end:

```bash
# in the repo root
make build
./.bin/canary-checker serve -c fixtures/gui_test.yaml --interval=5 --dev=true
```

### Do a manual distribution rebuild

Compiles and minify projet for release
```
npm run build
```

### Lints and fixes files
```
npm run lint
```

### Customize configuration
See [Configuration Reference](https://cli.vuejs.org/config/).
