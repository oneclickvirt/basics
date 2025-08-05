package network

import (
	"fmt"

	"github.com/oneclickvirt/basics/model"
	"github.com/oneclickvirt/basics/network/baseinfo"
	"github.com/oneclickvirt/basics/network/ipv6"
	. "github.com/oneclickvirt/defaultset"
)

// sortAndTranslateText 对原始文本进行排序和翻译
// func sortAndTranslateText(orginList []string, language string, fields []string) string {
// 	var result string
// 	for _, key := range fields {
// 		var displayKey string
// 		if language == "zh" {
// 			displayKey = model.TranslationMap[key]
// 			if displayKey == "" {
// 				displayKey = key
// 			}
// 		} else {
// 			displayKey = key
// 		}
// 		for _, line := range orginList {
// 			if strings.Contains(line, key) {
// 				if displayKey == key {
// 					result = result + line + "\n"
// 				} else {
// 					result = result + strings.ReplaceAll(line, key, displayKey) + "\n"
// 				}
// 				break
// 			}
// 		}
// 	}
// 	return result
// }

// processPrintIPInfo 处理IP信息
func processPrintIPInfo(ipVersion, language string, ipResult *model.IpInfo) string {
	var info string
	var headASNString, headLocationString string
	if ipVersion == "ipv4" {
		headASNString = " IPV4 ASN            : "
		headLocationString = " IPV4 Location       : "
	} else if ipVersion == "ipv6" {
		headASNString = " IPV6 ASN            : "
		headLocationString = " IPV6 Location       : "
	}
	// 处理ASN信息
	if ipResult.ASN != "" || ipResult.Org != "" {
		info += headASNString
		if ipResult.ASN != "" {
			info += "AS" + ipResult.ASN
			if ipResult.Org != "" {
				info += " "
			}
		}
		info += ipResult.Org + "\n"
	}
	// 处理位置信息
	if ipResult.City != "" || ipResult.Region != "" || ipResult.Country != "" {
		info += headLocationString
		if ipResult.City != "" {
			info += ipResult.City + " / "
		}
		if ipResult.Region != "" {
			info += ipResult.Region + " / "
		}
		if ipResult.Country != "" {
			info += ipResult.Country
		}
		info += "\n"
	}
	// 处理 IPv4 的活跃IP信息
	if ipVersion == "ipv4" && ipResult.Ip != "" && baseinfo.MaskIP(ipResult.Ip) != "" {
		if model.EnableLoger {
			InitLogger()
			defer Logger.Sync()
		}
		subnetCidrIp := baseinfo.MaskIP(ipResult.Ip)
		subnetActive, subnetTotal, err1 := baseinfo.GetActiveIpsCount(subnetCidrIp, 24)
		cidrIp, cidrPrefix := baseinfo.GetCIDRPrefix(ipResult.Ip)
		prefixActive, prefixTotal, err2 := baseinfo.GetActiveIpsCount(cidrIp, cidrPrefix)
		if (err1 == nil && subnetActive > 0 && subnetTotal > 0) || (err2 == nil && prefixActive > 0 && prefixTotal > 0) {
			info += " IPV4 Active IPs     :"
			if err1 == nil && subnetActive > 0 && subnetTotal > 0 {
				info += fmt.Sprintf(" %d/%d (subnet /24)", subnetActive, subnetTotal)
			} else if err1 != nil && model.EnableLoger {
				Logger.Info(fmt.Sprintf("subnet /24 data unavailable: %s", err1.Error()))
			}
			if err2 == nil && prefixActive > 0 && prefixTotal > 0 {
				if cidrPrefix != 24 {
					info += fmt.Sprintf(" %d/%d (prefix /%d)", prefixActive, prefixTotal, cidrPrefix)
				}
			} else if err2 != nil && model.EnableLoger {
				Logger.Info(fmt.Sprintf("prefix data unavailable: %s", err2.Error()))
			}
			info += "\n"
		}
	}
	// 处理 Ipv6 的Mask信息
	if ipVersion == "ipv6" && ipResult.Ip != "" {
		maskInfoV6, err := ipv6.GetIPv6Mask(ipResult.Ip, language)
		if err == nil {
			info += maskInfoV6
		}
	}
	return info
}

// NetworkCheck 查询网络信息
// checkType 可选 both ipv4 ipv6
// language 暂时仅支持 en 或 zh
func NetworkCheck(checkType string, enableSecurityCheck bool, language string) (string, string, string, string, error) {
	if model.EnableLoger {
		InitLogger()
		defer Logger.Sync()
	}
	var ipv4, ipv6, ipInfo string
	if checkType == "both" {
		ipInfoV4Result, ipInfoV6Result, err := baseinfo.RunIpCheck("both")
		if err != nil && model.EnableLoger {
			Logger.Info(err.Error())
		}
		if ipInfoV4Result != nil {
			ipInfo += processPrintIPInfo("ipv4", language, ipInfoV4Result)
			ipv4 = ipInfoV4Result.Ip
		}
		if ipInfoV6Result != nil {
			ipInfo += processPrintIPInfo("ipv6", language, ipInfoV6Result)
			ipv6 = ipInfoV4Result.Ip
		}
		return ipv4, ipv6, ipInfo, "", nil
	} else if checkType == "ipv4" {
		ipInfoV4Result, _, err := baseinfo.RunIpCheck("ipv4")
		if err != nil && model.EnableLoger {
			Logger.Info(err.Error())
		}
		if ipInfoV4Result != nil {
			ipInfo += processPrintIPInfo("ipv4", language, ipInfoV4Result)
			ipv4 = ipInfoV4Result.Ip
		}
		return ipv4, ipv6, ipInfo, "", nil
	} else if checkType == "ipv6" {
		ipInfoV6Result, _, err := baseinfo.RunIpCheck("ipv6")
		if err != nil && model.EnableLoger {
			Logger.Info(err.Error())
		}
		if ipInfoV6Result != nil {
			ipInfo += processPrintIPInfo("ipv6", language, ipInfoV6Result)
			ipv6 = ipInfoV6Result.Ip
		}
		return ipv4, ipv6, ipInfo, "", nil
	}
	return "", "", "", "", fmt.Errorf("wrong in NetworkCheck")
}
