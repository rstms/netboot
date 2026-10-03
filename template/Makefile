#
# download OS installer resources
#

default: build

# OS detection flags
windows = $(if $(SYSTEMROOT),1,)
openbsd = $(if $(shell uname | grep -i openbsd),1,)
linux = $(if $(shell uname | grep -i linux),1,)

openbsd_versions = 7.7 7.8 7.9
debian_versions = bookworm trixie
devuan_versions = excalibur daedalus
alpine_versions = 3.23.6 3.24.2

# functions for running only on selected OS
openbsd_exec = $(if $(openbsd),$(1),@echo "not on openbsd, skipping: $(1)")
linux_exec = $(if $(linux),$(1),@echo "not on linux, skipping: $(1)")
windows_exec = $(if $(windows),$(1),@echo "not on windows, skipping: $(1)")

debian_mirror = ftp.us.debian.org
devuan_mirror = deb.devuan.org
openbsd_mirror = cdn.openbsd.org/pub/OpenBSD
alpine_mirror = dl-cdn.alpinelinux.org

dist_root = ~/.cache/netboot/dist

dist_upload_host = $(shell [ -e ~/.dist_upload_host ] && cat ~/.dist_upload_host)

# ipxe-files are generated on phobos.rstms.net using github.com/rstms/rstms-netboot-xyz 
ipxe-files = ipxe/rstms-netboot.efi.gz ipxe/rstms-netboot.img.gz ipxe/rstms-netboot.iso.gz

# OS distribution files pulled from repositories
debian-12-kernel = $(dist_root)/debian/bookworm/amd64/linux
debian-12-initrd = $(dist_root)/debian/bookworm/amd64/initrd.gz

debian-13-kernel = $(dist_root)/debian/trixie/amd64/linux
debian-13-initrd = $(dist_root)/debian/trixie/amd64/initrd.gz

devuan-5-kernel = $(dist_root)/devuan/daedalus/amd64/linux
devuan-5-initrd = $(dist_root)/devuan/daedalus/amd64/initrd.gz

devuan-6-kernel = $(dist_root)/devuan/excalibur/amd64/linux
devuan-6-initrd = $(dist_root)/devuan/excalibur/amd64/initrd.gz

openbsd-77-iso = $(dist_root)/pub/OpenBSD/7.7/amd64/cd77.iso
openbsd-77-img = $(dist_root)/pub/OpenBSD/7.7/amd64/miniroot77.img

openbsd-77-netboot-iso = ipxe/openbsd-7.7-amd64.iso.gz
openbsd-77-netboot-img = ipxe/openbsd-7.7-amd64.img.gz

openbsd-78-iso = $(dist_root)/pub/OpenBSD/7.8/amd64/cd78.iso
openbsd-78-img = $(dist_root)/pub/OpenBSD/7.8/amd64/miniroot78.img

openbsd-78-netboot-iso = ipxe/openbsd-7.8-amd64.iso.gz
openbsd-78-netboot-img = ipxe/openbsd-7.8-amd64.img.gz

openbsd-79-iso = $(dist_root)/pub/OpenBSD/7.9/amd64/cd79.iso
openbsd-79-img = $(dist_root)/pub/OpenBSD/7.9/amd64/miniroot79.img

openbsd-79-netboot-iso = ipxe/openbsd-7.9-amd64.iso.gz
openbsd-79-netboot-img = ipxe/openbsd-7.9-amd64.img.gz

alpine-3236-kernel = $(dist_root)/alpine/3.23.6/x86_64/kernel
alpine-3236-initrd = $(dist_root)/alpine/3.23.6/x86_64/initrd
alpine-3236-modloop = $(dist_root)/alpine/3.23.6/x86_64/modloop

alpine-3242-kernel = $(dist_root)/alpine/3.24.2/x86_64/kernel
alpine-3242-initrd = $(dist_root)/alpine/3.24.2/x86_64/initrd
alpine-3242-modloop = $(dist_root)/alpine/3.24.2/x86_64/modloop

windows-11-iso = $(dist_root)/windows/11/x64/Win11_25H2_English_x64_v2.iso

curl_download = mkdir -p $(dir $(2)) && echo >>dist_init_files "$(1) $(2)" && curl -L $(1) --remote-time -o $(2) && test -s $(2)

openbsd-dist-files = \
	$(openbsd-77-iso) \
	$(openbsd-77-img) \
	$(openbsd-78-iso) \
	$(openbsd-78-img) \
	$(openbsd-79-iso) \
	$(openbsd-79-img)

openbsd-netboot-iso-files = \
	$(openbsd-77-netboot-iso) \
	$(openbsd-78-netboot-iso) \
	$(openbsd-79-netboot-iso)

openbsd-netboot-img-files = \
	$(openbsd-77-netboot-img) \
	$(openbsd-78-netboot-img) \
	$(openbsd-79-netboot-img)

debian-dist-files = \
	$(debian-12-kernel) $(debian-12-initrd) \
	$(debian-13-kernel) $(debian-13-initrd) 

devuan-dist-files = \
	$(devuan-5-kernel) $(devuan-5-initrd) \
	$(devuan-6-kernel) $(devuan-6-initrd) 

alpine-dist-files = \
	$(alpine-3236-kernel) $(alpine-3236-initrd) $(alpine-3236-modloop) \
	$(alpine-3242-kernel) $(alpine-3242-initrd) $(alpine-3242-modloop) 

windows-dist-files = \
	$(windows-11-iso)

dist-files = $(openbsd-dist-files) $(debian-dist-files) $(devuan-dist-files) $(alpine-dist-files) $(windows-dist-files)

generated-files = $(openbsd-netboot-iso-files) $(openbsd-netboot-img-files) 


