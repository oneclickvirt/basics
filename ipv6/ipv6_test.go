package ipv6

import (
	"fmt"
	"testing"
)

func TestGetIPv6Mask(t *testing.T) {
	ipv6Info, err := GetIPv6Mask("zh")
	if err == nil {
		fmt.Println(ipv6Info)
	}
}
