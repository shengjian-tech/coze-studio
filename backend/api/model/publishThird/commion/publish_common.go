/*
 * Copyright 2025 coze-dev Authors
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package publishThird_commion

import (
	"database/sql"
	"database/sql/driver"
	"fmt"
)

type PublishThirdType int64

const (
	PublishThirdType_XSH PublishThirdType = 1
)

func (p PublishThirdType) String() string {
	switch p {
	case PublishThirdType_XSH:
		return "Xsh"

	}
	return "<UNSET>"
}

func PublishThirdTypeFromString(s string) (PublishThirdType, error) {
	switch s {
	case "Xsh":
		return PublishThirdType_XSH, nil

	}
	return PublishThirdType(0), fmt.Errorf("not a valid PublishThirdType string")
}

func PublishThirdTypePtr(v PublishThirdType) *PublishThirdType { return &v }
func (p *PublishThirdType) Scan(value interface{}) (err error) {
	var result sql.NullInt64
	err = result.Scan(value)
	*p = PublishThirdType(result.Int64)
	return
}

func (p *PublishThirdType) Value() (driver.Value, error) {
	if p == nil {
		return nil, nil
	}
	return int64(*p), nil
}
