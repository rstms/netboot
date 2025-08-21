#
# download OS installer resources
#

debian_mirror = ftp.us.debian.org
debian-12-kernel = dist/debian/bookworm/amd64/linux
debian-12-initrd = dist/debian/bookworm/amd64/initrd.gz
debian-13-kernel = dist/debian/trixie/amd64/linux
debian-13-initrd = dist/debian/trixie/amd64/initrd.gz

openbsd_mirror = mirrors.mit.edu
openbsd-75-iso = dist/openbsd/7.5/amd64/cd75.iso
openbsd-76-iso = dist/openbsd/7.6/amd64/cd76.iso
openbsd-77-iso = dist/openbsd/7.7/amd64/cd77.iso

alpine_mirror = dl-cdn.alpinelinux.org
alpine-3221-kernel = dist/alpine/3.22.1/x86_64/kernel
alpine-3221-initrd = dist/alpine/3.22.1/x86_64/initrd
alpine-3221-modloop = dist/alpine/3.22.1/x86_64/modloop

all: debian12 debian13 openbsd75 openbsd76 openbsd77 alpine3221

debian12: $(debian-12-kernel) $(debian-12-initrd)
debian13: $(debian-13-kernel) $(debian-13-initrd)
openbsd75: $(openbsd-75-iso)
openbsd76: $(openbsd-76-iso)
openbsd77: $(openbsd-77-iso)
alpine3221: $(alpine-3221-kernel) $(alpine-3221-initrd) $(alpine-3221-modloop)


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

$(openbsd-76-iso):
	mkdir -p $(dir $@)
	curl -L https://$(openbsd_mirror)/pub/OpenBSD/7.6/amd64/cd76.iso -o $@

$(openbsd-77-iso):
	mkdir -p $(dir $@)
	curl -L https://$(openbsd_mirror)/pub/OpenBSD/7.7/amd64/cd77.iso -o $@

$(alpine-3221-kernel):
	mkdir -p $(dir $@)
	curl -L https://$(alpine_mirror)/v3.22/releases/x86_64/netboot-3.22.1/vmlinuz-virt -o $@

$(alpine-3221-initrd):
	mkdir -p $(dir $@)
	curl -L https://$(alpine_mirror)/v3.22/releases/x86_64/netboot-3.22.1/initramfs-virt -o $@

$(alpine-3221-modloop):
	mkdir -p $(dir $@)
	curl -L https://$(alpine_mirror)/v3.22/releases/x86_64/netboot-3.22.1/modloop-virt -o $@

clean:
	rm -rf dist
	mkdir dist

