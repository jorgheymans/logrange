// Copyright 2018-2019 The logrange Authors
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

package lql

import (
	"fmt"
	"github.com/logrange/logrange/pkg/model"
	"path"
	"strings"
)

type (
	// TagsExpFunc returns true if the provided tags are matched with the expression
	TagsExpFunc func(tags model.TagMap) bool

	tagsExpFuncBuilder struct {
		tef TagsExpFunc
	}
)

var positiveTagsExpFunc = func(model.TagMap) bool { return true }

func BuildTagsExpFunc(exp *Expression) (TagsExpFunc, error) {
	if exp == nil {
		return positiveTagsExpFunc, nil
	}

	var teb tagsExpFuncBuilder
	err := teb.buildOrConds(exp.Or)
	if err != nil {
		return nil, err
	}

	return teb.tef, nil
}

func (teb *tagsExpFuncBuilder) buildOrConds(ocn []*OrCondition) error {
	if len(ocn) == 0 {
		teb.tef = positiveTagsExpFunc
		return nil
	}

	err := teb.buildXConds(ocn[0].And)
	if err != nil {
		return err
	}

	if len(ocn) == 1 {
		// no need to go ahead anymore
		return nil
	}

	efd0 := teb.tef
	err = teb.buildOrConds(ocn[1:])
	if err != nil {
		return err
	}
	efd1 := teb.tef

	teb.tef = func(tags model.TagMap) bool { return efd0(tags) || efd1(tags) }
	return nil
}

func (teb *tagsExpFuncBuilder) buildXConds(cn []*XCondition) (err error) {
	if len(cn) == 0 {
		teb.tef = positiveTagsExpFunc
		return nil
	}

	if len(cn) == 1 {
		return teb.buildXCond(cn[0])
	}

	err = teb.buildXCond(cn[0])
	if err != nil {
		return err
	}

	efd0 := teb.tef
	err = teb.buildXConds(cn[1:])
	if err != nil {
		return err
	}
	efd1 := teb.tef

	teb.tef = func(tags model.TagMap) bool { return efd0(tags) && efd1(tags) }
	return nil

}

func (teb *tagsExpFuncBuilder) buildXCond(xc *XCondition) (err error) {
	if xc.Expr != nil {
		err = teb.buildOrConds(xc.Expr.Or)
	} else {
		err = teb.buildTagCond(xc.Cond)
	}

	if err != nil {
		return err
	}

	if xc.Not {
		efd1 := teb.tef
		teb.tef = func(tags model.TagMap) bool { return !efd1(tags) }
		return nil
	}

	return nil
}

func (teb *tagsExpFuncBuilder) buildTagCond(cn *Condition) (err error) {
	op := strings.ToUpper(cn.Op)
	switch op {
	case "<":
		teb.tef = func(tags model.TagMap) bool {
			return tags[cn.Operand] < cn.Value
		}
	case ">":
		teb.tef = func(tags model.TagMap) bool {
			return tags[cn.Operand] > cn.Value
		}
	case "<=":
		teb.tef = func(tags model.TagMap) bool {
			return tags[cn.Operand] <= cn.Value
		}
	case ">=":
		teb.tef = func(tags model.TagMap) bool {
			return tags[cn.Operand] >= cn.Value
		}
	case "!=":
		teb.tef = func(tags model.TagMap) bool {
			return tags[cn.Operand] != cn.Value
		}
	case "=":
		teb.tef = func(tags model.TagMap) bool {
			return tags[cn.Operand] == cn.Value
		}
	case CMP_LIKE:
		// test it first
		_, err := path.Match(cn.Value, "abc")
		if err != nil {
			err = fmt.Errorf("Wrong 'like' expression for %s, err=%s", cn.Value, err.Error())
		} else {
			teb.tef = func(tags model.TagMap) bool {
				if v, ok := tags[cn.Operand]; ok {
					res, _ := path.Match(cn.Value, v)
					return res
				}
				return false
			}
		}
	case CMP_CONTAINS:
		teb.tef = func(tags model.TagMap) bool {
			return strings.Contains(tags[cn.Operand], cn.Value)
		}
	case CMP_HAS_PREFIX:
		teb.tef = func(tags model.TagMap) bool {
			return strings.HasPrefix(tags[cn.Operand], cn.Value)
		}
	case CMP_HAS_SUFFIX:
		teb.tef = func(tags model.TagMap) bool {
			return strings.HasSuffix(tags[cn.Operand], cn.Value)
		}
	default:
		err = fmt.Errorf("Unsupported operation %s for '%s' tag ", cn.Op, cn.Operand)
	}
	return err
}
