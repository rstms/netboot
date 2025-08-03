package server

import (
	"bytes"
	"fmt"
	"github.com/rstms/netboot/template"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

type MkBoot struct {
	Config            *Config
	Dir               string
	Tarball           string
	Response          string
	DiskLabelTemplate string
}

func NewMkBoot(config *Config, dir, tarball, response, diskLabelTemplate string) *MkBoot {
	m := MkBoot{config, dir, tarball, response, diskLabelTemplate}
	return &m
}

func (m *MkBoot) Generate() ([]string, error) {
	fmt.Printf("Generate: %s\n", FormatJSON(m))
	switch strings.ToLower(m.Config.OS) {
	case "openbsd":
		return m.mkbootOpenBSD()
	case "debian":
		return m.mkbootDebian()
	case "alpine":
		return m.mkbootAlpine()
	}
	return []string{}, fmt.Errorf("unexpected OS: '%s'", m.Config.OS)
}

func (m *MkBoot) mkbootOpenBSD() ([]string, error) {
	script := fmt.Sprintf("/root/mkboot.%s", m.Config.OS)
	args := []string{script, m.Config.Address}
	if m.Config.Serial != "" {
		args = append(args, m.Config.Serial)
	}
	cmd := exec.Command("/usr/bin/doas", args...)
	var stdout bytes.Buffer
	cmd.Stdout = &stdout
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	err := cmd.Run()
	if err != nil {
		log.Printf("script %s failed: %v\n", script, err)
		elines := strings.Split(stderr.String(), "\n")
		for _, eline := range elines {
			log.Printf("stderr: %s\n", eline)
		}
		return elines, fmt.Errorf("script %s failed: %v\n", script, err)
	}
	if cmd.ProcessState.ExitCode() != 0 {
		log.Fatalf("uncaught process failure")
	}
	outputLines := strings.Split(stdout.String(), "\n")
	return outputLines, nil
}

func (m *MkBoot) mkbootAlpine() ([]string, error) {
	panic("todo")
}

type CpioFile struct {
	Name string
	Body string
}

func (m *MkBoot) mkbootDebian() ([]string, error) {
	fmt.Printf("mkbootDebian: %+v\n", *m.Config)
	lines := []string{}
	distFilename := filepath.Join(
		"debian",
		"dists",
		m.Config.Version,
		"main",
		"installer-amd64",
		"current",
		"images",
		"netboot",
		"debian-installer",
		"amd64",
		"initrd.gz",
	)
	infile, err := template.Debian.Open(distFilename)
	if err != nil {
		return lines, err
	}
	lines = append(lines, "initrd source: "+distFilename)
	initrd, err := NewInitrd(infile)
	if err != nil {
		return lines, err
	}
	responseData, err := os.ReadFile(m.Response)
	if err != nil {
		return lines, err
	}
	err = initrd.AddFile("preseed.cfg", responseData, 0600, 0, 0)
	if err != nil {
		return lines, err
	}
	lines = append(lines, "added preseed.cfg")
	tarballData, err := os.ReadFile(m.Tarball)
	if err != nil {
		return lines, err
	}
	err = initrd.AddFile("package.tgz", tarballData, 0600, 0, 0)
	if err != nil {
		return lines, err
	}
	lines = append(lines, "added package.tgz")
	outFilename := filepath.Join(m.Dir, m.Config.Address+".initrd")
	outFile, err := os.Create(outFilename)
	if err != nil {
		return lines, err
	}
	defer outFile.Close()
	err = initrd.Write(outFile)
	if err != nil {
		return lines, err
	}
	lines = append(lines, "initrd: "+outFilename)
	return lines, nil
}

/*
#!/bin/sh
#
set -ue

mac=$1
arch=amd64
codename=bookworm
webroot=/var/www/htdocs/debian
netboot=/var/www/netboot
tempdir=$(mktemp -d)
target=${netboot}/${mac}.initrd

fail() {
  echo >&2 $0 "$@"
  exit 1
}

cleanup() {
  if [ -n "$tempdir" ]; then
    if [ -e "$tempdir" ]; then
       rm -rf $tempdir
    fi
  fi
}
trap cleanup EXIT

[ -n "$mac" ] || fail no MAC

cd $tempdir

get_file() {
  cp $1 $2
  chown 0:0 $2
  chmod 0600 $2
}

get_file ${netboot}/${mac}.conf preseed.cfg
get_file ${netboot}/${mac}.tgz package.tgz
get_file ${webroot}/dists/${codename}/main/installer-${arch}/current/images/netboot/debian-installer/${arch}/initrd.gz initrd.gz
gunzip initrd.gz
echo "preseed.cfg" | cpio -H sv4cpio -o -A -F initrd
echo "package.tgz" | cpio -H sv4cpio -o -A -F initrd
gzip initrd
mv initrd.gz ${target}

/root/nbdperm
ls -l ${netboot}/${mac}*
*/
