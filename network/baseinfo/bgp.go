package baseinfo

import (
	"bytes"
	"fmt"
	"image/color"
	"image/png"
	"io"
	"math"
	"net"

	"github.com/imroc/req/v3"
	"github.com/nfnt/resize"
)

// GetCIDRPrefix 获取 IP 地址的实际 CIDR 前缀，获取失败时返回默认值 24
func GetCIDRPrefix(ip string) int {
	interfaces, err := net.Interfaces()
	if err != nil {
		return 24
	}
	for _, iface := range interfaces {
		addrs, err := iface.Addrs()
		if err != nil {
			continue
		}
		for _, addr := range addrs {
			if ipNet, ok := addr.(*net.IPNet); ok && ipNet.IP.To4() != nil {
				if ipNet.IP.String() == ip {
					ones, _ := ipNet.Mask.Size()
					return ones
				}
			}
		}
	}
	return 24
}

func GetNeighborCount(ip string, prefixNum int) (int, int, error) {
	client := req.C()
	client.ImpersonateChrome()
	cidrBase := fmt.Sprintf("%s/%d", ip, prefixNum)
	neighborTotal := int(math.Pow(2, float64(32-prefixNum)))
	neighborActive, err := countActiveIPs(client, fmt.Sprintf("https://bgp.tools/pfximg/%s", cidrBase))
	if err != nil {
		return 0, 0, err
	}
	// ipTotal := int(math.Pow(2, float64(32-prefixNum)))
	// ipActive, err := countActiveIPs(client, fmt.Sprintf("https://bgp.tools/pfximg/%s", ip))
	// if err != nil {
	// 	return 0, 0, err
	// }
	// fmt.Printf("Active IPs: %d/%d\n", ipActive, ipTotal)
	return neighborActive, neighborTotal, nil
}

func countActiveIPs(client *req.Client, url string) (int, error) {
	resp, err := client.R().Get(url)
	if err != nil {
		return 0, err
	}
	if !resp.IsSuccessState() {
		return 0, fmt.Errorf("HTTP request failed: %s", resp.Status)
	}
	// 读取 PNG 数据到内存
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return 0, err
	}
	// 确保数据正确
	if len(data) < 8 || !bytes.HasPrefix(data, []byte("\x89PNG\r\n\x1a\n")) {
		return 0, fmt.Errorf("invalid PNG format")
	}
	// 解码 PNG
	img, err := png.Decode(bytes.NewReader(data))
	if err != nil {
		return 0, fmt.Errorf("failed to decode PNG: %w", err)
	}
	// 调整图像大小
	resizedImg := resize.Resize(0, 100, img, resize.Lanczos3)
	count := 0
	for y := 0; y < resizedImg.Bounds().Dy(); y++ {
		for x := 0; x < resizedImg.Bounds().Dx(); x++ {
			c := color.RGBAModel.Convert(resizedImg.At(x, y)).(color.RGBA)
			if c.R == 0 && c.G == 3 && c.B == 255 {
				count++
			}
		}
	}
	return count, nil
}
