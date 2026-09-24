package main

import "testing"

func TestNewerVersion(t *testing.T) {
	for _, tt := range []struct {
		current, candidate string
		want               bool
	}{
		{"0.1.0", "0.1.1", true},
		{"0.1.0", "0.2.0", true},
		{"0.1.0", "1.0.0", true},
		{"0.1.0", "0.1.0", false},
		{"0.2.0", "0.1.9", false},
		{"1.0.0", "0.9.9", false},
		{"0.9.0", "0.10.0", true},
		{"0.10.0", "0.9.0", false},
		{"0.1.0-dev", "0.1.0", true},
		{"0.1.0-dev", "0.0.9", false},
		{"0.1.0", "not-a-version", false},
	} {
		if got := newerVersion(tt.current, tt.candidate); got != tt.want {
			t.Errorf("newerVersion(%q, %q) = %v, want %v", tt.current, tt.candidate, got, tt.want)
		}
	}
}

func TestPlatformAssetPicksOneInstaller(t *testing.T) {
	rel := ghRelease{Assets: []ghAsset{
		{Name: "SHA256SUMS", URL: "https://example.invalid/SHA256SUMS"},
		{Name: "release-manifest.json", URL: "https://example.invalid/manifest"},
		{Name: "CARE-Desktop-1.2.3-macos.dmg", URL: "https://example.invalid/dmg"},
		{Name: "CARE-Desktop-1.2.3-windows-amd64-setup.exe", URL: "https://example.invalid/exe"},
	}}
	asset, ok := platformAsset(rel)
	if !ok {
		t.Skip("no installer is published for this platform")
	}
	if asset.URL == "" || asset.Name == "SHA256SUMS" {
		t.Fatalf("picked %q as the installer", asset.Name)
	}
	if _, ok := findAsset(rel, func(name string) bool { return name == "SHA256SUMS" }); !ok {
		t.Fatal("checksums were not found alongside the installer")
	}
	empty := ghRelease{Assets: []ghAsset{{Name: "CARE-Desktop-1.2.3-macos.dmg"}}}
	if _, ok := platformAsset(empty); ok {
		t.Fatal("an asset with no URL was accepted")
	}
}
