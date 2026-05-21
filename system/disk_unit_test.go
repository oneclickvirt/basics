package system

import "testing"

func TestCreateDiskInfoZeroTotal(t *testing.T) {
	info := createDiskInfo(0, 0, "/dev/test", "/")
	if info.PercentageStr != "0.0%" {
		t.Fatalf("expected 0.0%%, got %s", info.PercentageStr)
	}
}

func TestParseSize(t *testing.T) {
	cases := []struct {
		in   string
		want uint64
	}{
		{"", 0},
		{"bad", 0},
		{"1K", 1024},
		{"2MB", 2 * 1024 * 1024},
		{"3G", 3 * 1024 * 1024 * 1024},
	}
	for _, c := range cases {
		got := parseSize(c.in)
		if got != c.want {
			t.Fatalf("parseSize(%q)=%d, want %d", c.in, got, c.want)
		}
	}
}

func TestGetPhysicalDiskName(t *testing.T) {
	cases := []struct {
		in   string
		want string
	}{
		{"/dev/sda1", "/dev/sda"},
		{"/dev/nvme0n1p2", "/dev/nvme0n1"},
		{"/dev/mmcblk0p1", "/dev/mmcblk0"},
		{"/dev/disk3s1", "/dev/disk3"},
		{"overlay", "overlay"},
	}
	for _, c := range cases {
		got := getPhysicalDiskName(c.in)
		if got != c.want {
			t.Fatalf("getPhysicalDiskName(%q)=%q, want %q", c.in, got, c.want)
		}
	}
}

func TestConsolidateDiskInfosEmptyWithCurrent(t *testing.T) {
	cur := &DiskSingelInfo{BootPath: "/dev/sda1", MountPath: "/"}
	res := consolidateDiskInfos(nil, cur)
	if len(res) != 1 {
		t.Fatalf("expected 1 result, got %d", len(res))
	}
	if res[0].BootPath != cur.BootPath || res[0].MountPath != cur.MountPath {
		t.Fatalf("unexpected current disk in result: %+v", res[0])
	}
}
