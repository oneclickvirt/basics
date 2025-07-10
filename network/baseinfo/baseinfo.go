package baseinfo

import (
	"fmt"
	"strconv"
	"strings"
	"sync"

	"github.com/oneclickvirt/basics/model"
	"github.com/oneclickvirt/basics/network/utils"
	. "github.com/oneclickvirt/defaultset"
)

// FetchIPInfoIo 从 ipinfo.io 获取 IP 信息
func FetchIPInfoIo(netType string) (*model.IpInfo, error) {
	data, err := utils.FetchJsonFromURL("http://ipinfo.io", netType, false, "")
	if err == nil {
		res := &model.IpInfo{}
		if ip, ok := data["ip"].(string); ok && ip != "" {
			res.Ip = ip
		}
		if city, ok := data["city"].(string); ok && city != "" {
			res.City = city
		}
		if region, ok := data["region"].(string); ok && region != "" {
			res.Region = region
		}
		if country, ok := data["country"].(string); ok && country != "" {
			res.Country = country
		}
		if org, ok := data["org"].(string); ok && org != "" {
			parts := strings.Split(org, " ")
			if len(parts) > 0 {
				res.ASN = parts[0]
				res.Org = strings.Join(parts[1:], " ")
			} else {
				res.ASN = org
			}
		}
		return res, nil
	} else {
		return nil, err
	}
}

// FetchCloudFlare 从 speed.cloudflare.com 获取 IP 信息
func FetchCloudFlare(netType string) (*model.IpInfo, error) {
	data, err := utils.FetchJsonFromURL("https://speed.cloudflare.com/meta", netType, false, "")
	if err == nil {
		res := &model.IpInfo{}
		if ip, ok := data["clientIp"].(string); ok && ip != "" {
			res.Ip = ip
		}
		if city, ok := data["city"].(string); ok && city != "" {
			res.City = city
		}
		if region, ok := data["region"].(string); ok && region != "" {
			res.Region = region
		}
		if country, ok := data["country"].(string); ok && country != "" {
			res.Country = country
		}
		if asnFloat, ok := data["asn"].(float64); ok {
			res.ASN = strconv.FormatInt(int64(asnFloat), 10)
		} else if asnStr, ok := data["asn"].(string); ok && asnStr != "" {
			res.ASN = asnStr
		}
		if org, ok := data["asOrganization"].(string); ok && org != "" {
			res.Org = org
		}
		return res, nil
	} else {
		return nil, err
	}
}

// FetchIpSb 从 api.ip.sb 获取 IP 信息
func FetchIpSb(netType string) (*model.IpInfo, error) {
	data, err := utils.FetchJsonFromURL("https://api.ip.sb/geoip", netType, true, "")
	if err == nil {
		res := &model.IpInfo{}
		if ip, ok := data["ip"].(string); ok && ip != "" {
			res.Ip = ip
		}
		if city, ok := data["city"].(string); ok && city != "" {
			res.City = city
		}
		if region, ok := data["region"].(string); ok && region != "" {
			res.Region = region
		}
		if country, ok := data["country"].(string); ok && country != "" {
			res.Country = country
		}
		if asnFloat, ok := data["asn"].(float64); ok {
			res.ASN = strconv.FormatInt(int64(asnFloat), 10)
		} else if asnStr, ok := data["asn"].(string); ok && asnStr != "" {
			res.ASN = asnStr
		}
		if org, ok := data["asn_organization"].(string); ok && org != "" {
			res.Org = org
		}
		return res, nil
	} else {
		return nil, err
	}
}

// executeFunctions 并发执行函数
// 仅区分IPV4或IPV6，BOTH的情况需要两次执行本函数分别指定
func executeFunctions(checkType string, fetchFunc func(string) (*model.IpInfo, error), ipInfoChan chan *model.IpInfo, wg *sync.WaitGroup) {
	defer wg.Done()
	ipFetcher := func(ipType string) {
		ipInfo, err := fetchFunc(ipType)
		if err == nil {
			select {
			case ipInfoChan <- ipInfo:
			default:
			}
		} else {
			select {
			case ipInfoChan <- nil:
			default:
			}
		}
	}
	if checkType == "ipv4" {
		wg.Add(1)
		go func() {
			defer wg.Done()
			ipFetcher("tcp4")
		}()
	}
	if checkType == "ipv6" {
		wg.Add(1)
		go func() {
			defer wg.Done()
			ipFetcher("tcp6")
		}()
	}
}

// RunIpCheck 并发请求获取信息
func RunIpCheck(checkType string) (*model.IpInfo, *model.IpInfo, error) {
	if model.EnableLoger {
		InitLogger()
		defer Logger.Sync()
	}
	// 定义函数名数组
	functions := []func(string) (*model.IpInfo, error){
		FetchIPInfoIo,
		FetchCloudFlare,
		FetchIpSb,
	}
	// 定义通道
	ipInfoIPv4 := make(chan *model.IpInfo, len(functions))
	ipInfoIPv6 := make(chan *model.IpInfo, len(functions))
	var wg sync.WaitGroup
	if checkType == "both" {
		wg.Add(len(functions) * 2) // 每个函数都会产生一个 IPv4 和一个 IPv6 结果
		// 启动协程执行函数
		for _, f := range functions {
			go executeFunctions("ipv4", f, ipInfoIPv4, &wg)
			go executeFunctions("ipv6", f, ipInfoIPv6, &wg)
		}
	} else if checkType == "ipv4" {
		wg.Add(len(functions)) // 每个函数都会产生一个 IPv4 结果
		// 启动协程执行函数
		for _, f := range functions {
			go executeFunctions("ipv4", f, ipInfoIPv4, &wg)
		}
	} else if checkType == "ipv6" {
		wg.Add(len(functions)) // 每个函数都会产生一个 IPv6 结果
		// 启动协程执行函数
		for _, f := range functions {
			go executeFunctions("ipv6", f, ipInfoIPv6, &wg)
		}
	} else {
		if model.EnableLoger {
			Logger.Info("RunIpCheck: wrong checkType")
		}
		return nil, nil, fmt.Errorf("wrong checkType")
	}
	go func() {
		wg.Wait()
		close(ipInfoIPv4)
		close(ipInfoIPv6)
	}()
	// 读取结果并处理
	var ipInfoV4Result *model.IpInfo
	var ipInfoV6Result *model.IpInfo
	for ipInfo := range ipInfoIPv4 {
		if ipInfo != nil {
			if ipInfoV4Result == nil {
				ipInfoV4Result = &model.IpInfo{}
			}
			ipInfoV4TempResult, err := utils.CompareAndMergeIpInfo(ipInfoV4Result, ipInfo)
			if err == nil {
				ipInfoV4Result = ipInfoV4TempResult
			} else {
				if model.EnableLoger {
					Logger.Info(fmt.Sprintf("utils.CompareAndMergeIpInfo(ipInfoV4Result, ipInfo): %s", err.Error()))
				}
			}
		}
	}
	for ipInfo := range ipInfoIPv6 {
		if ipInfo != nil {
			if ipInfoV6Result == nil {
				ipInfoV6Result = &model.IpInfo{}
			}
			ipInfoV6TempResult, err := utils.CompareAndMergeIpInfo(ipInfoV6Result, ipInfo)
			if err == nil {
				ipInfoV6Result = ipInfoV6TempResult
			} else {
				if model.EnableLoger {
					Logger.Info(fmt.Sprintf("utils.CompareAndMergeIpInfo(ipInfoV6Result, ipInfo): %s", err.Error()))
				}
			}
		}
	}
	return ipInfoV4Result, ipInfoV6Result, nil
}
