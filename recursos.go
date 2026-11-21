// Package hz528 expone los recursos del harness embebidos en el binario `hz`:
// el DDL del motor y el texto vigente de las reglas generales.
// Viven en la raíz del repo (schema/, reglas/) para que humanos y binario lean la misma fuente.
package hz528

import _ "embed"

// SchemaSQL es el DDL del motor de memoria operativa (schema/harness.sql).
//
//go:embed schema/harness.sql
var SchemaSQL string

// ReglasGenerales es el texto vigente de las RG (reglas/generales.md).
//
//go:embed reglas/generales.md
var ReglasGenerales string
