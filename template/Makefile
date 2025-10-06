#
# download OS installer resources
#

bsdgen = $(if $(windows),@echo "skipping OpenBSD script: $(1)",$(1))

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

custom-win11-iso = dist/windows/11/x64/customized_Win11_25H2_English_x64.iso
win11-iso = dist/windows/11/x64/Win11_25H2_English_x64.iso

dist_files = \
	$(debian-12-kernel) $(debian-12-initrd) \
	$(debian-13-kernel) $(debian-13-initrd) \
	$(openbsd-75-iso) $(openbsd-75-img) \
	$(openbsd-76-iso) $(openbsd-76-img) \
	$(openbsd-77-iso) $(openbsd-77-img) \
	$(alpine-3221-kernel) $(alpine-3221-initrd) $(alpine-3221-modloop) \

openbsd_netboot_iso_files = $(openbsd-75-netboot-iso) $(openbsd-76-netboot-iso) $(openbsd-77-netboot-iso)
openbsd_netboot_img_files = $(openbsd-75-netboot-img) $(openbsd-76-netboot-img) $(openbsd-77-netboot-img)

generated_files = $(openbsd_netboot_iso_files) $(openbsd_netboot_img_files)

gen: $(dist_files) $(generated_files)

regen: clean gen

wingen: $(custom-win11-iso)

$(win11-iso):
	curl -L https://vega.rstms.net/pub/iso/$(notdir $@) -o $@

$(custom-win11-iso): $(win11-iso)
	doas scripts/customize_windows_install_iso $< $@

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
	$(call bsdgen,scripts/generate_openbsd_netboot 7.5 amd64 iso)

$(openbsd-75-netboot-img): $(openbsd-75-netboot-iso)
	$(call bsdgen,scripts/generate_openbsd_netboot 7.5 amd64 img)

$(openbsd-76-netboot-iso): $(openbsd-76-iso) $(openbsd-76-img) $(wildcard mkboot/*.openbsd) $(wildcard scripts/*)
	$(call bsdgen,scripts/generate_openbsd_netboot 7.6 amd64 iso)

$(openbsd-76-netboot-img): $(openbsd-76-netboot-iso)
	$(call bsdgen,scripts/generate_openbsd_netboot 7.6 amd64 img)

$(openbsd-77-netboot-iso): $(openbsd-77-iso) $(openbsd-77-img) $(wildcard mkboot/*.openbsd) $(wildcard scripts/*)
	$(call bsdgen,scripts/generate_openbsd_netboot 7.7 amd64 iso)

$(openbsd-77-netboot-img): $(openbsd-77-netboot-iso)
	$(call bsdgen,scripts/generate_openbsd_netboot 7.7 amd64 img)


sterile: clean
	find dist -type f -not -name 'gdl??.tgz' -exec rm \{\} \;
	echo "# placeholder" >dist/README.md

clean:
	rm -f $(generated_files)

