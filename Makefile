# go makefile

program := $(shell basename $$(pwd))
version := $(shell cat VERSION)

default: build

include make/common.mk

build: $(binary)

$(binary): .fmt
	fix go build . ./...
	go build

go_src := $(wildcard *.go) $(wildcard **/*.go)

.fmt: $(go_src) go.sum
	fix go fmt . ./...
	touch $@

fmt: .fmt

go.mod:
	go mod init

go.sum: go.mod
	go mod tidy

install: $(binary)
	go install

test: .fmt testfiles
	go test -v -failfast . ./...

debug: .fmt testfiles
	go test -v -failfast -count=1 -run $(test) . ./...

release: $(binary)
	$(gitclean)
	gh release create v$(version) --notes "v$(version)"

update-release:
	gh release delete -y v$(version)
	$(MAKE) release


dist/$(release_binary): $(binary)
	$(gitclean)
	cp $< $@
	$(call dist_upload,$<,$@)
	$(call dist_upload,$<,dist/$(dist_binary))
	cd dist; gh release upload $(latest_release) $(release_binary) --clobber
	touch $@

upload: dist/$(release_binary)

update-modules:
	@echo checking dependencies for updated versions 
	$(foreach module,$(rstms_modules),go get $(module)@$(call latest_module_release,$(module));)
	curl -Lso .proxy https://raw.githubusercontent.com/rstms/go-common/master/proxy_common_go
	$(foreach s,$(common_go),sed <.proxy >$(s) 's/^package cmd/package $(lastword $(subst /, ,$(dir $(s))))/'; ) rm .proxy
	$(MAKE)

clean:
	rm -f $(binary) *.core 
	go clean
	rm -rf /tmp/netboot*
	rm -rf dist && mkdir dist
	rm -rf $(cache_dir)/ipxe
	mkdir -p $(cache_dir)/ipxe
	mkdir -p $(config_dir)
	$(if $(windows),,chown -R $(USER):$(USER) $(config_dir))

sterile: clean
	go clean -cache
	go clean -modcache
	rm -f go.mod go.sum
	rm -rf $(cache_dir)
	rm -rf cmd/certs/*
	touch cmd/certs/.placeholder

gen:
	@cd template && $(MAKE) gen

regen:
	@cd template && $(MAKE) regen

show-vars:
	@$(foreach var,$(all_variables),echo $(var)=$($(var));)

testfiles: server/testdata/BOOTX64.EFI

server/testdata/BOOTX64.EFI: template/ipxe/netboot.xyz.efi.gz
	$(if $(openbsd),gzcat,zcat) <$< >$@

run:
	./netboot -dL- server
