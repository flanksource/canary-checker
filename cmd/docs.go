// Copyright 2016 The prometheus-operator Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package cmd

import (
	"bytes"
	"fmt"
	"go/ast"
	"go/doc"
	"go/parser"
	"go/token"
	"reflect"
	"regexp"
	"sort"
	"strings"

	"github.com/spf13/cobra"
)

var APIDocs = &cobra.Command{
	Use:   "api",
	Short: "Generate docs ",
	Args:  cobra.MinimumNArgs(0),
	Run: func(cmd *cobra.Command, args []string) {
		printAPIDocs(args)
	},
}

var (
	excludePatterns = []string{".*Result", ".*Check", "URL", "SrvReply", "Metric"}
	first           = []string{"Config"}

	links = map[string]string{
		"metav1.ObjectMeta":        "https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.11/#objectmeta-v1-meta",
		"metav1.ListMeta":          "https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.11/#listmeta-v1-meta",
		"metav1.LabelSelector":     "https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.11/#labelselector-v1-meta",
		"v1.ResourceRequirements":  "https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.11/#resourcerequirements-v1-core",
		"v1.LocalObjectReference":  "https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.11/#localobjectreference-v1-core",
		"v1.SecretKeySelector":     "https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.11/#secretkeyselector-v1-core",
		"v1.PersistentVolumeClaim": "https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.11/#persistentvolumeclaim-v1-core",
		"v1.EmptyDirVolumeSource":  "https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.11/#emptydirvolumesource-v1-core",
	}

	selfLinks = map[string]string{}
)

func printAPIDocs(paths []string) {
	types := ParseDocumentationFrom(paths)
	for _, t := range types {
		strukt := t
		selfLinks[strukt.Name] = "#" + strings.ToLower(strukt.Name)
	}

	// we need to parse once more to now add the self links
	types = ParseDocumentationFrom(paths)

	for _, item := range first {
		if _, ok := types[item]; ok {
			fmt.Println(types[item].String())
		}
		delete(types, item)
	}

	alphabetical := []string{}

	for name := range types {
		alphabetical = append(alphabetical, name)
	}
	sort.Strings(alphabetical)

TYPES:
	for _, name := range alphabetical {
		strukt := types[name]
		for _, exclude := range excludePatterns {
			if regexp.MustCompile(exclude).MatchString(strukt.Name) {
				continue TYPES
			}
		}
		fmt.Println(strukt.String())
	}
}

type Struct struct {
	Name, Doc  string
	StructType *ast.StructType
	DocType    *doc.Type
	Fields     []StructField
}

func (strukt Struct) String() string {
	s := fmt.Sprintf("\n## %s\n\n%s\n\n", strukt.Name, strukt.Doc)
	s += fmt.Sprintln("| Field | Description | Scheme | Required |")
	s += fmt.Sprintln("| ----- | ----------- | ------ | -------- |")
	for _, f := range strukt.Fields {
		if f.Name == "-" {
			continue
		}
		required := ""
		if f.Mandatory {
			required = "Yes"
		}
		s += fmt.Sprintln("|", f.Name, "|", f.Doc, "|", f.Type, "|", required, "|")
	}
	return s
}

type StructField struct {
	Name, Doc, Type, ID string
	Mandatory           bool
}

func NewStructField(field *ast.Field) StructField {
	return StructField{
		ID:        fieldID(field.Type),
		Mandatory: fieldRequired(field),
		Name:      fieldName(field),
		Type:      fieldType(field.Type),
		Doc:       fmtRawDoc(field.Doc.Text()),
	}
}

// ParseDocumentationFrom gets all types' documentation and returns them as an
// array. Each type is again represented as an array (we have to use arrays as we
// need to be sure for the order of the fields). This function returns fields and
// struct definitions that have no documentation as {name, ""}.
func ParseDocumentationFrom(srcs []string) map[string]*Struct {
	structs := make(map[string]*Struct)

	for _, src := range srcs {
		pkg := astFrom(src)

		for _, kubType := range pkg.Types {
			if structType, ok := kubType.Decl.Specs[0].(*ast.TypeSpec).Type.(*ast.StructType); ok {
				_struct := &Struct{
					Name: kubType.Name,
					Doc:  fmtRawDoc(kubType.Doc),
				}
				for _, field := range structType.Fields.List {
					_struct.Fields = append(_struct.Fields, NewStructField(field))
				}
				structs[_struct.Name] = _struct
			}
		}
	}

	// check for inline fields (yaml:"-") and move their fields up
	for _, strukt := range structs {
		if len(strukt.Fields) == 1 && strukt.Fields[0].Name == "-" {
			if inline, ok := structs[strukt.Fields[0].ID]; ok {
				strukt.Fields = inline.Fields
			} else {
				fmt.Printf("Cannot find inline field: %s\n", strukt.Fields[0].ID)
			}
		}
	}
	return structs
}

func astFrom(filePath string) *doc.Package {
	fset := token.NewFileSet()
	m := make(map[string]*ast.File)

	f, err := parser.ParseFile(fset, filePath, nil, parser.ParseComments)
	if err != nil {
		fmt.Println(err)
		return nil
	}

	m[filePath] = f
	apkg, _ := ast.NewPackage(fset, m, nil, nil)

	return doc.New(apkg, "", 0)
}

