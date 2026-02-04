package utils

import (
	"encoding/json"
)

// ... 原有的 GenerateID 代码 ...

// 新增辅助函数：将任意对象转为 JSON 字节
func ToJSON(v interface{}) []byte {
	if v == nil {
		return []byte("[]") // 默认为空数组
	}
	bytes, _ := json.Marshal(v)
	return bytes
}
