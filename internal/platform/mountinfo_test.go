package platform

import (
	"strings"
	"testing"
)

// A trimmed but realistic mountinfo fixture: root on one device, /tmp as tmpfs,
// and an external drive mounted at /mnt/ext on its own device.
const mountinfoFixture = `22 28 0:21 / /proc rw,nosuid,nodev,noexec,relatime shared:14 - proc proc rw
25 28 8:2 / / rw,relatime shared:1 - ext4 /dev/sda2 rw
30 28 0:26 / /tmp rw,nosuid,nodev shared:15 - tmpfs tmpfs rw
40 28 0:44 / /run/user/1000 rw,nosuid,nodev,relatime shared:24 - tmpfs tmpfs rw,size=800000k
55 28 8:16 / /mnt/ext rw,relatime shared:30 - vfat /dev/sdb1 rw
`

func TestParseMountinfo(t *testing.T) {
	mounts, err := parseMountinfo(strings.NewReader(mountinfoFixture))
	if err != nil {
		t.Fatal(err)
	}
	if len(mounts) != 5 {
		t.Fatalf("expected 5 mounts, got %d", len(mounts))
	}
	ext := mounts[4]
	if ext.MountPoint != "/mnt/ext" || ext.FSType != "vfat" || ext.Device != "8:16" {
		t.Errorf("unexpected ext mount parse: %+v", ext)
	}
}

func TestMountpointFor(t *testing.T) {
	mounts, _ := parseMountinfo(strings.NewReader(mountinfoFixture))

	cases := []struct {
		path      string
		wantMount string
		wantFS    string
	}{
		{"/mnt/ext/repo/workspace", "/mnt/ext", "vfat"},
		{"/mnt/ext", "/mnt/ext", "vfat"},
		{"/home/pi/.config/neonroot", "/", "ext4"}, // on the card's root fs
		{"/tmp/neonroot-1000/workspaces", "/tmp", "tmpfs"},
	}
	for _, c := range cases {
		m, ok := MountpointFor(mounts, c.path)
		if !ok {
			t.Fatalf("no mount found for %s", c.path)
		}
		if m.MountPoint != c.wantMount || m.FSType != c.wantFS {
			t.Errorf("%s → got %s (%s), want %s (%s)",
				c.path, m.MountPoint, m.FSType, c.wantMount, c.wantFS)
		}
	}
}

// Guards the boundary bug where /mnt/ext must not match a path under /mnt/extra.
func TestMountpointFor_PathBoundary(t *testing.T) {
	const fixture = `25 28 8:2 / / rw shared:1 - ext4 /dev/sda2 rw
55 28 8:16 / /mnt/ext rw shared:30 - vfat /dev/sdb1 rw
`
	mounts, _ := parseMountinfo(strings.NewReader(fixture))
	m, ok := MountpointFor(mounts, "/mnt/extra/thing")
	if !ok || m.MountPoint != "/" {
		t.Errorf("/mnt/extra/thing should resolve to /, got %q (ok=%v)", m.MountPoint, ok)
	}
}
