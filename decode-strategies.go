package api

import (
	"encoding/json"
	"io"

	"github.com/goccy/go-yaml"
)

type DecodeStrategy func(data io.ReadCloser, v interface{}) error

var (
	NoDecodeStrategy DecodeStrategy = func(data io.ReadCloser, v interface{}) error {
		return nil
	}

	JSONDecodeStrategy DecodeStrategy = func(data io.ReadCloser, v interface{}) error {
		return json.NewDecoder(data).Decode(v)
	}

	YamlDecodeStrategy DecodeStrategy = func(data io.ReadCloser, v interface{}) error {
		return yaml.NewDecoder(data).Decode(v)
	}

	ByteDecodeStrategy DecodeStrategy = func(data io.ReadCloser, v interface{}) error {
		bytes, err := io.ReadAll(data)
		if err != nil {
			return err
		}
		ptr, ok := v.(*[]byte)
		if !ok {
			return nil
		}
		*ptr = bytes
		return nil
	}
)
