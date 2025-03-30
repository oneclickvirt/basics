package network

import (
	"fmt"

	"github.com/oneclickvirt/basics/model"
	"github.com/oneclickvirt/basics/network/baseinfo"
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
func processPrintIPInfo(ipVersion string, ipResult *model.IpInfo) string {
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
	// 仅处理 IPv4 的活跃IP信息
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
			if err1 == nil {
				info += fmt.Sprintf(" %d/%d (subnet /24)", subnetActive, subnetTotal)
			} else {
				if model.EnableLoger {
					Logger.Info(fmt.Sprintf("subnet /24 data unavailable: %s", err1.Error()))
				}
			}
			if err2 == nil {
				if cidrPrefix != 24 {
					info += fmt.Sprintf(" %d/%d (prefix /%d)", prefixActive, prefixTotal, cidrPrefix)
				}
			} else {
				if model.EnableLoger {
					Logger.Info(fmt.Sprintf("prefix data unavailable: %s", err2.Error()))
				}
			}
			info += "\n"
		}
	}
	return info
}

// NetworkCheck 查询网络信息
// checkType 可选 both ipv4 ipv6
// language 暂时仅支持 en 或 zh
func NetworkCheck(checkType string, enableSecurityCheck bool, language string) (string, string, error) {
	if model.EnableLoger {
		InitLogger()
		defer Logger.Sync()
	}
	var ipInfo string
	if checkType == "both" {
		ipInfoV4Result, _, ipInfoV6Result, _, err := baseinfo.RunIpCheck("both")
		if err != nil && model.EnableLoger {
			Logger.Info(err.Error())
		}
		if ipInfoV4Result != nil {
			ipInfo += processPrintIPInfo("ipv4", ipInfoV4Result)
		}
		if ipInfoV6Result != nil {
			ipInfo += processPrintIPInfo("ipv6", ipInfoV6Result)
		}
		return ipInfo, "", nil
	} else if checkType == "ipv4" {
		ipInfoV4Result, _, _, _, err := baseinfo.RunIpCheck("ipv4")
		if err != nil && model.EnableLoger {
			Logger.Info(err.Error())
		}
		if ipInfoV4Result != nil {
			ipInfo += processPrintIPInfo("ipv4", ipInfoV4Result)
		}
		return ipInfo, "", nil
	} else if checkType == "ipv6" {
		_, _, ipInfoV6Result, _, err := baseinfo.RunIpCheck("ipv6")
		if err != nil && model.EnableLoger {
			Logger.Info(err.Error())
		}
		if ipInfoV6Result != nil {
			ipInfo += processPrintIPInfo("ipv6", ipInfoV6Result)
		}
		return ipInfo, "", nil
	}
	return "", "", fmt.Errorf("wrong in NetworkCheck")
}
