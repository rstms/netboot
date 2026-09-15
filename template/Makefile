#
# download OS installer resources
#

default: gen

# OS detection flags
windows = $(if $(SYSTEMROOT),1,)
openbsd = $(if $(shell uname | grep -i openbsd),1,)
linux = $(if $(shell uname | grep -i linux),1,)

# functions for running only on selected OS
openbsd_exec = $(if $(openbsd),$(1),@echo "not on openbsd, skipping: $(1)")
linux_exec = $(if $(linux),$(1),@echo "not on linux, skipping: $(1)")
windows_exec = $(if $(windows),$(1),@echo "not on windows, skipping: $(1)")

debian_mirror = ftp.us.debian.org
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

openbsd-77-iso = $(dist_root)/OpenBSD/7.7/amd64/cd77.iso
openbsd-77-img = $(dist_root)/OpenBSD/7.7/amd64/miniroot77.img

openbsd-77-netboot-iso = ipxe/openbsd-7.7-amd64.iso.gz
openbsd-77-netboot-img = ipxe/openbsd-7.7-amd64.img.gz

openbsd-78-iso = $(dist_root)/OpenBSD/7.8/amd64/cd78.iso
openbsd-78-img = $(dist_root)/OpenBSD/7.8/amd64/miniroot78.img

openbsd-78-netboot-iso = ipxe/openbsd-7.8-amd64.iso.gz
openbsd-78-netboot-img = ipxe/openbsd-7.8-amd64.img.gz

openbsd-79-iso = $(dist_root)/OpenBSD/7.9/amd64/cd79.iso
openbsd-79-img = $(dist_root)/OpenBSD/7.9/amd64/miniroot79.img

openbsd-79-netboot-iso = ipxe/openbsd-7.9-amd64.iso.gz
openbsd-79-netboot-img = ipxe/openbsd-7.9-amd64.img.gz

alpine-3221-kernel = $(dist_root)/alpine/3.22.1/x86_64/kernel
alpine-3221-initrd = $(dist_root)/alpine/3.22.1/x86_64/initrd
alpine-3221-modloop = $(dist_root)/alpine/3.22.1/x86_64/modloop

alpine-3234-kernel = $(dist_root)/alpine/3.23.4/x86_64/kernel
alpine-3234-initrd = $(dist_root)/alpine/3.23.4/x86_64/initrd
alpine-3234-modloop = $(dist_root)/alpine/3.23.4/x86_64/modloop

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

alpine-dist-files = \
	$(alpine-3221-kernel) $(alpine-3221-initrd) \
	$(alpine-3221-modloop) $(alpine-3234-kernel) \
	$(alpine-3234-initrd) $(alpine-3234-modloop) 

windows-dist-files = \
	$(windows-11-iso)

dist-files = $(openbsd-dist-files) $(debian-dist-files) $(alpine-dist-files) $(windows-dist-files)

generated-files = $(openbsd-netboot-iso-files) $(openbsd-netboot-img-files) 


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

$(openbsd-dist-files):
	$(call curl_download,https://$(openbsd_mirror)/$(word 7,$(subst /, ,$@))/$(word 8,$(subst /, ,$@))/$(notdir $@),$@)

#$(openbsd-77-iso):
#	$(call curl_download,$(call openbsd_url,$@),$@)
#
#$(openbsd-77-img):
#	$(call curl_download,$(call openbsd_url,$@),$@)
#
#$(openbsd-78-iso):
#	$(call curl_download,$(call openbsd_url,$@),$@)
#
#$(openbsd-78-img):
#	$(call curl_download,$(call openbsd_url,$@),$@)
#
#$(openbsd-79-iso):
#	$(call curl_download,$(call openbsd_url,$@),$@)
#
#$(openbsd-79-img):
#	$(call curl_download,$(call openbsd_url,$@),$@)


$(alpine-3221-kernel):
	$(call curl_download,https://$(alpine_mirror)/v3.22/releases/x86_64/netboot-3.22.1/vmlinuz-virt,$@)

$(alpine-3221-initrd):
	$(call curl_download,https://$(alpine_mirror)/v3.22/releases/x86_64/netboot-3.22.1/initramfs-virt,$@)

$(alpine-3221-modloop):
	$(call curl_download,https://$(alpine_mirror)/v3.22/releases/x86_64/netboot-3.22.1/modloop-virt,$@)

$(alpine-3234-kernel):
	$(call curl_download,https://$(alpine_mirror)/v3.23/releases/x86_64/netboot-3.23.4/vmlinuz-virt,$@)

$(alpine-3234-initrd):
	$(call curl_download,https://$(alpine_mirror)/v3.23/releases/x86_64/netboot-3.23.4/initramfs-virt,$@)

$(alpine-3234-modloop):
	$(call curl_download,https://$(alpine_mirror)/v3.23/releases/x86_64/netboot-3.23.4/modloop-virt,$@)

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

testo:
	@for FILE in $(dist-files); do echo $$FILE; done
