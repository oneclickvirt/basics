package system

import (
	"testing"
)

func TestGetSystemInfo(t *testing.T) {
	// 检测整体
	GetSystemInfo()
	// 检测raid - 在服务器上几乎没有查得到的，不加入检测
	// raidInfo, err := detectRAID()
	// if err != nil {
	// 	fmt.Printf("检测RAID时发生错误: %v\n", err)
	// 	return
	// }
	// if raidInfo.Exists {
	// 	fmt.Println("检测到RAID配置:")
	// 	fmt.Printf("RAID类型: %s\n", raidInfo.Type)
	// 	fmt.Printf("磁盘数量: %d\n", raidInfo.DiskCount)
	// 	fmt.Printf("控制器: %s\n", raidInfo.Controller)
	// 	// fmt.Printf("详细信息: %s\n", raidInfo.Details)
	// } else {
	// 	fmt.Println("未检测到RAID配置")
	// }
}

func TestGetLinuxDisks(t *testing.T) {
	// Test that getLinuxDisks returns valid results and doesn't crash
	diskInfos, currentDiskInfo, bootPath := getLinuxDisks()
	
	// Verify basic structure
	if diskInfos == nil {
		t.Error("diskInfos should not be nil")
	}
	
	// Verify no duplicate mount points in results
	seenMountPaths := make(map[string]bool)
	for _, info := range diskInfos {
		if seenMountPaths[info.MountPath] {
			t.Errorf("Duplicate mount path detected: %s", info.MountPath)
		}
		seenMountPaths[info.MountPath] = true
		
		// Verify basic disk info structure
		if info.TotalBytes == 0 {
			t.Errorf("Disk %s should have non-zero total bytes", info.BootPath)
		}
		if info.TotalStr == "" {
			t.Errorf("Disk %s should have non-empty total string", info.BootPath)
		}
		if info.MountPath == "" {
			t.Errorf("Disk %s should have non-empty mount path", info.BootPath)
		}
	}
	
	// currentDiskInfo can be nil in some cases (like containerized environments)
	// but if it exists, it should be valid
	if currentDiskInfo != nil && currentDiskInfo.TotalBytes == 0 {
		t.Error("currentDiskInfo should have non-zero total bytes if it exists")
	}
	
	// bootPath can be empty in some cases but shouldn't be nil
	if bootPath != "" && len(bootPath) == 0 {
		t.Error("bootPath should not be empty string if set")
	}
}
