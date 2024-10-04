package xml

import (
	"encoding/xml"
)

type Slice[T any] []T

func (s *Slice[T]) UnmarshalXML(d *xml.Decoder, start xml.StartElement) error {
	for {
		tok, err := d.Token()
		if err != nil {
			return err
		}
		switch tok := tok.(type) {
		case xml.StartElement:
			// Start of a child
			var v T
			err = d.DecodeElement(&v, &tok)
			if err != nil {
				return err
			}
			*s = append(*s, v)

		case xml.EndElement:
			// Reached the end of the slice
			return nil

		default:
			// Ignore
		}
	}
}

type Dictionary = Slice[KeyValuePair]

type KeyValuePair struct {
	Name  string
	Value string
}

func (v *KeyValuePair) UnmarshalXML(d *xml.Decoder, start xml.StartElement) error {
	v.Name = start.Name.Local
	return d.DecodeElement(&v.Value, &start)
}
