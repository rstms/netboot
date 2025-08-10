package server

import (
	"bytes"
	"fmt"
	"github.com/rstms/netboot/template"
	"io"
	"io/fs"
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
	URL               string
	ExternalCPIO	  bool
}

func NewMkBoot(config *Config, dir, tarball, response, diskLabelTemplate, url string) *MkBoot {
	m := MkBoot{config, dir, tarball, response, diskLabelTemplate, url, true}
	return &m
}

func (m *MkBoot) Generate() error {
	fmt.Printf("Generate: %s\n", FormatJSON(m))
	switch strings.ToLower(m.Config.OS) {
	case "openbsd":
		return m.mkbootOpenBSD()
	case "debian":
		return m.mkbootDebian()
	case "alpine":
		return m.mkbootAlpine()
	}
	return fmt.Errorf("unexpected OS: '%s'", m.Config.OS)
}

func (m *MkBoot) mkbootOpenBSD() error {
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
		return Fatalf("script %s failed: %v\n", script, err)
	}
	if cmd.ProcessState.ExitCode() != 0 {
		return Fatalf("uncaught process failure")
	}
	return nil
}

func (m *MkBoot) mkbootAlpine() error {
	panic("fixme")
}

func (m *MkBoot) mkbootDebian() error {
	fmt.Printf("mkbootDebian: %+v\n", *m.Config)

	srcFilename := filepath.Join(
		"debian",
		"dists",
		m.Config.Version,
		"main",
		"installer-"+m.Config.Arch,
		"current",
		"images",
		"netboot",
		"debian-installer",
		m.Config.Arch,
		"initrd.gz",
	)

	/*
	err := copyFileFS(m.Dir, m.Config.Address+".initrd.gz", template.Debian, srcFilename)
	if err != nil {
		return Fatal(err)
	}
	*/

		tempDir, err := os.MkdirTemp("", "initrd*")
		if err != nil {
			return Fatal(err)
		}

		err = copyFileFS(tempDir, "initrd.gz", template.Debian, srcFilename)
		if err != nil {
			return Fatal(err)
		}

		tempCertsDir := filepath.Join(tempDir, "etc", "ssl", "certs")
		err = os.MkdirAll(tempCertsDir, 0755)
		if err != nil {
			return Fatal(err)
		}

		err = copyFileFS(tempCertsDir, "kemaster.crt", template.Certs, "keymaster.pem")
		if err != nil {
			return Fatal(err)
		}

		initrdName, err := UnzipFile(filepath.Join(tempDir, "initrd.gz"))
		if err != nil {
			return Fatal(err)
		}

		err = copyFile(filepath.Join(tempDir, "preseed.cfg"), m.Response)
		if err != nil {
			return Fatal(err)
		}

		err = copyFile(filepath.Join(tempDir, "package.tgz"), m.Tarball)
		if err != nil {
			return Fatal(err)
		}

		err = copyFile(filepath.Join(tempDir, "package.tgz"), m.Tarball)
		if err != nil {
			return Fatal(err)
		}


		if m.ExternalCPIO {
		    addFiles := []string{
			filepath.Join("etc", "ssl", "certs", "keymaster.crt"),
			"preseed.cfg",
			"package.tgz",
		    }
		    err = run(strings.Join(addFiles, "\n"), tempDir, "cpio", "-H", "sv4cpio", "-o", "-A", "-F", "initrd")
		    if err != nil {
			return Fatal(err)
		    }
		} else {

		newInitrdName := initrdName + ".new"
		addFiles := []string{
			filepath.Join(tempDir, "preseed.cfg"),
			filepath.Join(tempDir, "package.tgz"),
			filepath.Join(tempDir, "etc", "ssl", "certs", "keymaster.crt"),
		}
		err = GenerateInitrd(newInitrdName, initrdName, addFiles)
		if err != nil {
			return Fatal(err)
		}
		err = os.Rename(newInitrdName, initrdName)
		if err != nil {
			return Fatal(err)
		}
		initrdName, err = ZipFile(initrdName)
		if err != nil {
			return Fatal(err)
		}
		}

		dstFile := filepath.Join(m.Dir, m.Config.Address+".initrd.gz")

		err = copyFile(dstFile, initrdName)
		if err != nil {
			return Fatal(err)
		}

	return nil
}

func copyFileFS(tempDir, dstName string, srcFS fs.FS, srcName string) error {
	srcFile, err := srcFS.Open(srcName)
	if err != nil {
		return Fatal(err)
	}
	defer srcFile.Close()
	dstFile, err := os.Create(filepath.Join(tempDir, dstName))
	if err != nil {
		return Fatal(err)
	}
	defer dstFile.Close()
	_, err = io.Copy(dstFile, srcFile)
	if err != nil {
		return Fatal(err)
	}
	return nil
}

func run(stdin, tempDir, command string, args ...string) error {

	cmd := exec.Command(command, args...)
	cmd.Dir = tempDir
	if stdin != "" {
		cmd.Stdin = bytes.NewBuffer([]byte(stdin))
	}
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	err := cmd.Run()
	if err != nil {
		return Fatal(err)
	}
	return nil
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
