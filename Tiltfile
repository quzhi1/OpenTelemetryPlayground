# -*- mode: Python -*-

load('ext://restart_process', 'docker_build_with_restart')
load('ext://helm_resource', 'helm_resource')

compile_opt = 'GO111MODULE=on CGO_ENABLED=0 GOOS=linux GOARCH=amd64 '

# Compile example application
local_resource(
  'fiber-otel-compile',
  compile_opt + 'go build -o bin/fiber-otel fiber-otel/server.go',
  deps=['fiber-otel/server.go'],
  ignore=['bin', 'helm', 'Dockerfile', 'Tiltfile', 'README.md', 'LICENSE', '.gitignore'],
  labels="fiber-otel",
)

# Build example docker image
docker_build_with_restart(
  'fiber-otel-image',
  '.',
  entrypoint=['/opt/app/bin/fiber-otel'],
  dockerfile='Dockerfile',
  only=[
    './bin',
  ],
  live_update=[
    sync('./bin', '/opt/app/bin'),
  ],
)

# Install example helm chart
helm_resource(
  'fiber-otel-service',
  'helm',
  image_deps=['fiber-otel-image'],
  image_keys=[('image.repository', 'image.tag')],
  port_forwards=['3000:3000'],
  labels="fiber-otel",
)
