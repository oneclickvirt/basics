package utils

import (
	"strconv"

	"github.com/oneclickvirt/basics/model"
)

func ParseIpInfo(data map[string]interface{}) *model.IpInfo {
	ipInfo := &model.IpInfo{}
	if ip, ok := data["ip"].(string); ok {
		ipInfo.Ip = ip
	}
	if location, ok := data["location"].(map[string]interface{}); ok {
		if city, ok := location["city"].(string); ok {
			ipInfo.City = city
		}
		if region, ok := location["region"].(map[string]interface{}); ok {
			if name, ok := region["name"].(string); ok {
				ipInfo.Region = name
			}
		}
		if country, ok := location["country"].(map[string]interface{}); ok {
			if name, ok := country["name"].(string); ok {
				ipInfo.Country = name
			}
		}
	}
	if connection, ok := data["connection"].(map[string]interface{}); ok {
		if asn, ok := connection["asn"].(float64); ok {
			ipInfo.ASN = strconv.Itoa(int(asn))
		}
		if org, ok := connection["organization"].(string); ok {
			ipInfo.Org = org
		}
	}
	return ipInfo
}
