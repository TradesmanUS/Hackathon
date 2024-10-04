package json

import (
	"bytes"
	"encoding/json"
	"fmt"

	"gopkg.in/yaml.v3"
)

func YAML2JSON(b []byte) ([]byte, error) {
	var node yaml.Node
	err := yaml.Unmarshal(b, &node)
	if err != nil {
		return nil, err
	}
	buf := new(bytes.Buffer)
	err = writeYAML(buf, &node)
	return buf.Bytes(), err
}

func JSON2YAML(b []byte) ([]byte, error) {
	node := new(yaml.Node)
	err := yaml.Unmarshal(b, node)
	if err != nil {
		return nil, err
	}

	setStyle(node, 0)
	if node.Kind == yaml.DocumentNode && len(node.Content) == 1 {
		node = node.Content[0]
	}

	buf := new(bytes.Buffer)
	enc := yaml.NewEncoder(buf)
	enc.SetIndent(2)
	err = enc.Encode(node)
	return buf.Bytes(), err
}

func writeYAML(w *bytes.Buffer, node *yaml.Node) error {
	switch node.Kind {
	case yaml.DocumentNode:
		if len(node.Content) != 1 {
			panic(fmt.Errorf("expected exactly one element in the document, got %d", len(node.Content)))
		}
		return writeYAML(w, node.Content[0])

	case yaml.SequenceNode:
		w.WriteByte('[')
		for i, v := range node.Content {
			err := writeYAML(w, v)
			if err != nil {
				return err
			}
			if i+1 == len(node.Content) {
				continue
			}
			w.WriteByte(',')
		}
		w.WriteByte(']')
		return nil

	case yaml.MappingNode:
		w.WriteByte('{')
		for i, v := range node.Content {
			err := writeYAML(w, v)
			if err != nil {
				return err
			}
			if i+1 == len(node.Content) {
				continue
			}
			if i%2 == 0 {
				w.WriteByte(':')
			} else {
				w.WriteByte(',')
			}
		}
		w.WriteByte('}')
		return nil

	case yaml.ScalarNode:
		var v any
		err := node.Decode(&v)
		if err != nil {
			return err
		}
		b, err := json.Marshal(v)
		if err != nil {
			return err
		}
		w.Write(b)
		return nil

	case yaml.AliasNode:
		return writeYAML(w, node.Alias)

	default:
		panic(fmt.Errorf("invalid YAML node kind %v", node.Kind))
	}
}

func setStyle(node *yaml.Node, style yaml.Style) {
	node.Style = style
	for _, node := range node.Content {
		setStyle(node, style)
	}
}
