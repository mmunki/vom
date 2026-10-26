package catalog

import (
	"strings"
	"testing"
)

const sampleXML = `<?xml version="1.0"?>
<sdk-repository>
  <remotePackage path="system-images;android-24;default;x86_64">
    <type-details>
      <api-level>24</api-level>
      <tag><id>default</id><display>Default</display></tag>
      <abi>x86_64</abi>
    </type-details>
    <revision><major>8</major></revision>
    <display-name>Android 7.0 x86_64</display-name>
    <archives><archive><complete>
      <size>419261998</size>
      <checksum type="sha1">F6559E1949A5879F31A9662F4F0E50AD60181684</checksum>
      <url>x86_64-24_r08.zip</url>
    </complete></archive></archives>
  </remotePackage>
  <remotePackage path="system-images;android-24;default;x86_64">
    <type-details>
      <api-level>24</api-level>
      <tag><id>default</id></tag>
      <abi>x86_64</abi>
    </type-details>
    <revision><major>7</major></revision>
    <archives><archive><complete>
      <size>407148033</size>
      <checksum type="sha1">aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa</checksum>
      <url>x86_64-24_r07.zip</url>
    </complete></archive></archives>
  </remotePackage>
  <remotePackage path="system-images;android-37.2-beta1;google_apis;x86_64">
    <type-details><api-level>37.2-beta1</api-level><tag><id>google_apis</id></tag><abi>x86_64</abi></type-details>
    <revision><major>1</major></revision>
    <archives><archive><complete><size>1</size><url>beta.zip</url></complete></archive></archives>
  </remotePackage>
  <remotePackage path="system-images;android-37.0;google_apis_ps16k;x86_64">
    <type-details><api-level>37.0</api-level><tag><id>google_apis_ps16k</id></tag><abi>x86_64</abi></type-details>
    <revision><major>1</major></revision>
    <archives><archive><complete><size>1</size><url>ps16k.zip</url></complete></archive></archives>
  </remotePackage>
  <remotePackage path="system-images;android-23;default;x86_64">
    <type-details><api-level>23</api-level><tag><id>default</id></tag><abi>x86_64</abi></type-details>
    <revision><major>1</major></revision>
    <archives><archive><complete><size>1</size><url>old.zip</url></complete></archive></archives>
  </remotePackage>
</sdk-repository>`

func TestParseSourceFiltersAndKeepsLatest(t *testing.T) {
	images, err := parseSource("android", "https://example.test/", strings.NewReader(sampleXML))
	if err != nil {
		t.Fatal(err)
	}
	if len(images) != 2 {
		t.Fatalf("got %d images, want 2 (duplicate collapsed in dedupe only): %+v", len(images), images)
	}
	got := dedupe(images)
	if len(got) != 1 {
		t.Fatalf("dedupe kept %d images, want 1", len(got))
	}
	img := got[0]
	if img.API != 24 || img.ABI != "x86_64" || img.Tag != "default" {
		t.Fatalf("unexpected image: %+v", img)
	}
	if img.Revision != "8.0.0" {
		t.Fatalf("revision = %s, want 8.0.0", img.Revision)
	}
	if img.SHA1 != "f6559e1949a5879f31a9662f4f0e50ad60181684" {
		t.Fatalf("sha1 was not normalised: %s", img.SHA1)
	}
	if img.URL != "https://example.test/x86_64-24_r08.zip" {
		t.Fatalf("url = %s", img.URL)
	}
}

func TestVersionNames(t *testing.T) {
	if v, c := VersionName(24); v != "7.0" || c != "Nougat" {
		t.Fatalf("24 = %s %s", v, c)
	}
	if v, _ := VersionName(37); v != "17" {
		t.Fatalf("37 = %s", v)
	}
}

func TestEmbeddedCatalogCoversEveryVersion(t *testing.T) {
	images, _, err := Fallback()
	if err != nil {
		t.Fatal(err)
	}
	seen := map[int]bool{}
	ids := map[string]bool{}
	for _, img := range images {
		seen[img.API] = true
		if ids[img.ID] {
			t.Fatalf("duplicate image id %s", img.ID)
		}
		ids[img.ID] = true
		if img.Size <= 0 || img.SHA1 == "" || !strings.HasPrefix(img.URL, "https://dl.google.com/") {
			t.Fatalf("incomplete image metadata: %+v", img)
		}
	}
	for api := MinAPI; api <= MaxAPI; api++ {
		if !seen[api] {
			t.Errorf("embedded catalog is missing API %d", api)
		}
	}
}

func TestPickPrefersHostABI(t *testing.T) {
	images, _, err := Fallback()
	if err != nil {
		t.Fatal(err)
	}
	img, ok := Pick(images, 24, "", "x86_64")
	if !ok || img.ABI != "x86_64" {
		t.Fatalf("pick 24 x86_64 = %+v, %v", img, ok)
	}
	img, ok = Pick(images, 24, "", "arm64-v8a")
	if !ok || img.ABI != "arm64-v8a" {
		t.Fatalf("pick 24 arm64 = %+v, %v", img, ok)
	}
	if _, ok := Pick(images, 99, "", "x86_64"); ok {
		t.Fatal("picked an image outside the supported range")
	}
}