#	$(foreach version,$(openbsd_versions),scripts/populate_openbsd_dist $(dist_root) $(openbsd_mirror) $(version) amd64;)
openbsd-dist-dirs = $(foreach version,$(openbsd_versions),$(dist_root)/pub/OpenBSD/$(version))

$(openbsd-dist-dirs):
	scripts/populate_openbsd_dist $(dist_root) $(openbsd_mirror) $(notdir $@) amd64

$(openbsd-dist-files): $(openbsd-dist-dirs)

$(win11-iso):
	@scripts/update_winiso $(dist_upload_host) $(notdir $@) iso download

$(debian-12-kernel):
	$(call curl_download,http://$(debian_mirror)/debian/dists/bookworm/main/installer-amd64/current/images/netboot/debian-installer/amd64/linux,$@)

$(debian-12-initrd):
	$(call curl_download,http://$(debian_mirror)/debian/dists/bookworm/main/installer-amd64/current/images/netboot/debian-installer/amd64/initrd.gz,$@)

$(debian-13-kernel):
	$(call curl_download,http://$(debian_mirror)/debian/dists/trixie/main/installer-amd64/current/images/netboot/debian-installer/amd64/linux,$@)

$(debian-13-initrd):
	$(call curl_download,http://$(debian_mirror)/debian/dists/trixie/main/installer-amd64/current/images/netboot/debian-installer/amd64/initrd.gz,$@)

$(devuan-5-kernel):
	$(call curl_download,http://$(devuan_mirror)/devuan/dists/daedalus/main/installer-amd64/current/images/netboot/debian-installer/amd64/linux,$@)

$(devuan-5-initrd):
	$(call curl_download,http://$(devuan_mirror)/devuan/dists/daedalus/main/installer-amd64/current/images/netboot/debian-installer/amd64/initrd.gz,$@)

$(devuan-6-kernel):
	$(call curl_download,http://$(devuan_mirror)/devuan/dists/excalibur/main/installer-amd64/current/images/netboot/debian-installer/amd64/linux,$@)

$(devuan-6-initrd):
	$(call curl_download,http://$(devuan_mirror)/devuan/dists/excalibur/main/installer-amd64/current/images/netboot/debian-installer/amd64/initrd.gz,$@)

$(alpine-3236-kernel):
	$(call curl_download,https://$(alpine_mirror)/v3.23/releases/x86_64/netboot-3.23.6/vmlinuz-virt,$@)

$(alpine-3236-initrd):
	$(call curl_download,https://$(alpine_mirror)/v3.23/releases/x86_64/netboot-3.23.6/initramfs-virt,$@)

$(alpine-3236-modloop):
	$(call curl_download,https://$(alpine_mirror)/v3.23/releases/x86_64/netboot-3.23.6/modloop-virt,$@)

$(alpine-3242-kernel):
	$(call curl_download,https://$(alpine_mirror)/v3.24/releases/x86_64/netboot-3.24.2/vmlinuz-virt,$@)

$(alpine-3242-initrd):
	$(call curl_download,https://$(alpine_mirror)/v3.24/releases/x86_64/netboot-3.24.2/initramfs-virt,$@)

$(alpine-3242-modloop):
	$(call curl_download,https://$(alpine_mirror)/v3.24/releases/x86_64/netboot-3.24.2/modloop-virt,$@)

certs/cacerts.tgz: $(ca_certs)
	cd certs/debian && tar zcfv ../cacerts.tgz etc

$(openbsd-77-netboot-iso): $(openbsd-77-iso) $(wildcard mkboot/*.openbsd) $(wildcard scripts/*)
	$(call openbsd_exec,scripts/generate_openbsd_netboot 7.7 amd64 iso)

$(openbsd-77-netboot-img): $(openbsd-77-img) $(wildcard mkboot/*.openbsd) $(wildcard scripts/*)
	$(call openbsd_exec,scripts/generate_openbsd_netboot 7.7 amd64 img)

$(openbsd-78-netboot-iso): $(openbsd-78-iso) $(wildcard mkboot/*.openbsd) $(wildcard scripts/*)
	$(call openbsd_exec,scripts/generate_openbsd_netboot 7.8 amd64 iso)

$(openbsd-78-netboot-img): $(openbsd-78-img) $(wildcard mkboot/*.openbsd) $(wildcard scripts/*)
	$(call openbsd_exec,scripts/generate_openbsd_netboot 7.8 amd64 img)

$(openbsd-79-netboot-iso): $(openbsd-79-iso) $(wildcard mkboot/*.openbsd) $(wildcard scripts/*)
	$(call openbsd_exec,scripts/generate_openbsd_netboot 7.9 amd64 iso)

$(openbsd-79-netboot-img): $(openbsd-79-img) $(wildcard mkboot/*.openbsd) $(wildcard scripts/*)
	$(call openbsd_exec,scripts/generate_openbsd_netboot 7.9 amd64 img)

$(windows-11-iso):
	mkdir -p $(dir $@)
	ln ~/iso/$(notdir $@) $@

$(ipxe-files):
	scp phobos:src/rstms-netboot-xyz/dist/ipxe/$(basename $(notdir $@)) ipxe/$(basename $(notdir $@))
	gzip $(basename $@)


dist: $(dist-files) $(ipxe-files)

redist: distclean dist

gen: dist $(generated-files)

build: gen

rebuild: clean gen

regen: distclean gen 

# only clean files that can be regenerated
clean: 
	rm -f $(generated-files)
	rm -f ipxe/*.rd

distclean: clean
	rm -f $(ipxe-files)
	rm -rf $(dist_root)
	mkdir $(dist_root)
	rm -f dist_init_files

testo:
	@for FILE in $(dist-files); do echo $$FILE; done
