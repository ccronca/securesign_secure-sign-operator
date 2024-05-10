#!/usr/bin/env bash

set -e

SKOPEO_CMD=${SKOPEO_CMD:-skopeo}
OPM_CMD=${OPM_CMD:-opm}
AUTH_FILE=${AUTH_FILE:-}

package_name="rhtas-operator"

helpFunction()
{
  echo -e "Usage: $0\n"
  echo -e "\t--help:   see all commands of this script\n"
  echo -e "\t--init-basic <OCP_minor> <yq|jq>:   initialize a new composite fragment\n\t  example: $0 --init-basic v4.13 yq\n"
  echo -e "\t--init-basic-all:   initialize all the fragments from production\n\t  example: $0 --init-basic-all\n"
  echo -e "\t--comment-graph <OCP_minor>:   add human readable bundle tags as comments to graph generated by --init-basic\n\t  example: $0 --comment-graph v4.13\n"
  echo -e "\t--render <OCP_minor> <brew>: render one FBC fragment\n\t\"brew\" optional parameter will made it consuming bundle images from the brew registry\n\t  example: $0 --render v4.13 brew\n"
  echo -e "\t--render-all <brew>: render all the FBC fragments\n\t\"brew\" optional parameter will made it consuming bundle images from the brew registry\n\t  example: $0 --render-all brew\n"
  exit 1
}

devfile()
{
    cat <<EOT > "$1"/devfile.yaml
schemaVersion: 2.2.0
metadata:
  name: fbc-$1
  displayName: FBC $1
  description: 'File based catalog'
  language: fbc
  provider: Red Hat
components:
  - name: image-build
    image:
      imageName: ""
      dockerfile:
        uri: catalog.Dockerfile
        buildContext: ""
  - name: kubernetes
    kubernetes:
      inlined: placeholder
    attributes:
      deployment/container-port: 50051
      deployment/cpuRequest: "100m"
      deployment/memoryRequest: 512Mi
      deployment/replicas: 1
      deployment/storageRequest: "0"
commands:
  - id: build-image
    apply:
      component: image-build
EOT
}

dockerfile()
{
  cat <<EOT > "$1"/catalog.Dockerfile
# The base image is expected to contain
# /bin/opm (with a serve subcommand) and /bin/grpc_health_probe
FROM registry.redhat.io/openshift4/ose-operator-registry:$1

ENTRYPOINT ["/bin/opm"]
CMD ["serve", "/configs", "--cache-dir=/tmp/cache"]

add catalog /configs
RUN ["/bin/opm", "serve", "/configs", "--cache-dir=/tmp/cache", "--cache-only"]

# Core bundle labels.

LABEL operators.operatorframework.io.bundle.mediatype.v1=registry+v1
LABEL operators.operatorframework.io.bundle.manifests.v1=manifests/
LABEL operators.operatorframework.io.bundle.metadata.v1=metadata/
LABEL operators.operatorframework.io.bundle.package.v1=rhtas-operator
LABEL operators.operatorframework.io.bundle.channels.v1=stable,stable-v1.0
LABEL operators.operatorframework.io.metrics.builder=operator-sdk-v1.34.1
LABEL operators.operatorframework.io.metrics.mediatype.v1=metrics+v1
LABEL operators.operatorframework.io.metrics.project_layout=go.kubebuilder.io/v3
LABEL operators.operatorframework.io.index.configs.v1=/configs

EOT
}

setBrew()
{
if [[ "$2" == "brew" ]]; then
    sed -i 's|image: registry.redhat.ioregistry.redhat.io/rhtas-tech-preview/sigstore-rhel9-operator|image: brew.registry.redhat.ioregistry.redhat.io/rhtas-tech-preview/sigstore-rhel9-operator|g' "${frag}"/graph.yaml
fi
}

unsetBrew()
{
if [[ "$2" == "brew" ]]; then
    sed -i 's|image: brew.registry.redhat.io/rhtas-tech-preview/sigstore-rhel9-operator|image: registry.redhat.io/rhtas-tech-preview/sigstore-rhel9-operator|g' "${frag}"/graph.yaml
    sed -i 's|brew.registry.redhat.io/rhtas-tech-preview/sigstore-rhel9-operator|registry.redhat.io/rhtas-tech-preview/sigstore-rhel9-operator|g' "${frag}"/catalog/rhtas-operator/catalog.json
fi
}


