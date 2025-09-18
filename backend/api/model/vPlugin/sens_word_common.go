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

package vPlugin

import (
	"database/sql"
	"database/sql/driver"
	"fmt"
)

type SensWordType string

const (
	GeneralVocabulary   SensWordType = "0" //通用词库
	SensitiveVocabulary SensWordType = "1" //敏感词库
	XhsVocabulary       SensWordType = "2" //小红书词
	AdvertVocabulary    SensWordType = "3" //广告词
	MedicalVocabulary   SensWordType = "4" //医疗词
	BiLiBiLiVocabulary  SensWordType = "5" //B站词
)

// 缓存key
const LingKeCookie = "Ling_Ke_Login_Cookie"

func (p SensWordType) String() string {
	switch p {
	case GeneralVocabulary:
		return "通用词库"
	case SensitiveVocabulary:
		return "敏感词库"
	case AdvertVocabulary:
		return "小红书词"
	case MedicalVocabulary:
		return "广告词"
	case BiLiBiLiVocabulary:
		return "医疗词"
	case XhsVocabulary:
		return "B站词"
	}
	return "<UNSET>"
}

func TableDataTypeFromString(s string) (SensWordType, error) {
	switch s {
	case "通用词库":
		return GeneralVocabulary, nil
	case "敏感词库":
		return SensitiveVocabulary, nil
	case "小红书词":
		return AdvertVocabulary, nil
	case "广告词":
		return MedicalVocabulary, nil
	case "医疗词":
		return BiLiBiLiVocabulary, nil
	case "B站词":
		return XhsVocabulary, nil
	}
	return SensWordType(0), fmt.Errorf("not a valid TableDataType string")
}

func TableDataTypePtr(v SensWordType) *SensWordType { return &v }
func (p *SensWordType) Scan(value interface{}) (err error) {
	var result sql.NullString
	err = result.Scan(value)
	if result.Valid {
		*p = SensWordType(result.String) // 数据库值 → string → SensWordType
	} else {
		*p = "" // 如果是 NULL，就赋值为空字符串
	}
	return nil

}

func (p *SensWordType) Value() (driver.Value, error) {
	if p == nil {
		return nil, nil
	}
	return string(*p), nil
}

type WordInfo struct {
	Words         string `json:"words"`
	WordsType     int    `json:"words_type"`
	WordsTypeName string `json:"words_type_name"`
}

type LocalCheck struct {
	IsPass int        `json:"is_pass"`
	Target []string   `json:"target"`
	Words  []WordInfo `json:"words"`
}

type BaiduStatus struct {
	ErrorCode int    `json:"error_code"`
	ErrorMsg  string `json:"error_msg"`
}

type BaiduCheck struct {
	IsPass int           `json:"is_pass"`
	Target []interface{} `json:"target"`
	Words  []interface{} `json:"words"`
}

type ResponseData struct {
	LocalCheck  LocalCheck  `json:"local_check"`
	BaiduStatus BaiduStatus `json:"baidu_status"`
	BaiduCheck  BaiduCheck  `json:"baidu_check"`
}

type ApiResponse struct {
	Data  ResponseData `json:"data"`
	Error int          `json:"error"`
	Msg   string       `json:"msg"`
}

type CheckText struct {
	WordsType []string `json:"words_type"`
	CheckText string   `json:"check_text"`
}
type RespSensWords struct {
	Word      string `json:"word"`
	CheckType string `json:"check_type"`
}
