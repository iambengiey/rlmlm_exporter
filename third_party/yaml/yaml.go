package yaml

import (
	"errors"
	"fmt"
	"reflect"
	"strconv"
	"strings"
)

// Unmarshal provides a tiny subset of YAML decoding sufficient for the exporter configuration files.
func Unmarshal(in []byte, out interface{}) error {
	if out == nil {
		return errors.New("yaml: nil output")
	}
	rv := reflect.ValueOf(out)
	if rv.Kind() != reflect.Ptr || rv.IsNil() {
		return errors.New("yaml: non-pointer passed to Unmarshal")
	}
	rv = rv.Elem()
	if rv.Kind() != reflect.Struct {
		return fmt.Errorf("yaml: unsupported type %s", rv.Kind())
	}
	rawLicenses, err := parseLicenses(string(in))
	if err != nil {
		return err
	}
	field := rv.FieldByName("Licenses")
	if !field.IsValid() || field.Kind() != reflect.Slice {
		return errors.New("yaml: struct missing Licenses slice")
	}
	elemType := field.Type().Elem()
	slice := reflect.MakeSlice(field.Type(), 0, len(rawLicenses))
	for _, raw := range rawLicenses {
		elem := reflect.New(elemType).Elem()
		if err := populateStruct(elem, raw); err != nil {
			return err
		}
		slice = reflect.Append(slice, elem)
	}
	field.Set(slice)
	return nil
}

type rawLicense map[string]string

func parseLicenses(data string) ([]rawLicense, error) {
	lines := strings.Split(data, "\n")
	licenses := []rawLicense{}
	var current rawLicense
	inLicenses := false
	for _, line := range lines {
		if idx := strings.Index(line, "#"); idx >= 0 {
			line = line[:idx]
		}
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		if !inLicenses {
			if trimmed == "licenses:" {
				inLicenses = true
				continue
			}
			return nil, errors.New("yaml: expected 'licenses:' root key")
		}
		if strings.HasPrefix(trimmed, "-") {
			if current != nil {
				licenses = append(licenses, current)
			}
			current = rawLicense{}
			trimmed = strings.TrimSpace(strings.TrimPrefix(trimmed, "-"))
			if trimmed == "" {
				continue
			}
			key, value, err := parseKeyValue(trimmed)
			if err != nil {
				return nil, err
			}
			current[key] = value
			continue
		}
		if current == nil {
			return nil, errors.New("yaml: encountered key/value outside of list item")
		}
		key, value, err := parseKeyValue(trimmed)
		if err != nil {
			return nil, err
		}
		current[key] = value
	}
	if current != nil {
		licenses = append(licenses, current)
	}
	return licenses, nil
}

func parseKeyValue(line string) (string, string, error) {
	parts := strings.SplitN(line, ":", 2)
	if len(parts) != 2 {
		return "", "", fmt.Errorf("yaml: unable to parse line %q", line)
	}
	key := strings.TrimSpace(parts[0])
	value := strings.TrimSpace(parts[1])
	value = strings.Trim(value, "\"'")
	return key, value, nil
}

func populateStruct(v reflect.Value, raw rawLicense) error {
	t := v.Type()
	for i := 0; i < t.NumField(); i++ {
		field := t.Field(i)
		yamlName := field.Tag.Get("yaml")
		yamlName = strings.Split(yamlName, ",")[0]
		if yamlName == "" {
			yamlName = strings.ToLower(field.Name)
		}
		rawValue, ok := raw[yamlName]
		if !ok {
			continue
		}
		fv := v.Field(i)
		if !fv.CanSet() {
			continue
		}
		switch fv.Kind() {
		case reflect.String:
			fv.SetString(rawValue)
		case reflect.Bool:
			b, err := strconv.ParseBool(strings.ToLower(rawValue))
			if err != nil {
				return fmt.Errorf("yaml: invalid boolean %q for field %s", rawValue, field.Name)
			}
			fv.SetBool(b)
		default:
			return fmt.Errorf("yaml: unsupported field type %s", fv.Kind())
		}
	}
	return nil
}
