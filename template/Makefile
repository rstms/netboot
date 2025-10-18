#
# download OS installer resources
#

# OS detection flags
windows = $(if $(SYSTEMROOT),1,)
openbsd = $(if $(shell uname | grep -i openbsd),1,)
linux = $(if $(shell uname | grep -i linux),1,)

# functions for running only on selected OS
openbsd_exec = $(if $(openbsd),$(1),@echo "not on openbsd; skipping: $(1)")
linux_exec = $(if $(linux),$(1),@echo "not on linux; skipping: $(1)")
windows_exec = $(if $(windows),$(1),@echo "not on windows; skipping: $(1)")

debian_mirror = ftp.us.debian.org

debian-12-kernel = dist/debian/bookworm/amd64/linux
debian-12-initrd = dist/debian/bookworm/amd64/initrd.gz

debian-13-kernel = dist/debian/trixie/amd64/linux
debian-13-initrd = dist/debian/trixie/amd64/initrd.gz

openbsd_mirror = mirrors.mit.edu

openbsd-75-iso = dist/openbsd/7.5/amd64/cd75.iso
openbsd-75-img = dist/openbsd/7.5/amd64/miniroot75.img

openbsd-75-netboot-iso = ipxe/openbsd-7.5-amd64.iso.gz
openbsd-75-netboot-img = ipxe/openbsd-7.5-amd64.img.gz

openbsd-76-iso = dist/openbsd/7.6/amd64/cd76.iso
openbsd-76-img = dist/openbsd/7.6/amd64/miniroot76.img

openbsd-76-netboot-iso = ipxe/openbsd-7.6-amd64.iso.gz
openbsd-76-netboot-img = ipxe/openbsd-7.6-amd64.img.gz

openbsd-77-iso = dist/openbsd/7.7/amd64/cd77.iso
openbsd-77-img = dist/openbsd/7.7/amd64/miniroot77.img

openbsd-77-netboot-iso = ipxe/openbsd-7.7-amd64.iso.gz
openbsd-77-netboot-img = ipxe/openbsd-7.7-amd64.img.gz

alpine_mirror = dl-cdn.alpinelinux.org

alpine-3221-kernel = dist/alpine/3.22.1/x86_64/kernel
alpine-3221-initrd = dist/alpine/3.22.1/x86_64/initrd
alpine-3221-modloop = dist/alpine/3.22.1/x86_64/modloop

customized-win11-iso = iso/customized_Win11_25H2_English_x64.iso
win11-iso = iso/Win11_25H2_English_x64.iso

dist_files = \
	$(debian-12-kernel) $(debian-12-initrd) \
	$(debian-13-kernel) $(debian-13-initrd) \
	$(openbsd-75-iso) $(openbsd-75-img) \
	$(openbsd-76-iso) $(openbsd-76-img) \
	$(openbsd-77-iso) $(openbsd-77-img) \
	$(alpine-3221-kernel) $(alpine-3221-initrd) $(alpine-3221-modloop) \

openbsd_netboot_iso_files = $(openbsd-75-netboot-iso) $(openbsd-76-netboot-iso) $(openbsd-77-netboot-iso)
openbsd_netboot_img_files = $(openbsd-75-netboot-img) $(openbsd-76-netboot-img) $(openbsd-77-netboot-img)

openbsd_generated_files = $(openbsd_netboot_iso_files) $(openbsd_netboot_img_files) 
linux_generated_files = $(customized-win11-iso)

generated_files = $(openbsd_generated_files) $(linux_generated_files)

gen: $(dist_files) $(generated_files)

regen: clean gen

$(win11-iso):
	mkdir -p $(dir $@)
	curl -L https://vega.rstms.net/pub/iso/$(notdir $@) -o $@

dist_host != $(shell [ -e ~/.dist_host ] && cat ~/.dist_host)

$(customized-win11-iso): $(win11-iso)
	$(call linux_exec,scripts/generate_windows_netboot $< $@)
	$(if $(dist_host),scp -p $< $(dist_host):iso/$(notdir $@),:)

$(debian-12-kernel):
	mkdir -p $(dir $@)
	curl -L http://$(debian_mirror)/debian/dists/bookworm/main/installer-amd64/current/images/netboot/debian-installer/amd64/linux -o $@

