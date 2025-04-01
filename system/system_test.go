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
