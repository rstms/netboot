# go makefile

program != basename $$(pwd)

go_version = go1.24.5

latest_release != gh release list --json tagName --jq '.[0].tagName' | tr -d v
version != cat VERSION

rstms_modules = $(shell awk <go.mod '/^module/{next} /rstms/{print $$1}')

gitclean = $(if $(shell git status --porcelain),$(error git status is dirty),$(info git status is clean))

template_files = template/certs/keymaster.pem 

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
	cd template && gmake -j 7 
	scripts/update_debian_initrd

clean:
	rm -f $(program) *.core 
	go clean
	rm -rf ~/.cache/netboot
	mkdir ~/.cache/netboot
	rm -rf template/certs/*
	touch template/certs/.placeholder
	rm -f template/robots.txt template/wget-log*

sterile: clean
	which $(program) && go clean -i || true
	go clean
	go clean -cache
	go clean -modcache
	rm -f go.mod go.sum

template/certs/keymaster.pem: /etc/ssl/keymaster.pem
	cp $< $@

test-openbsd-mkboot:
	scripts/test_openbsd_mkboot

