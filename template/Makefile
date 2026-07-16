#
# download OS installer resources
#

# OS detection flags
windows = $(if $(SYSTEMROOT),1,)
openbsd = $(if $(shell uname | grep -i openbsd),1,)
linux = $(if $(shell uname | grep -i linux),1,)

# functions for running only on selected OS
openbsd_exec = $(if $(openbsd),$(1),@echo "not on openbsd, skipping: $(1)")
linux_exec = $(if $(linux),$(1),@echo "not on linux, skipping: $(1)")
windows_exec = $(if $(windows),$(1),@echo "not on windows, skipping: $(1)")

debian_mirror = ftp.us.debian.org

debian-12-kernel = dist/debian/bookworm/amd64/linux
debian-12-initrd = dist/debian/bookworm/amd64/initrd.gz

debian-13-kernel = dist/debian/trixie/amd64/linux
debian-13-initrd = dist/debian/trixie/amd64/initrd.gz

openbsd_mirror = ftp.openbsd.org

openbsd-77-iso = dist/openbsd/7.7/amd64/cd77.iso
openbsd-77-img = dist/openbsd/7.7/amd64/miniroot77.img

openbsd-77-netboot-iso = ipxe/openbsd-7.7-amd64.iso.gz
openbsd-77-netboot-img = ipxe/openbsd-7.7-amd64.img.gz

openbsd-78-iso = dist/openbsd/7.8/amd64/cd78.iso
openbsd-78-img = dist/openbsd/7.8/amd64/miniroot78.img

openbsd-78-netboot-iso = ipxe/openbsd-7.8-amd64.iso.gz
openbsd-78-netboot-img = ipxe/openbsd-7.8-amd64.img.gz

openbsd-79-iso = dist/openbsd/7.9/amd64/cd79.iso
openbsd-79-img = dist/openbsd/7.9/amd64/miniroot79.img

openbsd-79-netboot-iso = ipxe/openbsd-7.9-amd64.iso.gz
openbsd-79-netboot-img = ipxe/openbsd-7.9-amd64.img.gz

alpine_mirror = dl-cdn.alpinelinux.org

alpine-3221-kernel = dist/alpine/3.22.1/x86_64/kernel
alpine-3221-initrd = dist/alpine/3.22.1/x86_64/initrd
alpine-3221-modloop = dist/alpine/3.22.1/x86_64/modloop

alpine-3234-kernel = dist/alpine/3.23.4/x86_64/kernel
alpine-3234-initrd = dist/alpine/3.23.4/x86_64/initrd
alpine-3234-modloop = dist/alpine/3.23.4/x86_64/modloop


customized-win11-iso = iso/customized_Win11_25H2_English_x64.iso
win11-iso = iso/Win11_25H2_English_x64.iso

dist_files = \
	$(debian-12-kernel) $(debian-12-initrd) \
	$(debian-13-kernel) $(debian-13-initrd) \
	$(openbsd-77-iso) $(openbsd-77-img) \
	$(openbsd-78-iso) $(openbsd-78-img) \
	$(openbsd-79-iso) $(openbsd-79-img) \
	$(alpine-3221-kernel) $(alpine-3221-initrd) $(alpine-3221-modloop) \
	$(alpine-3234-kernel) $(alpine-3234-initrd) $(alpine-3234-modloop) 

openbsd_netboot_iso_files = \
	$(openbsd-77-netboot-iso) \
	$(openbsd-78-netboot-iso) \
	$(openbsd-79-netboot-iso)

openbsd_netboot_img_files = \
	$(openbsd-77-netboot-img) \
	$(openbsd-78-netboot-img) \
	$(openbsd-79-netboot-img)

openbsd_generated_files = $(openbsd_netboot_iso_files) $(openbsd_netboot_img_files) 
linux_generated_files = $(customized-win11-iso)

generated_files = $(openbsd_generated_files) $(linux_generated_files)

gen: distgen $(generated_files)

regen: clean gen

distgen: $(dist_files)

$(win11-iso):
	@scripts/update_winiso $(dist_upload_host) $(notdir $@) iso download

dist_upload_host = $(shell [ -e ~/.dist_upload_host ] && cat ~/.dist_upload_host)

$(customized-win11-iso): $(win11-iso)
	$(call linux_exec,scripts/generate_windows_netboot $< $@)
	#$(call openbsd_exec,scripts/update_winiso $(dist_upload_host) $(notdir $@) iso download)
	#$(if $(dist_upload_host),scripts/update_winiso $(dist_upload_host) $(notdir $@) iso upload)

curl_download = mkdir -p $(dir $(2)) && curl -L $(1) --remote-time -o $(2) && test -s $(2)

$(debian-12-kernel):
	$(call curl_download,http://$(debian_mirror)/debian/dists/bookworm/main/installer-amd64/current/images/netboot/debian-installer/amd64/linux,$@)

$(debian-12-initrd):
	$(call curl_download,http://$(debian_mirror)/debian/dists/bookworm/main/installer-amd64/current/images/netboot/debian-installer/amd64/initrd.gz,$@)

$(debian-13-kernel):
	$(call curl_download,http://$(debian_mirror)/debian/dists/trixie/main/installer-amd64/current/images/netboot/debian-installer/amd64/linux,$@)

$(debian-13-initrd):
	$(call curl_download,http://$(debian_mirror)/debian/dists/trixie/main/installer-amd64/current/images/netboot/debian-installer/amd64/initrd.gz,$@)

$(openbsd-77-iso):
	$(call curl_download,https://$(openbsd_mirror)/pub/OpenBSD/7.7/amd64/cd77.iso,$@)

$(openbsd-77-img):
	$(call curl_download,https://$(openbsd_mirror)/pub/OpenBSD/7.7/amd64/miniroot77.img,$@)

$(openbsd-78-iso):
	$(call curl_download,https://$(openbsd_mirror)/pub/OpenBSD/7.8/amd64/cd78.iso,$@)

$(openbsd-78-img):
	$(call curl_download,https://$(openbsd_mirror)/pub/OpenBSD/7.8/amd64/miniroot78.img,$@)

$(openbsd-79-iso):
	$(call curl_download,https://$(openbsd_mirror)/pub/OpenBSD/7.9/amd64/cd79.iso,$@)

$(openbsd-79-img):
	$(call curl_download,https://$(openbsd_mirror)/pub/OpenBSD/7.9/amd64/miniroot79.img,$@)


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

sterile: clean
	rm -f $(generated_files)

distclean:
	rm -rf dist
	mkdir dist
	echo "# placeholder" >dist/README.md

# only clean files that can be regenerated
clean: distclean
	$(call openbsd_exec,rm -f $(openbsd_generated_files))
	$(call linux_exec,rm -f $(linux_generated_files))

