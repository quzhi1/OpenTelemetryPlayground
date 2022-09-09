# -*- mode: Python -*-

load('ext://restart_process', 'docker_build_with_restart')
load('ext://helm_resource', 'helm_resource', 'helm_repo')

compile_opt = 'GO111MODULE=on CGO_ENABLED=0 GOOS=linux GOARCH=amd64 '

#################### OpenTelemetry collector ##################
helm_repo('open-telemetry', 'https://open-telemetry.github.io/opentelemetry-helm-charts')
helm_resource(
  'my-opentelemetry-collector',
  'open-telemetry/opentelemetry-collector',
  flags=[
    '-f',
    'collector/developer.values.yaml',
  ],
  deps=['collector/developer.values.yaml'],
  resource_deps=['open-telemetry'],
  labels='collector',
)

#################### api-a ######################

# Compile api-a binary
local_resource(
  'api-a-compile',
  compile_opt + 'go build -o bin/api-a api-a/server.go',
  deps=['api-a/server.go'],
  ignore=['bin'],
  labels="api-a",
)

# Build api-b docker image
docker_build_with_restart(
  'api-a-image',
  '.',
  entrypoint=['/opt/app/bin/api-a'],
  dockerfile='api-a/Dockerfile',
  only=[
    './bin',
  ],
  live_update=[
    sync('./bin', '/opt/app/bin'),
  ],
)

# Install example helm chart
helm_resource(
  'api-a-service',
  'api-a/helm',
  image_deps=['api-a-image'],
  image_keys=[('image.repository', 'image.tag')],
  port_forwards=['3000:3000'],
  labels="api-a",
)

#################### api-b ######################

# Compile api-b binary
local_resource(
  'api-b-compile',
  compile_opt + 'go build -o bin/api-b api-b/server.go',
  deps=['api-b/server.go'],
  ignore=['bin'],
  labels="api-b",
)

# Build api-b docker image
docker_build_with_restart(
  'api-b-image',
  '.',
  entrypoint=['/opt/app/bin/api-b'],
  dockerfile='api-b/Dockerfile',
  only=[
    './bin',
  ],
  live_update=[
    sync('./bin', '/opt/app/bin'),
  ],
)

# Install example helm chart
helm_resource(
  'api-b-service',
  'api-b/helm',
  image_deps=['api-b-image'],
  image_keys=[('image.repository', 'image.tag')],
  port_forwards=['3010:3010'],
  labels="api-b",
)