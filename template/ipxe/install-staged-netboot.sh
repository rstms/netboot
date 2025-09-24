#!/bin/sh

cat ->map <<EOF
label: dos
label-id: 0x7070fc21
device: /dev/sda
unit: sectors
sector-size: 512

/dev/sda1 : start=        2048, size=       20480, type=6, bootable
/dev/sda2 : start=       22528, size=      262144, type=6
EOF

sfdisk /dev/sda <map

losetup /dev/loop1 netboot.iso
mkdir cd
mount -o ro /dev/loop1 cd

mkfs.fat -F 16 /dev/sda2
mkdir disk
mount /dev/sda2 disk

for file in gdl.tgz keymaster.pem netboot.env netboot_exec netboot.key netboot.pem; do
  cp cd/$file disk
done

umount disk
umount cd
losetup -d /dev/loop1

dd if=netboot.img of=/dev/sda1

base64 -d >mbr <<EOF
61iQU1lTTElOVVgAAgEBAALwAMAP8AwAPwAQAAAAAAAAAAAAAAApf7bbWmlQWEUgICAgICAgRkFU
MTIgICD6McCO2I7A/LkAAb4AfL8AgPOl6lYAAAi4AQK7+vwxyY7RvHZ7UgZXHlaOwbEmv3h786WO
2bt4AA+0Nw+gViDSeBsxwLEGiT+JRwLzZKWKDhh8iE34UFBQUM0T62KLVaqLdajB7gQB8oP6T3Yx
gfqyB3Mr9kW0f3UlOE24dCBmPSFHUFR1EIB9uO11Cmb/dexm/3Xo6w9RUWb/dbzrB1FRZv82HHy0
COjpAHITIOR1D8HqCEKJFhp8g+E/iQ4YfPu7qlW0QejLAHIQgftVqnUK9sEBdAXGBkZ9AGa4tAsA
AGa6AAAAALsAgOgOAGaBPhyAsBumWHV06fgCZgMGYHtmExZke7kQAOsrZlJmUAZTagFqEInmZmC0
Quh3AGZhjWQQcgHDZmAxwOhoAGZh4trGBkZ9K2ZgZg+3Nhh8Zg+3Php8Zvf2McmHymb392Y9/wMA
AHcXwOQGQQjhiMWI1rgBAugvAGZhcgHD4skx9o7WvGh7jt5mjwZ4AL7afawgwHQJtA67BwDNEOvy
McDNFs0Z9Ov9ihZ0ewbNEwfDQm9vdCBlcnJvcg0KAAAAAAAAAAAAAAAAAAAAAAAA/gKyPhg3Vao=
EOF
dd if=mbr of=/dev/sda bs=512 count=1

