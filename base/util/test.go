// Copyright 2020 Red Hat, Inc
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
// limitations under the License.)

package util

import (
	"reflect"
	"strings"
	"testing"

	confutil "github.com/coreos/butane/config/util"
	"github.com/coreos/butane/translate"

	"github.com/coreos/vcontext/path"
	"github.com/coreos/vcontext/report"
	"github.com/stretchr/testify/assert"
)

// helper functions for writing tests

// VerifyTranslations ensures all the translations are identity, unless they
// match a listed one, and verifies that all the listed ones exist.
func VerifyTranslations(t *testing.T, set translate.TranslationSet, exceptions []translate.Translation) {
	exceptionSet := translate.NewTranslationSet(set.FromTag, set.ToTag)
	for _, ex := range exceptions {
		exceptionSet.AddTranslation(ex.From, ex.To)
		if tr, ok := set.Set[ex.To.String()]; ok {
			assert.Equal(t, ex, tr, "non-identity translation with unexpected From")
		} else {
			t.Errorf("missing non-identity translation %v", ex)
		}
	}
	for key, translation := range set.Set {
		if _, ok := exceptionSet.Set[key]; !ok {
			assert.Equal(t, translation.From.Path, translation.To.Path, "translation is not identity")
		}
	}
}

// VerifyTranslatedReport translates report paths from camelCase json to
// snake_case yaml and then verifies that every path in a report corresponds
// to a valid field in the object.
func VerifyTranslatedReport(t *testing.T, obj interface{}, ts translate.TranslationSet, r report.Report) {
	// check for stray snake_case
	for _, entry := range r.Entries {
		if entry.Context.Tag == "yaml" {
			// expected to be in snake case
			continue
		}
		for _, component := range entry.Context.Path {
			str, ok := component.(string)
			if !ok {
				continue
			}
			if strings.Contains(str, "_") {
				t.Errorf("%s: translated report contains snake_case name", entry.Context)
			}
		}
	}

	r2 := confutil.TranslateReportPaths(r, ts)
	VerifyReport(t, obj, r2)
}

// VerifyReport verifies that every path in a report corresponds to a valid
// field in the object.
func VerifyReport(t *testing.T, obj interface{}, r report.Report) {
	v := reflect.ValueOf(obj)
	for _, entry := range r.Entries {
		verifyPath(t, v, entry.Context)
	}
}

func verifyPath(t *testing.T, v reflect.Value, p path.ContextPath) {
	if len(p.Path) == 0 {
		return
	}
	switch v.Kind() {
	case reflect.Map:
		value := v.MapIndex(reflect.ValueOf(p.Path[0]))
		if v.IsZero() {
			t.Errorf("%s: path component %q is nonexistent map key", p, p.Path[0])
			return
		}
		verifyPath(t, value, p.Tail())
	case reflect.Pointer:
		if !v.IsValid() || v.IsNil() {
			t.Errorf("%s: path component %q points through a nil pointer", p, p.Path[0])
			return
		}
		verifyPath(t, v.Elem(), p)
	case reflect.Slice:
		index, ok := p.Path[0].(int)
		if !ok {
			t.Errorf("%s: path component %q is not a valid slice index", p, p.Path[0])
			return
		}
		if index >= v.Len() {
			t.Errorf("%s: path index %d out of bounds for slice of length %d", p, index, v.Len())
			return
		}
		verifyPath(t, v.Index(index), p.Tail())
	case reflect.Struct:
		fieldName, ok := p.Path[0].(string)
		if !ok {
			t.Errorf("%s: path component %q is not a valid struct field name", p, p.Path[0])
			return
		}
		if !verifyStruct(t, v, p, fieldName) {
			t.Errorf("%s: path component %q refers to nonexistent field", p, p.Path[0])
		}
	default:
		t.Errorf("%s: path component %q points through kind %s", p, p.Path[0], v.Kind())
	}
}

func verifyStruct(t *testing.T, v reflect.Value, p path.ContextPath, fieldName string) bool {
	if v.Kind() != reflect.Struct {
		panic("verifyStruct called on non-struct")
	}
	for i := 0; i < v.NumField(); i++ {
		fieldType := v.Type().Field(i)
		if fieldType.Anonymous {
			if verifyStruct(t, v.Field(i), p, fieldName) {
				return true
			}
		} else {
			tag := strings.Split(fieldType.Tag.Get("yaml"), ",")[0]
			if tag == fieldName {
				verifyPath(t, v.Field(i), p.Tail())
				return true
			}
		}
	}
	// didn't find field
	return false
}
