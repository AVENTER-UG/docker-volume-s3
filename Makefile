#Dockerfile vars

#vars
PROJECTNAME=docker-volume-s3
DESCRIPTION=Docker volume driver for S3
UNAME_M=`uname -m`
TAG=v0.1.1
BRANCH=`git rev-parse --abbrev-ref HEAD`
BUILDDATE=`date -u +%Y-%m-%dT%H:%M:%SZ`
LICENSE=MIT
VERSION_TU=$(subst -, ,$(TAG:v%=%))
BUILD_VERSION=$(word 1,$(VERSION_TU))
DISTRO := $(shell lsb_release -is | tr '[:upper:]' '[:lower:]')$(shell lsb_release -rs | tr -d .)
PKG_REL = 1.$(DISTRO)

FPM_OPTS= -s dir -n $(PROJECTNAME) -v $(BUILD_VERSION) \
	--architecture $(UNAME_M) \
	--url "https://www.aventer.biz" \
	--license $(LICENSE) \
	--description "$(DESCRIPTION)" \
	--maintainer "AVENTER Support <support@aventer.biz>" \
	--vendor "AVENTER UG (haftungsbeschraenkt)" \
	--iteration $(PKG_REL)

CONTENTS= usr/bin etc usr/lib

.DEFAULT_GOAL := all

.PHONY: build
build:
	@echo ">>>> Build binary"
	@CGO_ENABLED=0 GOOS=linux go build -o build/$(PROJECTNAME) -a -installsuffix cgo -ldflags "-X main.BuildVersion=${BUILDDATE} -X main.GitVersion=${TAG} -extldflags \"-static\"" .

update-gomod:
	go get -u
	go mod tidy

sboom:
	syft dir:. > sbom.txt
	syft dir:. -o json > sbom.json

seccheck:
	grype --add-cpes-if-none .
	grype --add-cpes-if-none . > cve-report.md

deb: build
	@echo ">>>> Build DEB"
	@mkdir -p /tmp/toor/usr/bin
	@mkdir -p /tmp/toor/etc/docker-volume/
	@mkdir -p /tmp/toor/usr/lib/systemd/system
	@cp build/$(PROJECTNAME) /tmp/toor/usr/bin
	@cp build/$(PROJECTNAME).service /tmp/toor/usr/lib/systemd/system
	@cp build/s3.env /tmp/toor/etc/docker-volume/s3.env
	@fpm -t deb -C /tmp/toor/ --config-files etc $(FPM_OPTS) $(CONTENTS)

rpm: build
	@echo ">>>> Build RPM"
	@mkdir -p /tmp/toor/usr/bin
	@mkdir -p /tmp/toor/etc/docker-volume/
	@mkdir -p /tmp/toor/usr/lib/systemd/system
	@cp build/$(PROJECTNAME) /tmp/toor/usr/bin
	@cp build/$(PROJECTNAME).service /tmp/toor/usr/lib/systemd/system
	@cp build/s3.env /tmp/toor/etc/docker-volume/s3.env
	@fpm -t rpm -C /tmp/toor/ --config-files etc $(FPM_OPTS) $(CONTENTS)


all: sboom seccheck build

