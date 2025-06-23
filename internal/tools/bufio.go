package tools

import "bufio"

func ReadLine(reader *bufio.Reader) ([]byte, error) {
	result := []byte{}
	for {
		line, isPrefix, err := reader.ReadLine()
		if err != nil {
			return nil, err
		}
		result = append(result, line...)
		if !isPrefix {
			return result, nil
		}
	}
}
