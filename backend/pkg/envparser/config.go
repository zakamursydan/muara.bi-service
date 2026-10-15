package envparser

import (
	"bufio"
	"fmt"
	"os"
	"reflect"
	"strconv"
	"strings"
)

// LoadEnv reads .env file if it exists and populates target struct based on `env` and `default` tags.
func LoadEnv(target any) error {
	// 1. Try loading .env file into os.Environ
	loadDotEnvFile(".env")

	// 2. Parse struct fields recursively
	val := reflect.ValueOf(target)
	if val.Kind() != reflect.Pointer || val.Elem().Kind() != reflect.Struct {
		return fmt.Errorf("target must be a pointer to a struct")
	}

	return parseStruct(val.Elem())
}

func parseStruct(v reflect.Value) error {
	t := v.Type()

	for i := 0; i < v.NumField(); i++ {
		fieldVal := v.Field(i)
		fieldType := t.Field(i)

		// Recursive for nested structs
		if fieldVal.Kind() == reflect.Struct {
			if err := parseStruct(fieldVal); err != nil {
				return err
			}
			continue
		}

		envKey := fieldType.Tag.Get("env")
		defaultVal := fieldType.Tag.Get("default")

		rawVal := ""
		if envKey != "" {
			rawVal = os.Getenv(envKey)
		}
		if rawVal == "" && defaultVal != "" {
			rawVal = defaultVal
		}

		if rawVal != "" && fieldVal.CanSet() {
			if err := setFieldValue(fieldVal, rawVal); err != nil {
				return fmt.Errorf("failed to set field %s (%s): %w", fieldType.Name, envKey, err)
			}
		}
	}

	return nil
}

func setFieldValue(val reflect.Value, raw string) error {
	switch val.Kind() {
	case reflect.String:
		val.SetString(raw)
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		intVal, err := strconv.ParseInt(raw, 10, 64)
		if err != nil {
			return err
		}
		val.SetInt(intVal)
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		uintVal, err := strconv.ParseUint(raw, 10, 64)
		if err != nil {
			return err
		}
		val.SetUint(uintVal)
	case reflect.Float32, reflect.Float64:
		floatVal, err := strconv.ParseFloat(raw, 64)
		if err != nil {
			return err
		}
		val.SetFloat(floatVal)
	case reflect.Bool:
		boolVal, err := strconv.ParseBool(raw)
		if err != nil {
			return err
		}
		val.SetBool(boolVal)
	default:
		return fmt.Errorf("unsupported field type: %s", val.Kind().String())
	}
	return nil
}

func loadDotEnvFile(filename string) {
	file, err := os.Open(filename)
	if err != nil {
		return // Ignore if .env doesn't exist
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		parts := strings.SplitN(line, "=", 2)
		if len(parts) == 2 {
			k := strings.TrimSpace(parts[0])
			v := strings.TrimSpace(parts[1])
			v = strings.Trim(v, `"'`)
			if os.Getenv(k) == "" {
				os.Setenv(k, v)
			}
		}
	}
}
