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
	ExternalCPIO      bool
}

func NewMkBoot(config *Config, dir, tarball, response, diskLabelTemplate, url string) *MkBoot {
	m := MkBoot{config, dir, tarball, response, diskLabelTemplate, url, true}
	return &m
}

// prepare OS-specific boot files in the cache directory named with MAC address
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

	distDir := filepath.Join(
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
	)

	err := copyFileFS(m.Dir, m.Config.Address+".kernel", template.Debian, filepath.Join(distDir, "linux"))
	if err != nil {
		return Fatal(err)
	}

	err = copyFileFS(m.Dir, m.Config.Address+".initrd", template.Debian, filepath.Join(distDir, "initrd.gz.keymaster"))
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
