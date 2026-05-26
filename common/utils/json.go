package utils

import "encoding/json"

func ToPrettyJsonForDebug(d any) string {
	dataStr, err := json.MarshalIndent(d, "", "  ")
	if err != nil {
		panic("json转换失败: " + err.Error())
	}
	return string(dataStr)
}
