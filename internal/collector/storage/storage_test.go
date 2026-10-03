package storage

import (
	"os"
	"path/filepath"
	"testing"
)

const mounts = "rootfs / rootfs ro 0 0\n" +
	"proc /proc proc rw 0 0\n" +
	"tmpfs /run tmpfs rw 0 0\n" +
	"ext4 / ext4 rw,relatime 0 0\n" +
	"ext4 /mnt/music ext4 rw 0 0\n" +
	"ext4 / ext4 rw 0 0\n"

func TestParseMounts(t *testing.T) {
	mounts, types := parseMounts(mounts)
	if len(mounts) != 6 || len(types) != 6 {
		t.Fatalf("got %d mounts / %d types", len(mounts), len(types))
	}
	if mounts[3] != "/" || types[3] != "ext4" {
		t.Errorf("entry 3 = %q %q", mounts[3], types[3])
	}
}

func TestUnescape(t *testing.T) {
	if got := unescape("/mnt/a\\040b"); got != "/mnt/a b" {
		t.Errorf("unescape = %q", got)
	}
	if got := unescape("/plain"); got != "/plain" {
		t.Errorf("unescape = %q", got)
	}
}

func TestCollect(t *testing.T) {
	root := t.TempDir()
	for _, d := range []string{"proc", filepath.Join("mnt", "music")} {
		if err := os.MkdirAll(filepath.Join(root, d), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(root, "proc", "mounts"), []byte(mounts), 0o644); err != nil {
		t.Fatal(err)
	}

	var called []string
	fakeStatfs := func(path string) (int64, int64, int64, error) {
		called = append(called, path)
		return 1000, 500, 500, nil
	}
	c := New(root, []string{"proc", "tmpfs", "rootfs"}, fakeStatfs)
	fs := c.Collect()

	if len(fs) != 2 {
		t.Fatalf("expected 2 filesystems, got %d: %+v", len(fs), fs)
	}
	if fs[0].Mount != "/" || fs[0].FSType != "ext4" || !fs[0].Mounted {
		t.Errorf("first fs = %+v", fs[0])
	}
	if fs[1].Mount != "/mnt/music" {
		t.Errorf("second fs = %+v", fs[1])
	}
	if fs[0].Total != 1000 || fs[0].Used != 500 || fs[0].Available != 500 {
		t.Errorf("sizes = %+v", fs[0])
	}
	if fs[0].Percent != 50 {
		t.Errorf("percent = %f, want 50", fs[0].Percent)
	}
	for _, p := range called {
		if p == filepath.Join(root, "proc") || p == filepath.Join(root, "run") {
			t.Errorf("statfs called for pseudo mount %q", p)
		}
	}
	if len(called) != 2 {
		t.Errorf("statfs called %d times, want 2 (duplicate mount point deduplicated)", len(called))
	}
}

func TestCollectMissingMountsFile(t *testing.T) {
	c := New(t.TempDir(), nil, func(string) (int64, int64, int64, error) {
		return 0, 0, 0, nil
	})
	if fs := c.Collect(); len(fs) != 0 {
		t.Errorf("expected no filesystems, got %d", len(fs))
	}
}

func TestCollectStatfsErrorSkipsMount(t *testing.T) {
	root := t.TempDir()
	procDir := filepath.Join(root, "proc")
	if err := os.MkdirAll(procDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(procDir, "mounts"), []byte("ext4 / ext4 rw 0 0\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	c := New(root, nil, func(string) (int64, int64, int64, error) {
		return 0, 0, 0, os.ErrNotExist
	})
	if fs := c.Collect(); len(fs) != 0 {
		t.Errorf("expected no filesystems, got %d", len(fs))
	}
}

func TestPercentZeroTotal(t *testing.T) {
	if p := percent(10, 0); p != 0 {
		t.Errorf("percent = %f", p)
	}
}
