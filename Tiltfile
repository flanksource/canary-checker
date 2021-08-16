# Build: tell Tilt what images to build from which directories
# docker_build( './', dockerfile=Dockerfile.dev)
default_registry('ttl.sh')
custom_build(
  'docker.io/flanksource/canary-checker',
  'make linux && docker build -t $EXPECTED_REF . -f Dockerfile',
  ['pkg'],
)
