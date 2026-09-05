package utils

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"reflect"
	"strings"
	"time"

	"github.com/imroc/req/v3"
)

// FetchJsonFromURL 函数用于从指定的 URL 获取信息
// url 参数表示要获取信息的 URL
// netType 参数表示网络类型，只能为 "tcp4" 或 "tcp6"。
// enableHeader 参数表示是否启用请求头信息。
// additionalHeader 参数表示传入的额外的请求头信息(用于传输api的key)。
// 返回一个解析 json 得到的 map 和 一个可能发生的错误 。
func FetchJsonFromURL(url, netType string, enableHeader bool, additionalHeader string) (map[string]interface{}, error) {
	netType, err := NormalizeNetwork(netType)
	if err != nil {
		return nil, err
	}
	client := req.C()
	dialNetwork := netType
	if dialNetwork == "" {
		dialNetwork = "tcp"
	}
	client.SetTimeout(12 * time.Second).
		SetDial(func(ctx context.Context, network, addr string) (net.Conn, error) {
			return (&net.Dialer{
				Timeout:   6 * time.Second,
				KeepAlive: 30 * time.Second,
			}).DialContext(ctx, dialNetwork, addr)
		}).
		SetTLSHandshakeTimeout(5 * time.Second).
		SetResponseHeaderTimeout(10 * time.Second).
		SetExpectContinueTimeout(2 * time.Second)
	// client.SetTLSClientConfig(&tls.Config{
	// 	NextProtos: []string{"http/1.1"},
	// })
	client.R().
		SetRetryCount(3).
		SetRetryBackoffInterval(2*time.Second, 5*time.Second).
		SetRetryHook(func(resp *req.Response, err error) {
			if err != nil && (strings.Contains(err.Error(), "timeout") ||
				strings.Contains(err.Error(), "http2")) {
			}
		})
	if enableHeader {
		client.Headers = make(http.Header)
		client.ImpersonateChrome()
		client.Headers.Set("Connection", "close")
		if additionalHeader != "" {
			tempList := strings.Split(additionalHeader, ":")
			if len(tempList) == 2 {
				client.Headers.Set(tempList[0], tempList[1])
			} else if len(tempList) > 2 {
				client.Headers.Set(tempList[0], strings.Join(tempList[1:], ":"))
			}
		}
	}
	resp, err := client.R().Get(url)
	if err != nil {
		return nil, fmt.Errorf("Error fetching %s info: %v", url, err)
	}
	if !resp.IsSuccessState() {
		return nil, fmt.Errorf("Error fetching %s info: status code %d", url, resp.StatusCode)
	}
	var data map[string]interface{}
	err = json.Unmarshal(resp.Bytes(), &data)
	if err != nil {
		return nil, fmt.Errorf("Error decoding %s info: %v", url, err)
	}
	return data, nil
}

// NormalizeNetwork validates an optional IP-family selector. An empty value,
// "auto", and "tcp" preserve the standard library's normal dual-stack
// behavior; tcp4/tcp6 make the caller's family intent explicit.
func NormalizeNetwork(network string) (string, error) {
	network = strings.ToLower(strings.TrimSpace(network))
	switch network {
	case "", "auto", "tcp":
		return "", nil
	case "tcp4", "ipv4", "4":
		return "tcp4", nil
	case "tcp6", "ipv6", "6":
		return "tcp6", nil
	default:
		return "", fmt.Errorf("invalid network: %s (expected auto, tcp4, or tcp6)", network)
	}
}

// DialContext returns a family-aware dial function suitable for
// http.Transport, req.Client, and other request wrappers. The automatic form
// delegates to net.Dialer unchanged so pure IPv6 callers remain supported.
func DialContext(network string) (func(context.Context, string, string) (net.Conn, error), error) {
	network, err := NormalizeNetwork(network)
	if err != nil {
		return nil, err
	}
	dialer := &net.Dialer{Timeout: 6 * time.Second, KeepAlive: 30 * time.Second}
	if network == "" {
		return dialer.DialContext, nil
	}
	return func(ctx context.Context, _ string, address string) (net.Conn, error) {
		return dialer.DialContext(ctx, network, address)
	}, nil
}

// BoolToString 将布尔值转换为对应的字符串表示，true 则返回 "Yes"，false 则返回 "No"
func BoolToString(value bool) string {
	if value {
		return "Yes"
	}
	return "No"
}

// ExtractFieldNames 获取结构体的属性名字
func ExtractFieldNames(data interface{}) []string {
	var fields []string
	val := reflect.ValueOf(data).Elem()
	for i := 0; i < val.NumField(); i++ {
		field := val.Type().Field(i)
		name := field.Name
		if name != "Tag" {
			fields = append(fields, name)
		}
	}
	return fields
}
