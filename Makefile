PLUGIN_NAME=cloudfoundry

ifndef _ARCH
_ARCH := $(shell ./print_arch)
export _ARCH
endif

ifndef REGISTRY
REGISTRY := ""
endif

IMAGE=swisscom/waypoint-plugin-cloudfoundry
TAG := $(shell ./generate-tag.sh)

.PHONY: all

all: protos build_all

# Generate the Go code from Protocol Buffer definitions
protos:
	@echo ""
	@echo "Build Protos"

	protoc -I . --go_out=plugins=grpc:. --go_opt=paths=source_relative ./platform/output.proto
	protoc -I . --go_out=plugins=grpc:. --go_opt=paths=source_relative ./release/output.proto

# Builds the plugin on your local machine
build:
	@echo ""
	@echo "Compile Plugin for current target"

	# Clear the output
	rm -rf ./bin
	GOARCH=amd64 go build -o ./bin/${_ARCH}_amd64/waypoint-plugin-${PLUGIN_NAME} ./main.go

build_all:
	@echo ""
	@echo "Compile Plugin for all targets"

	# Clear the output
	rm -rf ./bin

	GOOS=linux GOARCH=amd64 go build -o ./bin/linux_amd64/waypoint-plugin-${PLUGIN_NAME} ./main.go
	GOOS=darwin GOARCH=amd64 go build -o ./bin/darwin_amd64/waypoint-plugin-${PLUGIN_NAME} ./main.go
	GOOS=windows GOARCH=amd64 go build -o ./bin/windows_amd64/waypoint-plugin-${PLUGIN_NAME}.exe ./main.go
	GOOS=windows GOARCH=386 go build -o ./bin/windows_386/waypoint-plugin-${PLUGIN_NAME}.exe ./main.go

# Install the plugin locally
install:
	@echo ""
	@echo "Installing Plugin"

	@if [ "${_ARCH}" = "darwin" ]; then\
		cp ./bin/${_ARCH}_amd64/waypoint-plugin-${PLUGIN_NAME}* ~/Library/Preferences/waypoint/plugins/;\
	else\
		cp ./bin/${_ARCH}_amd64/waypoint-plugin-${PLUGIN_NAME}* ${HOME}/.config/waypoint/plugins/;\
	fi

# Zip the built plugin binaries
zip:
	zip -j ./bin/waypoint-plugin-${PLUGIN_NAME}_linux_amd64.zip ./bin/linux_amd64/waypoint-plugin-${PLUGIN_NAME}
	zip -j ./bin/waypoint-plugin-${PLUGIN_NAME}_darwin_amd64.zip ./bin/darwin_amd64/waypoint-plugin-${PLUGIN_NAME}
	zip -j ./bin/waypoint-plugin-${PLUGIN_NAME}_windows_amd64.zip ./bin/windows_amd64/waypoint-plugin-${PLUGIN_NAME}.exe
	zip -j ./bin/waypoint-plugin-${PLUGIN_NAME}_windows_386.zip ./bin/windows_386/waypoint-plugin-${PLUGIN_NAME}.exe

# Build the plugin using a Docker container
build-docker:
	rm -rf ./releases
	DOCKER_BUILDKIT=1 docker build --output releases --progress=plain .
