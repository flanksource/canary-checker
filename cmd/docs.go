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
	"os"
	"reflect"
	"regexp"
	"sort"
	"strings"

	v1 "github.com/flanksource/canary-checker/api/v1"
	"github.com/flanksource/commons/logger"
	"github.com/spf13/cobra"
)

var Docs = &cobra.Command{
	Use:    "docs",
	Short:  "Generate docs ",
	Hidden: true,
	Args:   cobra.MinimumNArgs(0),
	Run: func(cmd *cobra.Command, args []string) {
		printAPIDocs(args)
	},
}

var out string

func init() {
	Docs.Flags().StringVar(&out, "output-file", "docs/API.md", "")
}

var (
	excludePatterns = []string{".*Result", ".*Check", "URL", "SrvReply", "Metric", "FolderFilterContext", "Templatable", "Test", "Description", "Condition", "TCP"}
	first           = []string{"Canary", "CanarySpec", "CanaryStatus", "CanaryList"}
	last            = []string{"Template", "Connection", "AWSConnection", "Bucket", "FolderFilter", "GCPConnection", "Authentication", "Display", "VarSource", "CloudWatchFilter"}

	links = map[string]string{
		"metav1.ObjectMeta":        "https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.20/#objectmeta-v1-meta",
		"metav1.ListMeta":          "https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.20/#listmeta-v1-meta",
		"metav1.LabelSelector":     "https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.20/#labelselector-v1-meta",
		"v1.ResourceRequirements":  "https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.20/#resourcerequirements-v1-core",
		"v1.LocalObjectReference":  "https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.20/#localobjectreference-v1-core",
		"v1.SecretKeySelector":     "https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.20/#secretkeyselector-v1-core",
		"v1.PersistentVolumeClaim": "https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.20/#persistentvolumeclaim-v1-core",
		"v1.EmptyDirVolumeSource":  "https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.20/#emptydirvolumesource-v1-core",
		"v1.PodSpec":               "https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.20/#podspec-v1-core",
		"kommons.EnvVarSource":     "https://pkg.go.dev/github.com/flanksource/kommons#EnvVarSource",
		"kommons.EnvVar":           "https://pkg.go.dev/github.com/flanksource/kommons#EnvVar",
	}

	selfLinks = map[string]string{}
)

func print(names []string, types map[string]*Struct) (map[string]*Struct, string) {
	s := ""
	for _, item := range names {
		if _, ok := types[item]; ok {
			s += types[item].Print(types) + "\n"
		}
		delete(types, item)
	}
	return types, s
}

func printAPIDocs(paths []string) {
	types := ParseDocumentationFrom(paths)
	for _, t := range types {
		strukt := t
		selfLinks[strukt.Name] = "#" + strings.ToLower(strukt.Name)
	}

	// we need to parse once more to now add the self links
	types = ParseDocumentationFrom(paths)
	types, s := print(first, types)
	s = `---
title: Canary Types
hide:
  - toc
---
` + s

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
		for _, i := range last {
			if i == name {
				continue TYPES
			}
		}
		s += strukt.Print(types) + "\n"
	}
	_, lastPart := print(last, types)
	s += lastPart
	if err := os.WriteFile(out, []byte(s), 0644); err != nil {
		logger.Errorf("error writing %s: %v", out, err)
	}
}

type Struct struct {
	Name, Doc  string
	StructType *ast.StructType
	DocType    *doc.Type
	Fields     StructFields
}

func (s StructFields) GetAllFields(structs map[string]*Struct) StructFields {
	var fields StructFields
	for _, f := range s {
		if f.Name == "-" {
			if _, ok := structs[f.ID]; !ok {
				continue
			}
			fields = append(fields, structs[f.ID].Fields.GetAllFields(structs)...)
		} else {
			fields = append(fields, f)
		}
	}
	sort.Sort(fields)
	return fields
}

type StructFields []StructField

func (s StructFields) Len() int {
	return len(s)
}

func (s StructFields) Swap(i, j int) {
	s[i], s[j] = s[j], s[i]
}

