package config

import _ "embed"

//go:embed schema.json
var schemaJSON []byte

// Schema returns the JSON Schema (draft 2020-12) describing probaci.json. It is
// written to the user home so editors can offer validation/autocomplete.
func Schema() []byte { return schemaJSON }