cmd="$1"
case $cmd in
  "--help")
    helpFunction
  ;;
  "--init-basic")
    frag=$2
    if [ -z "$frag" ]
    then
      echo "Please specify OCP minor, eg: v4.12"
      exit 1
    fi
    FROMV=$(grep FROM "${frag}"/catalog.Dockerfile)
    OCPV=${FROMV##*:}
    from=registry.redhat.io/redhat/redhat-operator-index:${OCPV}
    yqOrjq=$3
    mkdir -p "${frag}/catalog/rhtas-operator/" "${frag}/${frag}"
    touch "${frag}/${frag}/.empty"
    case $yqOrjq in
      "yq")
        touch "${frag}"/graph.yaml
        echo opm render $from -o yaml
	"${OPM_CMD}" render "$from" -o yaml | yq "select( .package == \"$package_name\" or .name == \"$package_name\")" | yq 'select(.schema == "olm.bundle") = {"schema": .schema, "image": .image}' | yq 'select(.schema == "olm.package") = {"schema": .schema, "name": .name, "defaultChannel": .defaultChannel}' > "${frag}"/graph.yaml
      ;;
      "jq")
        "${OPM_CMD}" render "$from" | jq "select( .package == \"$package_name\" or .name == \"$package_name\")" | jq 'if (.schema == "olm.bundle") then {schema: .schema, image: .image} else (if (.schema == "olm.package") then {schema: .schema, name: .name, defaultChannel: .defaultChannel} else . end) end' > "${frag}"/graph.json
      ;;
      *)
        echo "please specify if yq or jq"
        exit 1
      ;;
    esac
    devfile "$frag"
    dockerfile "$frag"
  ;;
  "--init-basic-all")
    for f in ./"v4."*; do
      frag=${f#./}
      $0 --init-basic "${frag}" yq
      $0 --comment-graph "${frag}"
    done
  ;;
  "--render")
    frag=$2
    if [ -z "$frag" ]
    then
      echo "Please specify OCP minor, eg: v4.12"
      exit 1
    fi
    setBrew "${frag}" "$3"
    "${OPM_CMD}" alpha render-template basic "${frag}"/graph.yaml > "${frag}"/catalog/rhtas-operator/catalog.json
    unsetBrew "${frag}" "$3"
  ;;
  "--render-all")
    for f in ./"v4."*; do
      frag=${f#./}
      setBrew "${frag}" "$2"
      "${OPM_CMD}" alpha render-template basic "${frag}"/graph.yaml > "${frag}"/catalog/rhtas-operator/catalog.json
      unsetBrew "${frag}" "$2"
    done
  ;;
  "--comment-graph")
    frag=$2
    if [ -z "$frag" ]
    then
      echo "Please specify OCP minor, eg: v4.12"
      exit 1
    fi
    setBrew "${frag}" "$3"
    sed -i "/# hco-bundle-registry v4\./d" "$frag"/graph.yaml
    grep -E "^image: [brew\.]*registry.redhat.io/rhtas-tech-preview/sigstore-rhel9-operator[-rhel9]*@sha256" "$frag"/graph.yaml | while read -r line ; do
      image=${line/image: /}
      echo "Processing $image"
      # shellcheck disable=SC2086
      url=$(${SKOPEO_CMD} inspect --no-tags ${AUTH_FILE} docker://"$image" | grep "\"url\": ")
      tag1=${url/*\/images\/}
      tag=${tag1/\",/}
      sed -i "s|$image|$image\n# hco-bundle-registry $tag|g" "$frag"/graph.yaml
    done
    unsetBrew "${frag}" "$3"
  ;;
  "--comment-graph-all")
    for f in ./"v4."*; do
      frag=${f#./}
      setBrew "${frag}" "$2"
      sed -i "/# hco-bundle-registry v4\./d" "$frag"/graph.yaml
      grep -E "^image: [brew\.]*registry.redhat.io/rhtas-tech-preview/sigstore-rhel9-operator[-rhel9]*@sha256" "$frag"/graph.yaml | while read -r line ; do
        image=${line/image: /}
        echo "Processing $image"
	# shellcheck disable=SC2086
        url=$(${SKOPEO_CMD} inspect --no-tags ${AUTH_FILE} docker://"$image" | grep "\"url\": ")
        tag1=${url/*\/images\/}
        tag=${tag1/\",/}
        sed -i "s|$image|$image\n# hco-bundle-registry $tag|g" "$frag"/graph.yaml
      done
      unsetBrew "${frag}" "$2"
    done
  ;;
  *)
    echo "$cmd not one of the allowed flags"
    helpFunction
  ;;
esac