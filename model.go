package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
)

// -------------  Entity ------------- //

func NewEntity(ID string) *Entity {
	e := Entity{}
	e.ID = ID
	e.Properties = make(map[string]any)
	e.References = make(map[string]any)
	return &e
}

type Entity struct {
	ID         string
	Recorded   uint64
	IsDeleted  bool
	References map[string]any
	Properties map[string]any
	token      string
}

func (anEntity *Entity) getReferenceValue(typeURI string) (string, error) {
	if values, found := anEntity.References[typeURI]; found {
		switch v := values.(type) {
		case []string:
			if len(v) > 0 {
				return v[0], nil
			}
		case string:
			return v, nil
		}
	}
	return "", errors.New("no reference for type")
}

func (anEntity *Entity) getStringLiteralPropertyValue(typeURI string) (string, error) {
	if values, found := anEntity.Properties[typeURI]; found {
		switch v := values.(type) {
		case []string:
			if len(v) > 0 {
				return v[0], nil
			}
		case string:
			return v, nil
		}
	}
	return "", errors.New("no property string literal")
}

func (anEntity *Entity) getBooleanLiteralPropertyValue(typeURI string) (bool, error) {
	if values, found := anEntity.Properties[typeURI]; found {
		switch v := values.(type) {
		case []bool:
			if len(v) > 0 {
				return v[0], nil
			}
		case bool:
			return v, nil
		}
	}
	return false, errors.New("no property boolean literal")
}

func (anEntity *Entity) getIntLiteralPropertyValue(typeURI string) (int, error) {
	if values, found := anEntity.Properties[typeURI]; found {
		switch v := values.(type) {
		case []float64:
			if len(v) > 0 {
				return int(v[0]), nil
			}
		case float64:
			return int(v), nil
		}
	}
	return 0, errors.New("no property int32 literal")
}

// -------------  Context ------------- //

func NewContext() *Context {
	context := &Context{}
	context.prefixToExpansionMappings = make(map[string]string)
	context.expansionToPrefixMappings = make(map[string]string)
	return context
}

type Context struct {
	prefixToExpansionMappings map[string]string
	expansionToPrefixMappings map[string]string
}

func (aContext *Context) GetNamespaceExpansionForPrefix(prefix string) (string, error) {
	if expansion, found := aContext.prefixToExpansionMappings[prefix]; found {
		return expansion, nil
	} else {
		return "", errors.New("no expansion for prefix: " + prefix)
	}
}

func (aContext *Context) GetPrefixForExpansion(expansion string) (string, error) {
	if prefix, found := aContext.expansionToPrefixMappings[expansion]; found {
		return prefix, nil
	} else {
		return "", errors.New("no expansion for prefix: " + expansion)
	}
}

func (aContext *Context) StorePrefixExpansionMapping(prefix string, expansion string) {
	aContext.prefixToExpansionMappings[prefix] = expansion
	aContext.expansionToPrefixMappings[expansion] = prefix
}

func (aContext *Context) GetFullURIFromCURIE(curie string) (string, error) {
	if strings.HasPrefix(curie, "https:") {
		return curie, nil
	} else if strings.HasPrefix(curie, "http:") {
		return curie, nil
	} else {
		if strings.Contains(curie, ":") {
			parts := strings.Split(curie, ":")
			expansion, _ := aContext.GetNamespaceExpansionForPrefix(parts[0])
			return expansion + parts[1], nil
		}
		return "", errors.New("not a full uri and not a curie")
	}
}

// Merge joins the other context to the man context
func (aContext *Context) Merge(other *Context) error {
	for k, m := range other.expansionToPrefixMappings {
		if v, ok := aContext.expansionToPrefixMappings[k]; ok {
			if v != m {
				return fmt.Errorf("%s is already mapped to another value %s", k, v)
			}
		} else {
			aContext.expansionToPrefixMappings[k] = m
		}
	}
	for k, m := range other.prefixToExpansionMappings {
		if v, ok := aContext.prefixToExpansionMappings[k]; ok {
			if v != m {
				return fmt.Errorf("%s is already mapped to another value %s", k, v)
			}
		} else {
			aContext.prefixToExpansionMappings[k] = m
		}
	}
	return nil
}

// -------------  Entity Collection ------------- //

func NewEntityCollection() *EntityCollection {
	ec := &EntityCollection{}
	ec.Entities = make([]*Entity, 0)
	ec.Context = NewContext()
	return ec
}

type EntityCollection struct {
	Context      *Context
	Entities     []*Entity
	Continuation *Continuation
}

// -------------  Continuation ------------- //

type Continuation struct {
	Token string
}

type Parser interface {
	Parse(reader io.Reader) (*EntityCollection, error)
	Reset()
}

type StreamParser struct {
	entityCollection *EntityCollection
}

func NewEntityParser() *EntityParser {
	esp := &EntityParser{}
	esp.entityCollection = NewEntityCollection()
	return esp
}

type EntityParser struct {
	entityCollection *EntityCollection
}

func (esp *EntityParser) Reset() {
	esp.entityCollection = NewEntityCollection()
}

var _ Parser = (*EntityParser)(nil)

