package xml

import (
	"encoding/xml"
	"fmt"
	"log/slog"
	"strconv"
	"strings"

	"github.com/C3Rules/Go-DTRules/pkg/dt"
	"github.com/C3Rules/Go-DTRules/pkg/vm"
)

func (m *Map) LoadXml(s vm.State, edd map[string]dt.EntityDefinition, decoder *xml.Decoder) error {
	d := new(mapDecoder)
	d.xmap = m
	d.edd = edd
	d.decoder = decoder
	d.entities = map[string]map[int]*dt.Entity{}
	d.create = map[string]*XmlCreateEntity{}

	for _, ce := range m.Map.CreateEntity {
		d.create[ce.Tag] = ce
	}

	err := d.decodeElement(nil)
	if err != nil {
		return err
	}

outer:
	for _, ie := range m.InitEntities {
		if !ie.EPush {
			continue
		}
		for _, e := range d.entities[ie.Entity] {
			err := s.Entity().Push(e)
			if err != nil {
				return err
			}
			continue outer
		}
		return fmt.Errorf("there is no %s entity", ie.Entity)
	}
	return nil
}

type mapDecoder struct {
	xmap     *Map
	edd      map[string]dt.EntityDefinition
	decoder  *xml.Decoder
	stack    []*decodeScope
	entities map[string]map[int]*dt.Entity
	create   map[string]*XmlCreateEntity
}

type decodeScope struct {
	id     int
	entity *dt.Entity
	start  *xml.StartElement
}

func (d *mapDecoder) decodeElement(start *xml.StartElement) error {
	if start == nil {
		for {
			t, err := d.decoder.Token()
			if err != nil {
				return err
			}
			if t, ok := t.(xml.StartElement); ok {
				start = &t
				break
			}
		}
	}

	// Is it an entity?
	newEntity, err := d.decodeEntity(start)
	if err != nil {
		return err
	}

	if len(d.stack) == 0 {
		// Is it a root entity?
		if newEntity != nil {
			return nil
		}

		// Decode children
		return d.decodeChildren(start)
	}

	var attr *XmlSetAttr
	scope := d.stack[len(d.stack)-1]
	for _, sa := range d.xmap.Map.SetAttr {
		if sa.Tag != start.Name.Local {
			continue
		}
		if sa.Enclosure != scope.entity.EntityName() {
			continue
		}
		attr = sa
		break
	}
	if attr == nil {
		// Log but otherwise ignore
		slog.Warn("Unknown attribute", "name", start.Name.Local, "scope", scope.entity.EntityName(), "id", scope.id)
		if newEntity != nil {
			return nil
		}
		return d.skipElement(start)
	}

	if newEntity != nil {
		// If attr.Type != "entity", this will fail but that's ok
		return scope.entity.Set(attr.RAttr, newEntity)
	}

	var v any
	switch strings.ToLower(attr.Type) {
	case "integer":
		v, err = decodeXmlValue[int](d.decoder, start)
	case "float":
		v, err = decodeXmlValue[float64](d.decoder, start)
	case "string":
		v, err = decodeXmlValue[string](d.decoder, start)
	case "boolean":
		v, err = decodeXmlValue[bool](d.decoder, start)
	case "entity":
		// We reach here iff newEntity == nil, and newEntity == nil iff there's
		// no corresponding createentity
		return fmt.Errorf("unknown entity %s", start.Name.Local)
	}
	if err != nil {
		return err
	}

	return scope.entity.Set(attr.RAttr, v)
}

func (d *mapDecoder) decodeEntity(start *xml.StartElement) (*dt.Entity, error) {
	ce, ok := d.create[start.Name.Local]
	if !ok {
		return nil, nil
	}

	if _, ok := d.entities[ce.Entity]; !ok {
		d.entities[ce.Entity] = map[int]*dt.Entity{}
	}

	// Parse the ID
	idStr, ok := getAttr(start, ce.ID)
	if !ok {
		return nil, fmt.Errorf("missing id attribute (%s)", ce.ID)
	}
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		return nil, fmt.Errorf("invalid ID: %w", err)
	}

	// Check for an existing entity
	e, ok := d.entities[ce.Entity][int(id)]
	if !ok {
		// Create the entity
		ed, ok := d.edd[ce.Entity]
		if !ok {
			return nil, fmt.Errorf("unknown entity %s", ce.Entity)
		}
		e = ed.New(ce.Entity)

		// Register
		d.entities[ce.Entity][int(id)] = e
	}

	// Push onto the stack
	d.stack = append(d.stack, &decodeScope{
		id:     int(id),
		entity: e,
		start:  start,
	})

	// Decode children
	err = d.decodeChildren(start)
	if err != nil {
		return nil, fmt.Errorf("decoding %s: %w", start.Name.Local, err)
	}

	// Pop from the stack
	d.stack = d.stack[:len(d.stack)-1]
	return e, nil
}

func (d *mapDecoder) decodeChildren(start *xml.StartElement) error {
	for {
		t, err := d.decoder.Token()
		if err != nil {
			return err
		}

		switch t := t.(type) {
		case xml.StartElement:
			err = d.decodeElement(&t)

		case xml.EndElement:
			if t.Name.Local != start.Name.Local {
				panic(fmt.Errorf("invalid end element: want %s, got %s", start.Name.Local, t.Name.Local))
			}
			return nil
		}
		if err != nil {
			return err
		}
	}
}

func (d *mapDecoder) skipElement(start *xml.StartElement) error {
	for {
		t, err := d.decoder.Token()
		if err != nil {
			return err
		}

		switch t := t.(type) {
		case xml.StartElement:
			err := d.decodeElement(&t)
			if err != nil {
				return err
			}

		case xml.EndElement:
			if t.Name.Local != start.Name.Local {
				panic(fmt.Errorf("invalid end element: want %s, got %s", start.Name.Local, t.Name.Local))
			}
			return nil
		}
	}
}

func decodeXmlValue[T any](decoder *xml.Decoder, elem *xml.StartElement) (T, error) {
	var v T
	err := decoder.DecodeElement(&v, elem)
	return v, err
}

func getAttr(elem *xml.StartElement, name string) (string, bool) {
	for _, a := range elem.Attr {
		if a.Name.Local == name {
			return a.Value, true
		}
	}
	return "", false
}

type Map struct {
	Map          *XmlMap               `xml:"XMLtoEDD>map"`
	Entities     Slice[*MapEntity]     `xml:"XMLtoEDD>entities"`
	InitEntities Slice[*MapInitEntity] `xml:"XMLtoEDD>initialization"`
}

type XmlMap struct {
	SetAttr      []*XmlSetAttr      `xml:"setattribute"`
	CreateEntity []*XmlCreateEntity `xml:"createentity"`
}

type XmlSetAttr struct {
	Tag       string `xml:"tag,attr"`
	RAttr     string `xml:"RAttribute,attr"`
	Enclosure string `xml:"enclosure,attr"`
	Type      string `xml:"type,attr"`
}

type XmlCreateEntity struct {
	Entity string `xml:"entity,attr"`
	Tag    string `xml:"tag,attr"`
	ID     string `xml:"id,attr"`
}

type MapEntity struct {
	Name   string `xml:"name,attr"`
	Number string `xml:"number,attr"`
}

type MapInitEntity struct {
	Entity string `xml:"entity,attr"`
	EPush  bool   `xml:"epush,attr"`
}
