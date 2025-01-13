load('ext://git_resource', 'git_checkout')

# Clone the external repository
git_checkout('git@github.com:liatrio/tag-o11y-quick-start-manifests.git#main', 'quickstarts')


# Load the Tiltfile from the cloned repository
include('quickstarts/apps/default/Tiltfile')

# To run: `tilt trigger dockerbuild` or via trigger button 🔃 in the UI
local_resource(
  "dockerbuild",
  cmd="make dockerbuild BUILD_IN_TILT=true",
  trigger_mode=TRIGGER_MODE_MANUAL,
  auto_init=False,
  labels=["makefile"],
)

k8s_yaml(kustomize("./manifests/overlays/local/"))

k8s_resource(
  workload="${{ values.binaryName }}",
  port_forwards=8080,
  labels=[
    "dora-api-intermediary"
  ]
)

k8s_resource(
  new_name="otelcol-collector",
  objects=[
    "otelcol:OpenTelemetryCollector:${{ values.binaryName }}",
  ],
  labels=[
    "${{ values.binaryName }}"
  ],
  resource_deps=[
    "opentelemetry-operator-controller-manager"
  ]
)