func isCURIE(value string) (bool, string, string) {
	if isFullURI(value) {
		return false, "", ""
	} else {
		parts := strings.Split(value, ":")
		if len(parts) != 2 {
			return false, "", ""
		}
		return true, parts[0], parts[1]
	}
}

func isFullURI(value string) bool {
	if strings.HasPrefix(value, "https://") || strings.HasPrefix(value, "http://") {
		return true
	}
	return false
}

// Parse stream of json to produce an EntityCollection
func (esp *EntityParser) Parse(reader io.Reader) (*EntityCollection, error) {
	err := esp.parse(reader, func(entity *Entity) error {
		if entity.ID == "@continuation" {
			esp.entityCollection.Continuation = &Continuation{}
			esp.entityCollection.Continuation.Token = entity.Properties["token"].(string)
		} else {
			esp.entityCollection.Entities = append(esp.entityCollection.Entities, entity)
		}
		return nil
	}, func(ctx *Context) error {
		esp.entityCollection.Context = ctx
		return nil
	})
	return esp.entityCollection, err
}

// The value could be either a CURIE or a URI and the full URI is returned
func (esp *EntityParser) produceCanonicalURI(value string) (string, error) {
	if isFullURI(value) {
		return value, nil
	}
	isCURIE, prefix, postfix := isCURIE(value)
	if isCURIE {
		expansion, err := esp.entityCollection.Context.GetNamespaceExpansionForPrefix(prefix)
		if err != nil {
			return "", err
		}
		return expansion + postfix, nil
	} else {
		// lookup default namespace expansion
		// and append the original value
		expansion, err := esp.entityCollection.Context.GetNamespaceExpansionForPrefix("_")
		if err != nil {
			return "", err
		}
		return expansion + value, nil
	}
}

func (esp *EntityParser) parse(reader io.Reader, emitEntity func(*Entity) error, emitContext func(ctx *Context) error) error {

	decoder := json.NewDecoder(reader)

	// expect start of array
	t, err := decoder.Token()
	if err != nil {
		return fmt.Errorf("parsing error: Bad token at start of stream: %w", err)
	}

	if delim, ok := t.(json.Delim); !ok || delim != '[' {
		return errors.New("parsing error: Expected [ at start of document")
	}

	// decode context object
	context := make(map[string]any)
	err = decoder.Decode(&context)
	if err != nil {
		return fmt.Errorf("parsing error: Unable to decode context: %w", err)
	}

	if context["id"] == "@context" {
		ctx := NewContext()
		for k, v := range context["namespaces"].(map[string]any) {
			ctx.StorePrefixExpansionMapping(k, v.(string))
		}
		emitContext(ctx)
	} else {
		return errors.New("first entity in array must be a context")
	}

	for {
		t, err = decoder.Token()
		if err != nil {
			if err == io.EOF {
				break
			} else {
				return fmt.Errorf("parsing error: Unable to read next token: %w", err)
			}
		}

		switch v := t.(type) {
		case json.Delim:
			if v == '{' {
				e, err := esp.parseEntity(decoder)
				if err != nil {
					return fmt.Errorf("parsing error: Unable to parse entity: %w", err)
				}
				err = emitEntity(e)
				if err != nil {
					return err
				}
			} else if v == ']' {
				// done
				break
			}
		default:
			return errors.New("parsing error: unexpected value in entity array")
		}
	}

	return nil
}

func (esp *EntityParser) parseEntity(decoder *json.Decoder) (*Entity, error) {
	e := &Entity{}
	e.Properties = make(map[string]any)
	e.References = make(map[string]any)
	isContinuation := false
	for {
		t, err := decoder.Token()
		if err != nil {
			return nil, fmt.Errorf("unable to read token: %w", err)
		}

		switch v := t.(type) {
		case json.Delim:
			if v == '}' {
				return e, nil
			}
		case string:
			if v == "id" {
				val, err := decoder.Token()
				if err != nil {
					return nil, fmt.Errorf("unable to read token of id value: %w", err)
				}

				if val.(string) == "@continuation" {
					e.ID = "@continuation"
					isContinuation = true
				} else {
					id, err := esp.produceCanonicalURI(val.(string))
					if err != nil {
						return nil, err
					}
					e.ID = id
				}
			} else if v == "recorded" {
				val, err := decoder.Token()
				if err != nil {
					return nil, fmt.Errorf("unable to read token of recorded value: %w", err)
				}
				e.Recorded = uint64(val.(float64))

			} else if v == "deleted" {
				val, err := decoder.Token()
				if err != nil {
					return nil, fmt.Errorf("unable to read token of deleted value: %w", err)
				}
				e.IsDeleted = val.(bool)

			} else if v == "props" {
				e.Properties, err = esp.parseProperties(decoder)
				if err != nil {
					return nil, fmt.Errorf("unable to parse properties: %w", err)
				}
			} else if v == "refs" {
				e.References, err = esp.parseReferences(decoder)
				if err != nil {
					return nil, fmt.Errorf("unable to parse references %w", err)
				}
			} else if v == "token" {
				if !isContinuation {
					return nil, errors.New("token property found but not a continuation entity")
				}
				val, err := decoder.Token()
				if err != nil {
					return nil, fmt.Errorf("unable to read continuation token value: %w", err)
				}
				e.Properties = make(map[string]any)
				e.Properties["token"] = val
			} else {
				// log named property
				// read value
				_, err := decoder.Token()
				if err != nil {
					return nil, fmt.Errorf("unable to parse value of unknown key: %s %w", v, err)
				}
			}
		default:
			return nil, errors.New("unexpected value in entity")
		}
	}
}

