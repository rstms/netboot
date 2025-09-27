# go makefile

program != basename $$(pwd)
version != cat VERSION
org = rstms

default: build

include make/common.mk

build: $(binary)

$(binary): fmt
	fix go build . ./...
	go build . ./...

fmt: go.sum
	fix go fmt . ./...

go.mod:
	go mod init

go.sum: go.mod
	go mod tidy

install: build
	go install

test: fmt
	go test -v -failfast . ./...

debug: fmt
	go test -v -failfast -count=1 -run $(test) . ./...

release: build
	$(gitclean)
	@$(if $(update),gh release delete -y v$(version),)
	gh release create v$(version) --notes "v$(version)"

dist: dist/$(release_binary)

dist/$(release_binary): $(binary)
	mkdir -p dist
	cp $< $@
	scp $@ $(dist_host):$(dist_dir)/$(notdir $@)

release-upload: dist
	cd dist; gh release upload $(latest_release) $(release_binary) $(CLOBBER)

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
	rm -rf ~/.cache/netboot/ipxe
	mkdir -p ~/.cache/netboot/ipxe
	chown -R $(USER):$(USER) ~/.cache

sterile: clean
	go clean -cache
	go clean -modcache
	rm -f go.mod go.sum
	rm -rf ~/.cache/netboot
	rm -rf cmd/certs/*
	touch cmd/certs/.placeholder

gen:
	@cd template && $(MAKE)

regen: clean gen

show-vars:
	@$(foreach var,$(all_variables),echo $(var)=$($(var));)
