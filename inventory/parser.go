package inventory

import "errors"

type parser = func(string) (*Data, error)

var parsers = [...]parser{parseYAML, parseIni}

func Parse(filename string) (*Data, error) {
	for _, parser := range parsers {
		data, err := parser(filename)
		if err == nil {
			return data, nil
		}
	}
	return nil, errors.New("unknown parse format")
}