func fmtRawDoc(rawDoc string) string {
	var buffer bytes.Buffer
	delPrevChar := func() {
		if buffer.Len() > 0 {
			buffer.Truncate(buffer.Len() - 1) // Delete the last " " or "\n"
		}
	}

	// Ignore all lines after ---
	rawDoc = strings.Split(rawDoc, "---")[0]

	for _, line := range strings.Split(rawDoc, "\n") {
		// line = strings.TrimRight(line, " ")
		leading := strings.TrimLeft(line, " ")
		switch {
		case len(line) == 0: // Keep paragraphs
			delPrevChar()
			buffer.WriteString("\n\n")
		case strings.HasPrefix(leading, "TODO"): // Ignore one line TODOs
		case strings.HasPrefix(leading, "+"): // Ignore instructions to go2idl
		default:
			if strings.HasPrefix(line, " ") || strings.HasPrefix(line, "\t") {
				delPrevChar()
				line = "\n" + line + "\n" // Replace it with newline. This is useful when we have a line with: "Example:\n\tJSON-someting..."
			} else {
				line += "\n"
			}
			buffer.WriteString(line)
		}
	}

	postDoc := strings.TrimRight(buffer.String(), "\n")
	// postDoc = strings.Replace(postDoc, "\\\"", "\"", -1) // replace user's \" to "
	// postDoc = strings.Replace(postDoc, "\"", "\\\"", -1) // Escape "
	// postDoc = strings.Replace(postDoc, "\n", "\\n", -1)
	postDoc = strings.Replace(postDoc, "\t", "  ", -1)
	// postDoc = strings.Replace(postDoc, "|", "\\|", -1)

	return postDoc
}

func toLink(typeName string) string {
	selfLink, hasSelfLink := selfLinks[typeName]
	if hasSelfLink {
		return wrapInLink(typeName, selfLink)
	}

	link, hasLink := links[typeName]
	if hasLink {
		return wrapInLink(typeName, link)
	}

	return typeName
}

func wrapInLink(text, link string) string {
	return fmt.Sprintf("[%s](%s)", text, link)
}

// fieldName returns the name of the field as it should appear in JSON format
// "-" indicates that this field is not part of the JSON representation
func fieldName(field *ast.Field) string {
	jsonTag := ""
	if field.Tag != nil {
		jsonTag = reflect.StructTag(field.Tag.Value[1 : len(field.Tag.Value)-1]).Get("yaml") // Delete first and last quotation
		if strings.Contains(jsonTag, "inline") {
			return "-"
		}
	}

	jsonTag = strings.Split(jsonTag, ",")[0] // This can return "-"
	if jsonTag == "" {
		if field.Names != nil {
			return field.Names[0].Name
		}
		return field.Type.(*ast.Ident).Name
	}
	return jsonTag
}

// fieldRequired returns whether a field is a required field.
func fieldRequired(field *ast.Field) bool {
	jsonTag := ""
	if field.Tag != nil {
		jsonTag = reflect.StructTag(field.Tag.Value[1 : len(field.Tag.Value)-1]).Get("yaml") // Delete first and last quotation
		return !strings.Contains(jsonTag, "omitempty")
	}

	return false
}

// nolint: gosimple
func fieldID(typ ast.Expr) string {
	switch typ.(type) {
	case *ast.Ident:
		return typ.(*ast.Ident).Name
	case *ast.StarExpr:
		return fieldType(typ.(*ast.StarExpr).X)
	case *ast.SelectorExpr:
		e := typ.(*ast.SelectorExpr)
		pkg := e.X.(*ast.Ident)
		t := e.Sel
		return pkg.Name + "." + t.Name
	case *ast.ArrayType:
		return "[]" + fieldType(typ.(*ast.ArrayType).Elt)
	case *ast.MapType:
		mapType := typ.(*ast.MapType)
		return "map[" + fieldType(mapType.Key) + "]" + fieldType(mapType.Value)
	default:
		return ""
	}
}
func fieldType(typ ast.Expr) string {
	switch typ.(type) { // nolint: gosimple
	case *ast.Ident:
		return toLink(typ.(*ast.Ident).Name) // nolint: gosimple
	case *ast.StarExpr:
		return "*" + toLink(fieldType(typ.(*ast.StarExpr).X)) // nolint: gosimple
	case *ast.SelectorExpr:
		e := typ.(*ast.SelectorExpr)
		pkg := e.X.(*ast.Ident)
		t := e.Sel
		return toLink(pkg.Name + "." + t.Name)
	case *ast.ArrayType:
		return "[]" + toLink(fieldType(typ.(*ast.ArrayType).Elt)) // nolint: gosimple
	case *ast.MapType:
		mapType := typ.(*ast.MapType) // nolint: gosimple
		return "map[" + toLink(fieldType(mapType.Key)) + "]" + toLink(fieldType(mapType.Value))
	default:
		return ""
	}
}
