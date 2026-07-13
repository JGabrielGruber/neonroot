package platform

import (
	"bufio"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// Mount is a single entry from /proc/self/mountinfo (the fields NeonRoot needs).
type Mount struct {
	// MountPoint is the path where the filesystem is mounted.
	MountPoint string
	// Device is the "major:minor" device id (mountinfo field 3), the stable
	// way to tell two filesystems apart even when mount paths coincide.
	Device string
	// FSType is the filesystem type (e.g. ext4, vfat, tmpfs).
	FSType string
}

// parseMountinfo parses the /proc/self/mountinfo format. Extracted from the
// file read so it can be unit-tested against fixture content.
//
// Line format (space-separated):
//
//	36 35 98:0 /mnt1 /mnt2 rw,noatime master:1 - ext3 /dev/root rw,errors=continue
//	                 (5)                        (sep)(fstype)
//
// The variable-length optional fields end at the "-" separator; the filesystem
// type is the first field after it.
func parseMountinfo(r io.Reader) ([]Mount, error) {
	var mounts []Mount
	sc := bufio.NewScanner(r)
	sc.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for sc.Scan() {
		fields := strings.Fields(sc.Text())
		if len(fields) < 7 {
			continue
		}
		// Fields 0-4: mountID, parentID, major:minor, root, mountPoint.
		device := fields[2]
		mountPoint := unescapeOctal(fields[4])

		// Find the "-" separator that ends the optional fields.
		sep := -1
		for i := 5; i < len(fields); i++ {
			if fields[i] == "-" {
				sep = i
				break
			}
		}
		if sep < 0 || sep+1 >= len(fields) {
			continue
		}
		mounts = append(mounts, Mount{
			MountPoint: mountPoint,
			Device:     device,
			FSType:     fields[sep+1],
		})
	}
	return mounts, sc.Err()
}

// Mounts reads and parses the current mount table.
func Mounts() ([]Mount, error) {
	f, err := os.Open("/proc/self/mountinfo")
	if err != nil {
		return nil, err
	}
	defer f.Close()
	return parseMountinfo(f)
}

// MountpointFor returns the deepest mount whose MountPoint contains path, i.e.
// the filesystem that actually backs path. Ok is false if none matched (should
// not happen for an absolute path on a running system, where "/" always does).
//
// This is the primitive behind vault-availability: a stale mountpoint directory
// left behind after unmount resolves to a *different* (parent) mount than when
// the drive is present, which the vault layer detects via a stable marker file.
func MountpointFor(mounts []Mount, path string) (Mount, bool) {
	path = filepath.Clean(path)
	var best Mount
	found := false
	for _, m := range mounts {
		if pathHasPrefix(path, m.MountPoint) {
			if !found || len(m.MountPoint) > len(best.MountPoint) {
				best = m
				found = true
			}
		}
	}
	return best, found
}

// pathHasPrefix reports whether path is prefix itself or lies beneath it,
// comparing on path boundaries so /mnt/a is not treated as under /mnt/ab.
func pathHasPrefix(path, prefix string) bool {
	if prefix == "/" {
		return true
	}
	if path == prefix {
		return true
	}
	return strings.HasPrefix(path, prefix+"/")
}

// unescapeOctal decodes the \NNN octal escapes the kernel uses for spaces,
// tabs, and newlines in mountinfo paths.
func unescapeOctal(s string) string {
	if !strings.Contains(s, "\\") {
		return s
	}
	var b strings.Builder
	for i := 0; i < len(s); i++ {
		if s[i] == '\\' && i+3 < len(s) {
			var v int
			ok := true
			for j := 1; j <= 3; j++ {
				c := s[i+j]
				if c < '0' || c > '7' {
					ok = false
					break
				}
				v = v*8 + int(c-'0')
			}
			if ok {
				b.WriteByte(byte(v))
				i += 3
				continue
			}
		}
		b.WriteByte(s[i])
	}
	return b.String()
}
