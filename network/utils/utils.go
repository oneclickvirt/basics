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
	if netType != "tcp4" && netType != "tcp6" {
		return nil, fmt.Errorf("Invalid netType: %s. Expected 'tcp4' or 'tcp6'.", netType)
	}
	client := req.C()
	client.SetTimeout(12 * time.Second).
		SetDial(func(ctx context.Context, network, addr string) (net.Conn, error) {
			return (&net.Dialer{
				Timeout:   6 * time.Second,
				KeepAlive: 30 * time.Second,
			}).DialContext(ctx, netType, addr)
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