func (s StructFields) Less(i, j int) bool {
	return s[i].Name < s[j].Name
}

func (strukt Struct) Print(structs map[string]*Struct) string {
	icon := " "
	for _, check := range v1.AllChecks {
		name := reflect.TypeOf(check).Name()
		if strings.HasPrefix(name, strukt.Name) {
			icon = fmt.Sprintf(" <img src='https://raw.githubusercontent.com/flanksource/flanksource-ui/main/src/icons/%s.svg' style='height: 32px'/> ", check.GetType())
		}
	}
	for _, i := range first {
		if i == strukt.Name {
			break
		}
	}
	for _, i := range last {
		if i == strukt.Name {
			icon = " "
			break
		}
	}

	s := fmt.Sprintf("\n##%s%s\n\n%s\n\n", icon, strukt.Name, fmtCell(strukt.Doc))
	s += fmt.Sprintln("| Field | Description | Scheme | Required |")
	s += fmt.Sprintln("| ----- | ----------- | ------ | -------- |")

	for _, f := range strukt.Fields.GetAllFields(structs) {
		s += f.MarkdownRow(structs)
	}
	return s
}

type StructField struct {
	Name, Doc, Type, ID string
	Mandatory           bool
	Markdown            string
}

func (f StructField) MarkdownRow(structs map[string]*Struct) string {
	// if f.Name == "-" {
	// 	s := ""
	// 	sort.Sort(structs[f.ID].Fields)
	// 	for _, inner := range structs[f.ID].Fields {
	// 		s += inner.MarkdownRow(structs)
	// 	}
	// 	return s
	// }
	required := ""
	name := f.Name

	if f.Mandatory {
		required = "Yes"
		name = "**" + name + "**"
	}
	return fmt.Sprintln("|", name, "|", f.Doc, "|", f.Type, "|", required, "|")
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
	apkg, _ := ast.NewPackage(fset, m, nil, nil) //nolint:staticcheck

	return doc.New(apkg, "", 0)
}

func fmtCell(rawDoc string) string {
	// postDoc = strings.Replace(postDoc, "\\\"", "\"", -1) // replace user's \" to "
	// postDoc = strings.Replace(postDoc, "\"", "\\\"", -1) // Escape "
	postDoc := rawDoc
	// postDoc = strings.Replace(postDoc, "\n", "<br>", -1)
	// postDoc = strings.Replace(postDoc, "\t", "  ", -1)
	// postDoc = strings.Replace(postDoc, "|", "\\|", -1)
	return postDoc
}

func fmtRawDoc(rawDoc string) string {
	var buffer bytes.Buffer
	delPrevChar := func() {
		if buffer.Len() > 0 {
			buffer.Truncate(buffer.Len() - 1) // Delete the last " " or "\n"
		}
	}

	include := regexp.MustCompile(`\[include:(.*)\]`)
	for _, fixture := range include.FindAllStringSubmatch(rawDoc, -1) {
		content, err := os.ReadFile("fixtures/" + fixture[1])
		if err != nil {
			logger.Warnf("cannot find fixture: %s: %v", fixture[1], err)
		}
		example :=
			"??? example\n" +
				"		 ```yaml\n"

		for _, line := range strings.Split(string(content), "\n") {
			example +=
				"		 " + line + "\n"
		}
		example += "		 ```"

		rawDoc = strings.ReplaceAll(rawDoc, fixture[0], example)
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
	if strings.TrimSpace(text) == "" {
		return link
	}
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
		switch v := field.Type.(type) {
		case *ast.Ident:
			return v.Name
		case *ast.SelectorExpr:
			return v.Sel.Name
		}
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
		logger.Infof("%s is unknown", typ)
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
		return "\\[\\]" + toLink(fieldType(typ.(*ast.ArrayType).Elt)) // nolint: gosimple
	case *ast.MapType:
		mapType := typ.(*ast.MapType) // nolint: gosimple
		return "map[" + toLink(fieldType(mapType.Key)) + "]" + toLink(fieldType(mapType.Value))
	default:
		logger.Infof("%s is unknown", typ)
		return ""
	}
}