$(debian-12-initrd):
	mkdir -p $(dir $@)
	curl -L http://$(debian_mirror)/debian/dists/bookworm/main/installer-amd64/current/images/netboot/debian-installer/amd64/initrd.gz -o $@

$(debian-13-kernel):
	mkdir -p $(dir $@)
	curl -L http://$(debian_mirror)/debian/dists/trixie/main/installer-amd64/current/images/netboot/debian-installer/amd64/linux -o $@

$(debian-13-initrd):
	mkdir -p $(dir $@)
	curl -L http://$(debian_mirror)/debian/dists/trixie/main/installer-amd64/current/images/netboot/debian-installer/amd64/initrd.gz -o $@

$(openbsd-75-iso):
	mkdir -p $(dir $@)
	curl -L https://$(openbsd_mirror)/pub/OpenBSD/7.5/amd64/cd75.iso -o $@

$(openbsd-75-img):
	mkdir -p $(dir $@)
	curl -L https://$(openbsd_mirror)/pub/OpenBSD/7.5/amd64/miniroot75.img -o $@

$(openbsd-76-iso):
	mkdir -p $(dir $@)
	curl -L https://$(openbsd_mirror)/pub/OpenBSD/7.6/amd64/cd76.iso -o $@

$(openbsd-76-img):
	mkdir -p $(dir $@)
	curl -L https://$(openbsd_mirror)/pub/OpenBSD/7.6/amd64/miniroot76.img -o $@

$(openbsd-77-iso):
	mkdir -p $(dir $@)
	curl -L https://$(openbsd_mirror)/pub/OpenBSD/7.7/amd64/cd77.iso -o $@

$(openbsd-77-img):
	mkdir -p $(dir $@)
	curl -L https://$(openbsd_mirror)/pub/OpenBSD/7.7/amd64/miniroot77.img -o $@

$(alpine-3221-kernel):
	mkdir -p $(dir $@)
	curl -L https://$(alpine_mirror)/v3.22/releases/x86_64/netboot-3.22.1/vmlinuz-virt -o $@

$(alpine-3221-initrd):
	mkdir -p $(dir $@)
	curl -L https://$(alpine_mirror)/v3.22/releases/x86_64/netboot-3.22.1/initramfs-virt -o $@

$(alpine-3221-modloop):
	mkdir -p $(dir $@)
	curl -L https://$(alpine_mirror)/v3.22/releases/x86_64/netboot-3.22.1/modloop-virt -o $@

certs/cacerts.tgz: $(ca_certs)
	cd certs/debian && tar zcfv ../cacerts.tgz etc

$(openbsd-75-netboot-iso): $(openbsd-75-iso) $(openbsd-75-img) $(wildcard mkboot/*.openbsd) $(wildcard scripts/*)
	$(call openbsd_exec,scripts/generate_openbsd_netboot 7.5 amd64 iso)

$(openbsd-75-netboot-img): $(openbsd-75-netboot-iso)
	$(call openbsd_exec,scripts/generate_openbsd_netboot 7.5 amd64 img)

$(openbsd-76-netboot-iso): $(openbsd-76-iso) $(openbsd-76-img) $(wildcard mkboot/*.openbsd) $(wildcard scripts/*)
	$(call openbsd_exec,scripts/generate_openbsd_netboot 7.6 amd64 iso)

$(openbsd-76-netboot-img): $(openbsd-76-netboot-iso)
	$(call openbsd_exec,scripts/generate_openbsd_netboot 7.6 amd64 img)

$(openbsd-77-netboot-iso): $(openbsd-77-iso) $(openbsd-77-img) $(wildcard mkboot/*.openbsd) $(wildcard scripts/*)
	$(call openbsd_exec,scripts/generate_openbsd_netboot 7.7 amd64 iso)

$(openbsd-77-netboot-img): $(openbsd-77-netboot-iso)
	$(call openbsd_exec,scripts/generate_openbsd_netboot 7.7 amd64 img)

sterile: clean
	rm -f $(generated_files)
	find dist -type f -not -name 'gdl??.tgz' -exec rm \{\} \;
	echo "# placeholder" >dist/README.md

# only clean files that can be regenerated
clean:
	$(call openbsd_exec,rm -f $(openbsd_generated_files))
	$(call linux_exec,rm -f $(linux_generated_files))

