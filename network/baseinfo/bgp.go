package baseinfo

import (
	"bytes"
	"encoding/json"
	"fmt"
	"image/color"
	"image/png"
	"io"
	"math"
	"net"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/imroc/req/v3"
	"github.com/nfnt/resize"
	"github.com/oneclickvirt/basics/model"
	. "github.com/oneclickvirt/defaultset"
)

// GetCIDRPrefix
func GetCIDRPrefix(ip string) (string, int) {
	if model.EnableLoger {
		InitLogger()
		defer Logger.Sync()
	}
	client := req.C()
	client.ImpersonateChrome()
	client.SetTimeout(6 * time.Second)
	cidrIp, cidrPrefix, err := fetchCIDRFromBGPToolsAndHe(client, ip)
	if err == nil && cidrPrefix > 0 {
		return cidrIp, cidrPrefix
	}
	if model.EnableLoger {
		Logger.Info("Can not find ipv4 BGP CIDR")
	}
	return "", -1
}

// fetchCIDRFromBGPToolsAndHe 通过 BGP Tools 和 HE 查询 CIDR 前缀
func fetchCIDRFromBGPToolsAndHe(client *req.Client, ip string) (string, int, error) {
	// 先尝试从 HE 获取 CIDR
	heURL := fmt.Sprintf("https://bgp.he.net/whois/ip/%s", ip)
	heResp, err := client.R().Get(heURL)
	if err == nil && heResp.IsSuccessState() {
		cidr := parseCIDRFromHE(heResp.String())
		if cidr != "" {
			cidrs := strings.Split(cidr, "/")
			if len(cidrs) == 2 {
				cidrNum, _ := strconv.Atoi(cidrs[1])
				return cidrs[0], cidrNum, nil
			}
		}
	}
	// 如果 HE 解析失败，尝试从 BGP Tools 获取 CIDR
	bgpURL := fmt.Sprintf("https://bgp.tools/prefix/%s", ip)
	bgpResp, err := client.R().Get(bgpURL)
	if err != nil {
		return "", -1, err
	}
	if !bgpResp.IsSuccessState() {
		return "", -1, fmt.Errorf("BGP Tools HTTP request failed: %s", bgpResp.Status)
	}
	cidr := parseCIDRFromBGPTools(bgpResp.String())
	if cidr == "" {
		return "", -1, fmt.Errorf("failed to extract CIDR from BGP Tools")
	}
	cidrs := strings.Split(cidr, "/")
	if len(cidrs) != 2 {
		return "", -1, fmt.Errorf("failed to extract CIDR from BGP Tools")
	}
	// fmt.Println("bgp", cidr)
	cidrNum, _ := strconv.Atoi(cidrs[1])
	return cidrs[0], cidrNum, nil
}

// parseCIDRFromHE 解析 HE 的 whois 数据，提取 CIDR
func parseCIDRFromHE(jsonData string) string {
	var result map[string]string
	err := json.Unmarshal([]byte(jsonData), &result)
	if err != nil {
		return ""
	}
	data, ok := result["data"]
	if !ok {
		return ""
	}
	re := regexp.MustCompile(`CIDR:\s+([0-9./]+)`)
	matches := re.FindStringSubmatch(data)
	if len(matches) > 1 {
		return matches[1]
	}
	return ""
}

// parseCIDRFromBGPTools 解析 BGP Tools HTML，提取 CIDR
func parseCIDRFromBGPTools(data string) string {
	re := regexp.MustCompile(`(?m)<td class="smallonmobile nowrap"><a href="/prefix/([0-9./]+)">`) // 提取 <a href="/prefix/54.92.128.0/17">
	matches := re.FindStringSubmatch(data)
	if len(matches) > 1 {
		return matches[1]
	}
	return ""
}

func MaskIP(ipStr string) string {
	ip := net.ParseIP(ipStr)
	if ip == nil {
		return ""
	}
	parts := strings.Split(ipStr, ".")
	if len(parts) == 4 {
		parts[3] = "0"
		return strings.Join(parts, ".")
	}
	return ""
}

func GetActiveIpsCount(ip string, prefixNum int) (int, int, error) {
	if ip == "" {
		return 0, 0, fmt.Errorf("IP address cannot be empty")
	}
	if prefixNum < 0 || prefixNum > 32 {
		return 0, 0, fmt.Errorf("prefixNum must be between 0 and 32")
	}
	client := req.C()
	client.ImpersonateChrome()
	cidrBase := fmt.Sprintf("%s/%d", ip, prefixNum)
	total := int(math.Pow(2, float64(32-prefixNum)))
	active, err := countActiveIPs(client, fmt.Sprintf("https://bgp.tools/pfximg/%s", cidrBase))
	if err != nil {
		return 0, 0, err
	}
	return active, total, nil
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