func (esp *EntityParser) parseReferences(decoder *json.Decoder) (map[string]any, error) {
	refs := make(map[string]any)

	_, err := decoder.Token()
	if err != nil {
		return nil, fmt.Errorf("unable to read token of at start of references: %w", err)
	}

	for {
		t, err := decoder.Token()
		if err != nil {
			return nil, fmt.Errorf("unable to read token in parse references: %w", err)
		}

		switch v := t.(type) {
		case json.Delim:
			if v == '}' {
				return refs, nil
			}
		case string:
			val, err := esp.parseRefValue(decoder)
			if err != nil {
				return nil, fmt.Errorf("unable to parse value of reference key %s", v)
			}

			id, err := esp.produceCanonicalURI(v)
			if err != nil {
				return nil, err
			}
			refs[id] = val
		default:
			return nil, errors.New("unknown type")
		}
	}
}

func (esp *EntityParser) parseProperties(decoder *json.Decoder) (map[string]any, error) {
	props := make(map[string]any)

	_, err := decoder.Token()
	if err != nil {
		return nil, fmt.Errorf("unable to read token of at start of properties: %w ", err)
	}

	for {
		t, err := decoder.Token()
		if err != nil {
			return nil, fmt.Errorf("unable to read token in parse properties: %w", err)
		}

		switch v := t.(type) {
		case json.Delim:
			if v == '}' {
				return props, nil
			}
		case string:
			val, err := esp.parseValue(decoder)
			if err != nil {
				return nil, fmt.Errorf("unable to parse property value of key %s err: %w", v, err)
			}

			if val != nil {
				id, err := esp.produceCanonicalURI(v)
				if err != nil {
					return nil, err
				}
				props[id] = val
			}
		default:
			return nil, errors.New("unknown type")
		}
	}
}

func (esp *EntityParser) parseRefValue(decoder *json.Decoder) (any, error) {
	for {
		t, err := decoder.Token()
		if err != nil {
			return nil, fmt.Errorf("unable to read token in parse value: %w", err)
		}

		switch v := t.(type) {
		case json.Delim:
			if v == '[' {
				return esp.parseRefArray(decoder)
			}
		case string:
			id, err := esp.produceCanonicalURI(v)
			if err != nil {
				return nil, err
			}
			return id, nil
		default:
			return nil, errors.New("unknown token in parse ref value")
		}
	}
}

func (esp *EntityParser) parseRefArray(decoder *json.Decoder) ([]string, error) {
	array := make([]string, 0)
	for {
		t, err := decoder.Token()
		if err != nil {
			return nil, fmt.Errorf("unable to read token in parse ref array: %w", err)
		}

		switch v := t.(type) {
		case json.Delim:
			if v == ']' {
				return array, nil
			}
		case string:
			id, err := esp.produceCanonicalURI(v)
			if err != nil {
				return nil, err
			}
			array = append(array, id)
		default:
			return nil, errors.New("unknown type")
		}
	}
}

func (esp *EntityParser) parseArray(decoder *json.Decoder) ([]any, error) {
	array := make([]any, 0)
	for {
		t, err := decoder.Token()
		if err != nil {
			return nil, fmt.Errorf("unable to read token in parse array: %w", err)
		}

		switch v := t.(type) {
		case json.Delim:
			if v == '{' {
				r, err := esp.parseEntity(decoder)
				if err != nil {
					return nil, fmt.Errorf("unable to parse array: %w", err)
				}
				array = append(array, r)
			} else if v == ']' {
				return array, nil
			} else if v == '[' {
				r, err := esp.parseArray(decoder)
				if err != nil {
					return nil, fmt.Errorf("unable to parse array: %w", err)
				}
				array = append(array, r)
			}
		case string:
			array = append(array, v)
		case int:
			array = append(array, v)
		case float64:
			array = append(array, v)
		case bool:
			array = append(array, v)
		default:
			return nil, errors.New("unknown type")
		}
	}
}

func (esp *EntityParser) parseValue(decoder *json.Decoder) (any, error) {
	for {
		t, err := decoder.Token()
		if err != nil {
			return nil, fmt.Errorf("unable to read token in parse value: %w", err)
		}

		if t == nil {
			// there is a good chance that we got a null value, and we need to handle that
			return nil, nil
		}

		switch v := t.(type) {
		case json.Delim:
			if v == '{' {
				return esp.parseEntity(decoder)
			} else if v == '[' {
				return esp.parseArray(decoder)
			}
		case string:
			return v, nil
		case int:
			return v, nil
		case float64:
			return v, nil
		case bool:
			return v, nil
		default:
			return nil, errors.New("unknown token in parse value")
		}
	}
}
