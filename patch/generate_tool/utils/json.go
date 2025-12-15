package utils

import "github.com/sagernet/sing/common/json"

func MarshalArrayF(v any) string {
	result, _ := json.Marshal(v)
	return string(result)
}
