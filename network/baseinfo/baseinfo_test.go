package baseinfo

import (
	"fmt"
	"github.com/oneclickvirt/basics/model"
	"testing"
	"time"
)

// printIPInfo 重构输出函数
func printIPInfo(ipInfo *model.IpInfo, err error) {
	if err != nil {
		fmt.Println("获取 IP 信息时出错:", err)
		return
	}
	if ipInfo != nil {
		fmt.Println("IPInfo:")
		fmt.Println("IP:", ipInfo.Ip)
		fmt.Println("ASN:", ipInfo.ASN)
		fmt.Println("Org:", ipInfo.Org)
		fmt.Println("Country:", ipInfo.Country)
		fmt.Println("Region:", ipInfo.Region)
		fmt.Println("City:", ipInfo.City)
		fmt.Println("---------------------------------")
	}
}

func TestIPInfo(t *testing.T) {
	// Test for IPv4
	fmt.Println("IPv4 Testing:")
	startV4 := time.Now()
	ipInfoV4Result, _, err := RunIpCheck("ipv4")
	elapsedV4 := time.Since(startV4)
	if err == nil {
		fmt.Println("IPv4:")
		fmt.Println("------")
		printIPInfo(ipInfoV4Result, nil)
	}
	fmt.Println("---***********************************---")
	// Test for IPv6
	fmt.Println("IPv6 Testing:")
	startV6 := time.Now()
	_, ipInfoV6Result, err := RunIpCheck("ipv6")
	elapsedV6 := time.Since(startV6)
	if err == nil {
		fmt.Println("IPv6:")
		fmt.Println("------")
		printIPInfo(ipInfoV6Result, nil)
	}
	fmt.Println("---***********************************---")
	// Test for both IPv4 and IPv6
	fmt.Println("Both Testing:")
	startBoth := time.Now()
	ipInfoV4Result, ipInfoV6Result, err = RunIpCheck("both")
	elapsedBoth := time.Since(startBoth)
	if err == nil {
		fmt.Println("IPv4:")
		fmt.Println("------")
		printIPInfo(ipInfoV4Result,nil)
		fmt.Println("IPv6:")
		fmt.Println("------")
		printIPInfo(ipInfoV6Result, nil)
	}
	fmt.Printf("IPv4 test took %s\n", elapsedV4)
	fmt.Printf("IPv6 test took %s\n", elapsedV6)
	fmt.Printf("Both test took %s\n", elapsedBoth)
}
