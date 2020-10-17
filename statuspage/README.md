# StatusPage

The Vue project for the canary-checker front-end.

## Rebuild and Package Front-End

To rebuild and pakage into main canary checker application:

```bash
# (In the main repo directory)
make vue-dist
```

## Project setup for local development

For local development, in the `statuspage` directory:

> **Requirements:**
> * node,
> * npm
 
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
