#
# download OS installer resources
#

debian_mirror = ftp.us.debian.org
openbsd_mirror = mirrors.mit.edu

debian-12-kernel = dist/debian/bookworm/amd64/linux
debian-12-initrd = dist/debian/bookworm/amd64/initrd.gz

debian-13-kernel = dist/debian/trixie/amd64/linux
debian-13-initrd = dist/debian/trixie/amd64/initrd.gz

openbsd-75-iso = dist/openbsd/7.5/amd64/cd75.iso
openbsd-76-iso = dist/openbsd/7.6/amd64/cd76.iso
openbsd-77-iso = dist/openbsd/7.7/amd64/cd77.iso

all: debian12 debian13 openbsd75 openbsd76 openbsd77

debian12: $(debian-12-kernel) $(debian-12-initrd)
debian13: $(debian-13-kernel) $(debian-13-initrd)
openbsd75: $(openbsd-75-iso)
openbsd76: $(openbsd-76-iso)
openbsd77: $(openbsd-77-iso)


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

clean:
	rm -rf dist
	mkdir dist

