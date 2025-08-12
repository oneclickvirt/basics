package baseinfo

import (
	"context"
	"fmt"
	"net"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/imroc/req/v3"
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

// FetchMaxMind 从 MaxMind 获取 IP 信息
func FetchMaxMind(netType string) (*model.IpInfo, error) {
	data, err := utils.FetchJsonFromURL("https://geoip.maxmind.com/geoip/v2.1/city/me", netType, true, "Referer: https://www.maxmind.com/en/locate-my-ip-address")
	if err == nil {
		res := &model.IpInfo{}
		if traits, ok := data["traits"].(map[string]interface{}); ok {
			if ip, ok := traits["ip_address"].(string); ok && ip != "" {
				res.Ip = ip
			}
			if asnFloat, ok := traits["autonomous_system_number"].(float64); ok {
				res.ASN = strconv.FormatInt(int64(asnFloat), 10)
			}
			if org, ok := traits["autonomous_system_organization"].(string); ok && org != "" {
				res.Org = org
			}
		}
		if city, ok := data["city"].(map[string]interface{}); ok {
			if names, ok := city["names"].(map[string]interface{}); ok {
				if cityName, ok := names["en"].(string); ok && cityName != "" {
					res.City = cityName
				}
			}
		}
		if subdivisions, ok := data["subdivisions"].([]interface{}); ok && len(subdivisions) > 0 {
			if subdivision, ok := subdivisions[0].(map[string]interface{}); ok {
				if names, ok := subdivision["names"].(map[string]interface{}); ok {
					if regionName, ok := names["en"].(string); ok && regionName != "" {
						res.Region = regionName
					}
				}
			}
		}
		if country, ok := data["country"].(map[string]interface{}); ok {
			if names, ok := country["names"].(map[string]interface{}); ok {
				if countryName, ok := names["en"].(string); ok && countryName != "" {
					res.Country = countryName
				}
			}
		}
		return res, nil
	} else {
		return nil, err
	}
}

// FetchHackerTargetASN 使用HackerTarget获取ASN信息
func FetchHackerTargetASN(ip string, netType string) (*model.IpInfo, error) {
	url := fmt.Sprintf("https://api.hackertarget.com/aslookup/?q=%s", ip)
	// 检查网络类型是否有效
	if netType != "tcp4" && netType != "tcp6" {
		return nil, fmt.Errorf("Invalid netType: %s. Expected 'tcp4' or 'tcp6'.", netType)
	}
	// 创建 HTTP 客户端
	client := req.C()
	client.SetTimeout(6 * time.Second).
		SetDial(func(ctx context.Context, network, addr string) (net.Conn, error) {
			return (&net.Dialer{}).DialContext(ctx, netType, addr)
		}).
		SetTLSHandshakeTimeout(2 * time.Second).
		SetResponseHeaderTimeout(2 * time.Second).
		SetExpectContinueTimeout(2 * time.Second)
	client.R().
		SetRetryCount(2).
		SetRetryBackoffInterval(1*time.Second, 2*time.Second).
		SetRetryFixedInterval(1 * time.Second)
	// 执行请求
	resp, err := client.R().Get(url)
	if err != nil {
		return nil, err
	}
	// 检查响应状态码
	if !resp.IsSuccessState() {
		return nil, fmt.Errorf("Error fetching ASN info: status code %d", resp.StatusCode)
	}
	response := strings.TrimSpace(resp.String())
	if response == "" || strings.Contains(response, "error") {
		return nil, fmt.Errorf("no ASN data found")
	}
	// 格式是: "1.1.1.1","13335","1.1.1.0/24","CLOUDFLARENET, US"
	parts := strings.Split(response, ",")
	if len(parts) >= 4 {
		// 移除引号并获取ASN和组织信息
		asn := strings.Trim(parts[1], "\"")
		org := strings.Trim(parts[3], "\"")
		res := &model.IpInfo{
			Ip:  ip,
			ASN: asn,
			Org: strings.TrimSpace(org),
		}
		return res, nil
	}
	return nil, fmt.Errorf("invalid ASN response format")
}

// FetchIPApiASN 使用ip-api.com获取ASN信息
func FetchIPApiASN(ip string, netType string) (*model.IpInfo, error) {
	url := fmt.Sprintf("http://ip-api.com/json/%s?fields=as", ip)
	data, err := utils.FetchJsonFromURL(url, netType, false, "")
	if err != nil {
		return nil, err
	}
	res := &model.IpInfo{Ip: ip}
	// as字段格式是 "AS13335 Cloudflare, Inc."
	if as, ok := data["as"].(string); ok && as != "" {
		parts := strings.Fields(as)
		if len(parts) > 0 {
			asn := strings.TrimPrefix(parts[0], "AS")
			res.ASN = asn
			if len(parts) > 1 {
				res.Org = strings.Join(parts[1:], " ")
			}
		}
	}
	return res, nil
}

// hasGeoInfo 检查是否有地理信息
func hasGeoInfo(info *model.IpInfo) bool {
	if info == nil {
		return false
	}
	return info.Country != "" || info.Region != "" || info.City != ""
}

// needsASNFallback 检查是否需要ASN备用查询
func needsASNFallback(info *model.IpInfo) bool {
	if info == nil {
		return false
	}
	return hasGeoInfo(info) && info.ASN == ""
}

// fillASNWithFallback 使用备用方法补充ASN信息
func fillASNWithFallback(ipInfo *model.IpInfo, netType string) *model.IpInfo {
	if !needsASNFallback(ipInfo) {
		return ipInfo
	}
	asnFunctions := []func(string, string) (*model.IpInfo, error){
		FetchHackerTargetASN,
		FetchIPApiASN,
	}
	asnFuncNames := []string{
		"hackertarget",
		"ipapi",
	}
	// 通道收集ASN结果
	asnChan := make(chan *ipInfoWithSource, len(asnFunctions))
	var wg sync.WaitGroup
	// 并发执行ASN查询
	wg.Add(len(asnFunctions))
	for i, fn := range asnFunctions {
		go func(f func(string, string) (*model.IpInfo, error), name string) {
			defer wg.Done()
			if result, err := f(ipInfo.Ip, netType); err == nil && result != nil {
				select {
				case asnChan <- &ipInfoWithSource{info: result, source: name}:
				default:
				}
			} else {
				select {
				case asnChan <- &ipInfoWithSource{info: nil, source: name}:
				default:
				}
			}
		}(fn, asnFuncNames[i])
	}
	go func() {
		wg.Wait()
		close(asnChan)
	}()
	// 收集ASN查询结果
	asnResults := make([]*ipInfoWithSource, 0)
	for asnResult := range asnChan {
		asnResults = append(asnResults, asnResult)
	}
	// ASN查询的优先级顺序
	asnOrderMap := map[string]int{
		"asnlookup":    0,
		"hackertarget": 1,
		"ipapi":        2,
	}
	// 按优先级顺序处理ASN结果
	for order := 0; order < len(asnFuncNames); order++ {
		for _, asnResult := range asnResults {
			if asnOrderMap[asnResult.source] == order && asnResult.info != nil {
				// 补充ASN信息
				if ipInfo.ASN == "" && asnResult.info.ASN != "" {
					ipInfo.ASN = asnResult.info.ASN
				}
				if ipInfo.Org == "" && asnResult.info.Org != "" {
					ipInfo.Org = asnResult.info.Org
				}
				// 如果ASN已经补充完整，直接返回
				if ipInfo.ASN != "" {
					if model.EnableLoger {
						Logger.Info(fmt.Sprintf("ASN filled by %s: ASN=%s, Org=%s", asnResult.source, ipInfo.ASN, ipInfo.Org))
					}
					return ipInfo
				}
			}
		}
	}
	return ipInfo
}

// ipInfoWithSource 包装IP信息和来源
type ipInfoWithSource struct {
	info   *model.IpInfo
	source string
}

// executeFunctions 并发执行函数
// 仅区分IPV4或IPV6，BOTH的情况需要两次执行本函数分别指定
func executeFunctions(checkType string, fetchFunc func(string) (*model.IpInfo, error), funcName string, ipInfoChan chan *ipInfoWithSource, wg *sync.WaitGroup) {
	defer wg.Done()
	ipFetcher := func(ipType string) {
		ipInfo, err := fetchFunc(ipType)
		if err == nil {
			select {
			case ipInfoChan <- &ipInfoWithSource{info: ipInfo, source: funcName}:
			default:
			}
		} else {
			select {
			case ipInfoChan <- &ipInfoWithSource{info: nil, source: funcName}:
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
	functions := []func(string) (*model.IpInfo, error){
		FetchIPInfoIo,
		FetchMaxMind,
		FetchCloudFlare,
		FetchIpSb,
	}
	funcNames := []string{
		"ipinfo",
		"maxmind",
		"cloudflare",
		"ipsb",
	}
	ipInfoIPv4 := make(chan *ipInfoWithSource, len(functions))
	ipInfoIPv6 := make(chan *ipInfoWithSource, len(functions))
	var wg sync.WaitGroup
	if checkType == "both" {
		wg.Add(len(functions) * 2) // 每个函数都会产生一个 IPv4 和一个 IPv6 结果
		// 启动协程执行函数
		for i, f := range functions {
			go executeFunctions("ipv4", f, funcNames[i], ipInfoIPv4, &wg)
			go executeFunctions("ipv6", f, funcNames[i], ipInfoIPv6, &wg)
		}
	} else if checkType == "ipv4" {
		wg.Add(len(functions)) // 每个函数都会产生一个 IPv4 结果
		// 启动协程执行函数
		for i, f := range functions {
			go executeFunctions("ipv4", f, funcNames[i], ipInfoIPv4, &wg)
		}
	} else if checkType == "ipv6" {
		wg.Add(len(functions)) // 每个函数都会产生一个 IPv6 结果
		// 启动协程执行函数
		for i, f := range functions {
			go executeFunctions("ipv6", f, funcNames[i], ipInfoIPv6, &wg)
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
	// 收集并排序IPv4结果
	ipInfoV4List := make([]*ipInfoWithSource, 0)
	for ipInfo := range ipInfoIPv4 {
		ipInfoV4List = append(ipInfoV4List, ipInfo)
	}
	// 收集并排序IPv6结果
	ipInfoV6List := make([]*ipInfoWithSource, 0)
	for ipInfo := range ipInfoIPv6 {
		ipInfoV6List = append(ipInfoV6List, ipInfo)
	}
	// 排序顺序
	orderMap := map[string]int{
		"ipinfo":     0,
		"maxmind":    1,
		"cloudflare": 2,
		"ipsb":       3,
	}
	// 按顺序处理IPv4结果
	var ipInfoV4Result *model.IpInfo
	for order := 0; order < len(funcNames); order++ {
		for _, ipInfoWithSrc := range ipInfoV4List {
			if orderMap[ipInfoWithSrc.source] == order && ipInfoWithSrc.info != nil {
				if ipInfoV4Result == nil {
					ipInfoV4Result = &model.IpInfo{}
				}
				ipInfoV4TempResult, err := utils.CompareAndMergeIpInfo(ipInfoV4Result, ipInfoWithSrc.info)
				if err == nil {
					ipInfoV4Result = ipInfoV4TempResult
				} else {
					if model.EnableLoger {
						Logger.Info(fmt.Sprintf("utils.CompareAndMergeIpInfo(ipInfoV4Result, ipInfo): %s", err.Error()))
					}
				}
			}
		}
	}
	// 按顺序处理IPv6结果
	var ipInfoV6Result *model.IpInfo
	for order := 0; order < len(funcNames); order++ {
		for _, ipInfoWithSrc := range ipInfoV6List {
			if orderMap[ipInfoWithSrc.source] == order && ipInfoWithSrc.info != nil {
				if ipInfoV6Result == nil {
					ipInfoV6Result = &model.IpInfo{}
				}
				ipInfoV6TempResult, err := utils.CompareAndMergeIpInfo(ipInfoV6Result, ipInfoWithSrc.info)
				if err == nil {
					ipInfoV6Result = ipInfoV6TempResult
				} else {
					if model.EnableLoger {
						Logger.Info(fmt.Sprintf("utils.CompareAndMergeIpInfo(ipInfoV6Result, ipInfo): %s", err.Error()))
					}
				}
			}
		}
	}
	// 如果地理信息存在但ASN缺失，使用备用方法补充ASN信息
	if needsASNFallback(ipInfoV4Result) {
		if model.EnableLoger {
			Logger.Info("IPv4 ASN missing, trying fallback methods")
		}
		ipInfoV4Result = fillASNWithFallback(ipInfoV4Result, "tcp4")
	}
	if needsASNFallback(ipInfoV6Result) {
		if model.EnableLoger {
			Logger.Info("IPv6 ASN missing, trying fallback methods")
		}
		ipInfoV6Result = fillASNWithFallback(ipInfoV6Result, "tcp6")
	}
	return ipInfoV4Result, ipInfoV6Result, nil
}
