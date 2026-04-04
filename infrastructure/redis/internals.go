package redisModel

import (
	common "github.com/Mapex-Solutions/mapexGoKit/infrastructure/common/ports"
	serialize "github.com/Mapex-Solutions/mapexGoKit/utils/serialize"
)

func (r *RedisClient) getSetMode(opts *common.SetOptions) string {
	if opts == nil {
		return ""
	}
	if opts.NX {
		return "NX"
	}
	if opts.XX {
		return "XX"
	}
	return ""
}

// prepareValue ensures the input value is converted to a string before storing.
// It handles string, []byte, and marshals any other types.
func (r *RedisClient) prepareValue(value interface{}) (string, error) {
	switch v := value.(type) {
	case string:
		return v, nil
	case []byte:
		return string(v), nil
	default:
		marshaled, err := serialize.Marshal(value)
		if err != nil {
			return "", err
		}
		return marshaled, nil
	}
}
