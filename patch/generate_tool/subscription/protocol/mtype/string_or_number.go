package mtype

import (
	"strconv"

	"github.com/sagernet/sing/common/json"
)

type StringOrNumber string

func (p *StringOrNumber) UnmarshalJSON(b []byte) error {
	if b[0] == '"' {
		var s string
		if err := json.Unmarshal(b, &s); err != nil {
			return err
		}
		*p = StringOrNumber(s)
	} else {
		var i int
		if err := json.Unmarshal(b, &i); err != nil {
			return err
		}
		*p = StringOrNumber(strconv.Itoa(i))
	}
	return nil
}
