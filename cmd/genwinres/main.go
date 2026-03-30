package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"github.com/tc-hib/winres"
)

func main() {
	var manifestPath string
	var outputPath string
	var archName string

	flag.StringVar(&manifestPath, "manifest", "", "path to the Windows application manifest")
	flag.StringVar(&outputPath, "out", "", "path to the generated .syso output")
	flag.StringVar(&archName, "arch", "", "target architecture (defaults to GOARCH)")
	flag.Parse()

	if manifestPath == "" {
		exitf("missing -manifest")
	}
	if outputPath == "" {
		exitf("missing -out")
	}
	if archName == "" {
		archName = os.Getenv("GOARCH")
	}
	if archName == "" {
		exitf("missing -arch and GOARCH is not set")
	}

	if err := generateResource(manifestPath, outputPath, archName); err != nil {
		exitf("%v", err)
	}
}

func generateResource(manifestPath, outputPath, archName string) error {
	arch, err := parseArch(archName)
	if err != nil {
		return err
	}

	manifestData, err := os.ReadFile(manifestPath)
	if err != nil {
		return fmt.Errorf("read manifest: %w", err)
	}

	if err := os.MkdirAll(filepath.Dir(outputPath), 0o755); err != nil {
		return fmt.Errorf("create output dir: %w", err)
	}

	out, err := os.Create(outputPath)
	if err != nil {
		return fmt.Errorf("create output: %w", err)
	}
	defer out.Close()

	rs := winres.ResourceSet{}
	if err := rs.Set(winres.RT_MANIFEST, winres.ID(1), winres.LCIDDefault, manifestData); err != nil {
		return fmt.Errorf("set manifest: %w", err)
	}
	if err := rs.WriteObject(out, arch); err != nil {
		return fmt.Errorf("write object: %w", err)
	}

	return nil
}

func parseArch(name string) (winres.Arch, error) {
	switch name {
	case string(winres.ArchI386):
		return winres.ArchI386, nil
	case string(winres.ArchAMD64):
		return winres.ArchAMD64, nil
	case string(winres.ArchARM):
		return winres.ArchARM, nil
	case string(winres.ArchARM64):
		return winres.ArchARM64, nil
	default:
		return "", fmt.Errorf("unsupported arch %q", name)
	}
}

func exitf(format string, args ...any) {
	fmt.Fprintf(os.Stderr, format+"\n", args...)
	os.Exit(1)
}
