package utils

import (
	"fmt"

	"github.com/oneclickvirt/basics/model"
)


// chooseString 用于选择非空字符串
func chooseString(src, dst string) string {
	if src != "" {
		return src
	}
	return dst
}

// CompareAndMergeIpInfo 用于比较和合并两个 IpInfo 结构体，非空则不替换
func CompareAndMergeIpInfo(dst, src *model.IpInfo) (res *model.IpInfo, err error) {
	if src == nil {
		return nil, fmt.Errorf("Error merge IpInfo")
	}
	if dst == nil {
		dst = &model.IpInfo{}
	}
	dst.Ip = chooseString(src.Ip, dst.Ip)
	dst.ASN = chooseString(src.ASN, dst.ASN)
	dst.Org = chooseString(src.Org, dst.Org)
	dst.Country = chooseString(src.Country, dst.Country)
	dst.Region = chooseString(src.Region, dst.Region)
	dst.City = chooseString(src.City, dst.City)
	return dst, nil
}