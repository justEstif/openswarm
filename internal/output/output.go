// Package output provides uniform output formatting for all swarm commands.
//
// Every command handler produces output through this package — never by calling
// fmt.Print directly. This ensures the --json contract is honoured consistently
// across the entire CLI surface.
//
// Rules:
//   - JSON always goes to stdout (including errors in JSON mode).
//   - Errors in human mode go to stderr.
//   - Print on a nil value is a no-op.
package output

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"reflect"
	"strings"
	"text/tabwriter"
)

// ─── Error types ────────────────────────────────────────────────────────────

// SwarmError is the structured error type used across all commands.
// Callers should test with errors.As(err, &SwarmError{}) rather than a type
// assertion, so wrapping survives in the future.
type SwarmError struct {
	Code    string `json:"code"`    // NOT_FOUND | CONFLICT | VALIDATION | IO_ERROR | LOCKED
	Message string `json:"message"`
}

func (e *SwarmError) Error() string {
	return fmt.Sprintf("%s: %s", e.Code, e.Message)
}

// Convenience constructors — prefer these over creating SwarmError literals.

func ErrNotFound(msg string) *SwarmError  { return &SwarmError{Code: "NOT_FOUND", Message: msg} }
func ErrConflict(msg string) *SwarmError  { return &SwarmError{Code: "CONFLICT", Message: msg} }
func ErrValidation(msg string) *SwarmError { return &SwarmError{Code: "VALIDATION", Message: msg} }
func ErrIO(msg string) *SwarmError        { return &SwarmError{Code: "IO_ERROR", Message: msg} }
func ErrLocked(msg string) *SwarmError    { return &SwarmError{Code: "LOCKED", Message: msg} }

// ─── Output functions ────────────────────────────────────────────────────────

// Print renders v as JSON (if asJSON) or as human-readable text.
//
// v should be a struct or a slice of structs with json tags.
//
// JSON output: json.MarshalIndent with 2-space indent, written to stdout.
//
// Human output:
//   - Slice: a tabwriter table whose header row is derived from the json tags.
//   - Struct: key: value pairs, one per line, keys taken from json tags.
//
// Print on a nil v is a no-op.
func Print(v any, asJSON bool) error {
	if v == nil {
		return nil
	}

	// Dereference top-level pointers so the nil-pointer case also short-circuits.
	rv := reflect.ValueOf(v)
	if rv.Kind() == reflect.Ptr {
		if rv.IsNil() {
			return nil
		}
		rv = rv.Elem()
	}

	if asJSON {
		return printJSON(os.Stdout, v)
	}
	return printHuman(os.Stdout, rv)
}

// PrintError renders err as human-readable text (to stderr) or as structured
// JSON (to stdout).
//
// JSON form: {"error": {"code": "NOT_FOUND", "message": "..."}}
//
// If err is not a *SwarmError it is wrapped as ErrIO(err.Error()).
func PrintError(err error, asJSON bool) {
	if err == nil {
		return
	}

	var se *SwarmError
	var ok bool
	if se, ok = err.(*SwarmError); !ok {
		se = ErrIO(err.Error())
	}

	if asJSON {
		wrapper := struct {
			Error *SwarmError `json:"error"`
		}{Error: se}
		_ = printJSON(os.Stdout, wrapper)
		return
	}

	fmt.Fprintf(os.Stderr, "error [%s]: %s\n", se.Code, se.Message)
}

// ─── internal helpers ────────────────────────────────────────────────────────

func printJSON(w io.Writer, v any) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(v)
}

// printHuman dispatches to table or field-dump based on whether rv is a slice.
func printHuman(w io.Writer, rv reflect.Value) error {
	switch rv.Kind() {
	case reflect.Slice:
		return printTable(w, rv)
	default:
		return printFields(w, rv)
	}
}

// jsonTag extracts the first segment of a struct field's json tag (the name),
// falling back to the field name if no tag is present.
func jsonTag(f reflect.StructField) string {
	tag := f.Tag.Get("json")
	if tag == "" || tag == "-" {
		return f.Name
	}
	name, _, _ := strings.Cut(tag, ",")
	if name == "" || name == "-" {
		return f.Name
	}
	return name
}

// visibleFields returns the exported fields of a struct type, skipping fields
// whose json tag is "-".
func visibleFields(t reflect.Type) []reflect.StructField {
	var fields []reflect.StructField
	for i := range t.NumField() {
		f := t.Field(i)
		if !f.IsExported() {
			continue
		}
		tag := f.Tag.Get("json")
		if tag == "-" {
			continue
		}
		// Skip the field if json tag is "-,"
		name, _, _ := strings.Cut(tag, ",")
		if name == "-" {
			continue
		}
		fields = append(fields, f)
	}
	return fields
}

// printTable renders a slice of structs as a tab-separated table with headers.
// Non-struct element types fall back to one-value-per-line.
func printTable(w io.Writer, rv reflect.Value) error {
	if rv.Len() == 0 {
		fmt.Fprintln(w, "(none)")
		return nil
	}

	// Determine element type (dereference pointer elements).
	elemType := rv.Type().Elem()
	for elemType.Kind() == reflect.Ptr {
		elemType = elemType.Elem()
	}

	if elemType.Kind() != reflect.Struct {
		// Fallback: print each element on its own line.
		for i := range rv.Len() {
			fmt.Fprintln(w, rv.Index(i).Interface())
		}
		return nil
	}

	fields := visibleFields(elemType)
	tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)

	// Header row.
	headers := make([]string, len(fields))
	for i, f := range fields {
		headers[i] = strings.ToUpper(jsonTag(f))
	}
	fmt.Fprintln(tw, strings.Join(headers, "\t"))

	// Data rows.
	for i := range rv.Len() {
		elem := rv.Index(i)
		for elem.Kind() == reflect.Ptr {
			elem = elem.Elem()
		}
		row := make([]string, len(fields))
		for j, f := range fields {
			row[j] = fmt.Sprintf("%v", elem.FieldByIndex(f.Index).Interface())
		}
		fmt.Fprintln(tw, strings.Join(row, "\t"))
	}

	return tw.Flush()
}

// printFields renders a single struct as "key: value" lines.
func printFields(w io.Writer, rv reflect.Value) error {
	// Dereference pointer.
	for rv.Kind() == reflect.Ptr {
		rv = rv.Elem()
	}

	if rv.Kind() != reflect.Struct {
		// Scalar or other: just print it.
		fmt.Fprintln(w, rv.Interface())
		return nil
	}

	fields := visibleFields(rv.Type())
	tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
	for _, f := range fields {
		key := jsonTag(f)
		val := fmt.Sprintf("%v", rv.FieldByIndex(f.Index).Interface())
		fmt.Fprintf(tw, "%s:\t%s\n", key, val)
	}
	return tw.Flush()
}
