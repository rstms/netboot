# go makefile

program != basename $$(pwd)

go_version = go1.24.5

latest_release != gh release list --json tagName --jq '.[0].tagName' | tr -d v
version != cat VERSION

rstms_modules = $(shell awk <go.mod '/^module/{next} /rstms/{print $$1}')

gitclean = $(if $(shell git status --porcelain),$(error git status is dirty),$(info git status is clean))

openbsd_netboot_iso = template/ipxe/openbsd-7.5-amd64.iso template/ipxe/openbsd-7.6-amd64.iso template/ipxe/openbsd-7.7-amd64.iso 
keymaster = template/certs/keymaster.pem
debian_cacerts = template/certs/cacerts.tgz

generated_template_files = $(debian_cacerts) $(openbsd_netboot_iso)

$(program): build

gen: $(generated_template_files)

regen: 
	rm -f $(generated_template_files)
	$(MAKE) gen

build: fmt gen
	fix go build . ./...
	go build

fmt: go.sum
	fix go fmt . ./...

go.mod:
	$(go_version) mod init

go.sum: go.mod
	go mod tidy

install: build
	go install

test: fmt
	go test -v -failfast . ./...

debug: fmt
	go test -v -failfast -count=1 -run $(test) . ./...

release:
	$(gitclean)
	@$(if $(update),gh release delete -y v$(version),)
	gh release create v$(version) --notes "v$(version)"

latest_module_release = $(shell gh --repo $(1) release list --json tagName --jq '.[0].tagName')

update:
	@echo checking dependencies for updated versions 
	@$(foreach module,$(rstms_modules),go get $(module)@$(call latest_module_release,$(module));)
	curl -L -o cmd/common.go https://raw.githubusercontent.com/rstms/go-common/master/proxy_common_go
	sed <cmd/common.go >server/common.go 's/^package cmd/package server/'

mirrors:
	cd template && gmake -j 7 


clean-mirrors:
	find template/dist -type f -not -name 'gdl??.tgz' -exec rm \{\} \;

clean:
	rm -f $(program) *.core 
	go clean
	rm -rf /tmp/netboot*
	rm -rf ~/.cache/netboot/ipxe
	mkdir ~/.cache/netboot/ipxe
	rm -f template/robots.txt template/wget-log*
	rm -rf template/certs/debian

sterile: clean
	which $(program) && go clean -i || true
	go clean
	go clean -cache
	go clean -modcache
	rm -f go.mod go.sum
	rm -rf ~/.cache/netboot
	rm -rf template/certs/*
	touch template/certs/.placeholder

$(keymaster): /etc/ssl/keymaster.pem
	cp $< $@

$(debian_cacerts): $(keymaster)
	scripts/hash_debian_cacerts

template/ipxe/openbsd-7.5-amd64.iso: template/dist/openbsd/7.5/amd64/cd75.iso $(wildcard template/mkboot/*.openbsd)
	scripts/generate_openbsd_netboot_iso 7.5 amd64

template/ipxe/openbsd-7.6-amd64.iso: template/dist/openbsd/7.6/amd64/cd76.iso $(wildcard template/mkboot/*.openbsd)
	scripts/generate_openbsd_netboot_iso 7.6 amd64

template/ipxe/openbsd-7.7-amd64.iso: template/dist/openbsd/7.7/amd64/cd77.iso $(wildcard template/mkboot/*.openbsd)
	scripts/generate_openbsd_netboot_iso 7.7 amd64

run:
	./netboot -vl-
