package system

import (
	"github.com/oneclickvirt/gostun/model"
	"github.com/oneclickvirt/gostun/stuncheck"
)

func getNatType() string {
	model.EnableLoger = false
	model.Verbose = 0
	model.Timeout = 3
	model.IPVersion = "ipv4"
	addrStrList := model.GetDefaultServers(model.IPVersion)
	rfcMethods := []string{"RFC5780", "RFC5389", "RFC3489"}
	successfulDetection := false
	for _, rfcMethod := range rfcMethods {
		if successfulDetection {
			break
		}
		for _, addrStr := range addrStrList {
			model.NatMappingBehavior = ""
			model.NatFilteringBehavior = ""
			var err1, err2 error
			switch rfcMethod {
			case "RFC5780":
				err1 = stuncheck.MappingTests(addrStr)
				if err1 != nil {
					model.NatMappingBehavior = "inconclusive"
				}
				err2 = stuncheck.FilteringTests(addrStr)
				if err2 != nil {
					model.NatFilteringBehavior = "inconclusive"
				}
			case "RFC5389":
				err1 = stuncheck.MappingTestsRFC5389(addrStr)
				if err1 != nil {
					model.NatMappingBehavior = "inconclusive"
					model.NatFilteringBehavior = "inconclusive"
				}
			case "RFC3489":
				err1 = stuncheck.MappingTestsRFC3489(addrStr)
				if err1 != nil {
					model.NatMappingBehavior = "inconclusive"
					model.NatFilteringBehavior = "inconclusive"
				}
			}
			if model.NatMappingBehavior != "inconclusive" && model.NatFilteringBehavior != "inconclusive" &&
				model.NatMappingBehavior != "" && model.NatFilteringBehavior != "" {
				successfulDetection = true
				break
			}
		}
	}
	return stuncheck.CheckType()
}
