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
)

// FetchJsonFromURL 函数用于从指定的 URL 获取信息
// url 参数表示要获取信息的 URL
// netType 参数表示网络类型，只能为 "tcp4" 或 "tcp6"。
// enableHeader 参数表示是否启用请求头信息。
// additionalHeader 参数表示传入的额外的请求头信息(用于传输api的key)。
// 返回一个解析 json 得到的 map 和 一个可能发生的错误 。
func FetchJsonFromURL(url, netType string, enableHeader bool, additionalHeader string) (map[string]interface{}, error) {
	// 检查网络类型是否有效
	if netType != "tcp4" && netType != "tcp6" {
		return nil, fmt.Errorf("Invalid netType: %s. Expected 'tcp4' or 'tcp6'.", netType)
	}
	// 创建 HTTP 客户端
	client := &http.Client{
		Timeout: 6 * time.Second,
		Transport: &http.Transport{
			DialContext: func(ctx context.Context, network string, addr string) (net.Conn, error) {
				return (&net.Dialer{}).DialContext(ctx, netType, addr)
			},
			TLSHandshakeTimeout:   3 * time.Second,
			ResponseHeaderTimeout: 3 * time.Second,
			ExpectContinueTimeout: 3 * time.Second,
		},
	}
	// 创建 HTTP 请求
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("Error creating request: %v", err)
	}
	// 如果启用请求头，则设置请求头信息
	if enableHeader {
		req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/115.0.0.0 Safari/537.36")
		req.Header.Set("Accept-Language", "zh-CN,zh;q=0.8,zh-TW;q=0.7,zh-HK;q=0.5,en-US;q=0.3,en;q=0.2")
		if additionalHeader != "" {
			tempList := strings.Split(additionalHeader, ":")
			if len(tempList) == 2 {
				req.Header.Set(tempList[0], tempList[1])
			} else if len(tempList) > 2 {
				req.Header.Set(tempList[0], strings.Join(tempList[1:], ":"))
			}
		}
	}
	// 执行 HTTP 请求
	resp, err := client.Do(req)
	if err != nil {
		//fmt.Printf("Error fetching %s info: %v \n", url, err)
		return nil, fmt.Errorf("Error fetching %s info: %v", url, err)
	}
	defer resp.Body.Close()
	// 解析 JSON 响应体
	var data map[string]interface{}
	err = json.NewDecoder(resp.Body).Decode(&data)
	if err != nil {
		//fmt.Printf("Error decoding %s info: %v \n", url, err)
		return nil, fmt.Errorf("Error decoding %s info: %v ", url, err)
	}
	// 返回解析后的数据和 nil 错误
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
