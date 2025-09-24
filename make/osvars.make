# set os variables

os = $(if $(SYSTEMROOT),windows,$(shell uname | tr A-Z a-z))

ifeq ($(os),windows)
  hostname = $(shell cmd /c hostname)
  fqdn = $(hostname).$(lastword $(shell ipconfig /all | findstr /R /C:"Connection-specific DNS Suffix.*: [^ ]"))
  os_version=$(firstword $(subst ., ,$(lastword $(subst ],,$(shell cmd /c ver)))))
  arch = $(subst -,,$(lastword $(subst =, ,$(shell wmic os get osarchitecture /VALUE))))
  windows = 1
endif

ifeq ($(os),linux)
  hostname = $(shell hostname -s)
  fqdn = $(shell hostname --fqdn)
  os_version = $(firstword $(subst +, ,$(subst -, ,$(shell uname -r))))
  arch = $(shell uname -m)
  linux = 1
endif

ifeq ($(os),openbsd)
  hostname=$(shell hostname -s)
  fqdn=$(shell hostname)
  os_version=$(shell uname -r)
  arch = $(shell uname -m)
  openbsd = 1
endif
