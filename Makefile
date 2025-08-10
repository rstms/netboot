# go makefile

program != basename $$(pwd)

go_version = go1.24.5

latest_release != gh release list --json tagName --jq '.[0].tagName' | tr -d v
version != cat VERSION

rstms_modules = $(shell awk <go.mod '/^module/{next} /rstms/{print $$1}')

gitclean = $(if $(shell git status --porcelain),$(error git status is dirty),$(info git status is clean))

template_files = template/ipxe/BOOTX64.EFI template/ipxe/autoexec.ipxe template/certs/keymaster.pem

$(program): build

build: fmt $(template_files)

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

update:
	@echo updating modules
	$(foreach module,$(rstms_modules),go get $(module)@latest;)

mirrors:
	scripts/update_mirrors
	find template -type d -exec chmod 0755 \{\} \;
	find template -type f -exec chmod 0644 \{\} \;
	find template/pub/OpenBSD -type f -name install??.??? -exec mv \{\} template/openbsd \;

clean:
	rm -f $(program) *.core 
	go clean
	rm -rf ~/.cache/netboot
	mkdir ~/.cache/netboot

sterile: clean
	which $(program) && go clean -i || true
	go clean
	go clean -cache
	go clean -modcache
	rm -f go.mod go.sum

template/ipxe/BOOTX64.EFI: template/ipxe/netboot.xyz.efi
	cp $< $@

template/ipxe/autoexec.ipxe: template/ipxe/menu.ipxe
	cp $< $@

template/certs/keymaster.pem: /etc/ssl/keymaster.pem
	cp $< $@

