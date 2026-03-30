package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestGenerateResourceWritesSyso(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	manifestPath := filepath.Join(dir, "app.manifest")
	outputPath := filepath.Join(dir, "rsrc_windows_amd64.syso")

	manifest := `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<assembly xmlns="urn:schemas-microsoft-com:asm.v1" manifestVersion="1.0">
  <dependency>
    <dependentAssembly>
      <assemblyIdentity type="win32" name="Microsoft.Windows.Common-Controls" version="6.0.0.0" processorArchitecture="*" publicKeyToken="6595b64144ccf1df" language="*"/>
    </dependentAssembly>
  </dependency>
</assembly>`
	if err := os.WriteFile(manifestPath, []byte(manifest), 0o644); err != nil {
		t.Fatalf("WriteFile(manifest): %v", err)
	}

	if err := generateResource(manifestPath, outputPath, "amd64"); err != nil {
		t.Fatalf("generateResource: %v", err)
	}

	info, err := os.Stat(outputPath)
	if err != nil {
		t.Fatalf("Stat(output): %v", err)
	}
	if info.Size() == 0 {
		t.Fatal("generated .syso is empty")
	}
}

func TestParseArchRejectsUnsupportedArch(t *testing.T) {
	t.Parallel()

	if _, err := parseArch("mips64"); err == nil {
		t.Fatal("parseArch accepted an unsupported arch")
	}
}
