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

type TweetType int64

const (
	TweetType_XSH TweetType = 1
)

func (p TweetType) String() string {
	switch p {
	case TweetType_XSH:
		return "Xsh"

	}
	return "<UNSET>"
}

func TweetTypeFromString(s string) (TweetType, error) {
	switch s {
	case "Xsh":
		return TweetType_XSH, nil

	}
	return TweetType(0), fmt.Errorf("not a valid TweetType string")
}

func TweetTypePtr(v TweetType) *TweetType { return &v }
func (p *TweetType) Scan(value interface{}) (err error) {
	var result sql.NullInt64
	err = result.Scan(value)
	*p = TweetType(result.Int64)
	return
}

func (p *TweetType) Value() (driver.Value, error) {
	if p == nil {
		return nil, nil
	}
	return int64(*p), nil
}
