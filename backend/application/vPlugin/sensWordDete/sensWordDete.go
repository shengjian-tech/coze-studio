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

package sensWordDete

import (
	"bytes"
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/coze-dev/coze-studio/backend/api/model/vPlugin"
	apVp "github.com/coze-dev/coze-studio/backend/application/vPlugin"
	"github.com/coze-dev/coze-studio/backend/pkg/logs"
	"github.com/redis/go-redis/v9"
	"io"
	"log"
	"math"
	"net/http"
	"strings"
	"time"
)

type SensWordDeteRequest struct {
	Content  string                 `thrift:"content,1" json:"content" `
	SensType []vPlugin.SensWordType `thrift:"sens_word_type,2" json:"sens_word_type"`
}
type SensWordDeteRespone struct {
	Code int                     `thrift:"code,1" json:"code,required"`
	Msg  string                  `thrift:"msg,2" json:"msg,required"`
	Data []vPlugin.RespSensWords `thrift:"data,3" json:"data"`
}

type SensWordApplicationService struct {
	// DomainSVC service.vPlugin
	//storage storage.Storage
	baseService *apVp.BaseVpluginService
}

var SensWordApplicationSVC = &SensWordApplicationService{
	baseService: apVp.Bs,
}

//***********************function**************************//

func (s *SensWordApplicationService) DeteSensWord(ctx context.Context, req *SensWordDeteRequest) (*SensWordDeteRespone, error) {
	content := req.Content
	resp := &SensWordDeteRespone{
		Code: 500,
		Data: make([]vPlugin.RespSensWords, 0),
	}

	if content == "" || len(strings.TrimSpace(content)) == 0 {
		resp.Msg = "content is null"
		return resp, errors.New("content is null")
	}
	//获取登录cookie缓存
	cookie, err2 := s.baseService.Cache.Get(ctx, vPlugin.LingKeCookie).Bytes()
	if err2 != nil && err2 != redis.Nil {
		resp.Msg = "获取cookie失败"
		logs.CtxErrorf(ctx, "get cookie %v", err2)
		return resp, err2
	}
	var cookies []*http.Cookie
	if len(cookie) > 0 {

		json.Unmarshal([]byte(cookie), &cookies)
	}
	//敏感词检测,默认为全类型检测，目前只支持B站和小红书
	checkText := &vPlugin.CheckText{ //0---通用词库，1---敏感词，2----小红书，3---广告词，4----医疗词，5----B站
		WordsType: []string{"0", "1", "2", "3", "4"},
		CheckText: content,
	}
	loop := 0
	apiResp, err := sensDeteWords(ctx, checkText, &loop, cookies) //防止递归一直调用，最大不能超过10次调用（可能会存在登录失败，超时的情况）,记得优化，尽量和cookie解耦
	//检测成功
	if err != nil || loop > 10 {
		resp.Msg = "登录次数过多，请稍后重试"
		return resp, err
	}

	fmt.Printf("结果：----%v", apiResp)
	baiduCheck := apiResp.Data.BaiduCheck
	if len(baiduCheck.Words) > 0 {
		//更新data
	}
	sensWords := apiResp.Data.LocalCheck.Words
	newData := make([]vPlugin.RespSensWords, len(sensWords))
	if len(sensWords) > 0 {
		for i, sw := range sensWords {
			newData[i] = vPlugin.RespSensWords{
				Word:      sw.Words,
				CheckType: sw.WordsTypeName,
			}

		}
		resp.Data = append(resp.Data, newData...)
	}

	resp.Code = 0
	resp.Msg = "ok"
	return resp, nil
}

func sensDeteWords(ctx context.Context, checkText *vPlugin.CheckText, loop *int, cookies []*http.Cookie) (vPlugin.ApiResponse, error) {

	ids := "1174016"

	// 测试getAesKey函数
	fmt.Printf("AES Key for %s: %s (length: %d)\n", ids, getAesKey(ids), len(getAesKey(ids)))
	var apiResp vPlugin.ApiResponse
	// 示例数据
	//testData := map[string]interface{}{
	//	"message":   "标题：宝子们，HashKey Exchange 带你玩转数字资产！🚀 正文： 宝子们👋，今天必须给大家安利一波 HashKey Exchange！作为持牌虚拟资产交易所，它简直就是数字资产界的 “安全卫士”➕“效率王者”！💪 🔍 为什么选择 HashKey Exchange？ 持牌合规，安全第一 在数字资产的世界里，安全永远是第一位！HashKey Exchange 拥有正规牌照，严格遵循监管要求，让你的每一笔交易都稳稳当当！🔒 高效便捷，操作丝滑 从充值到交易，再到提现，全程流畅无卡顿！再也不用担心 “卡成 PPT” 的尴尬场面了！🚀 专业团队，保驾护航 HashKey Group 的首席分析师丁肇飞先生曾提到，像 HashKey Exchange 这样的持牌交易所，能够协助上市公司安全、高效地建立数字资产配置。专业团队为你提供全方位的支持，小白也能轻松上手！👨‍💼 🌟 用户真实体验 “第一次用 HashKey Exchange，简直爽到飞起！界面简洁明了，操作简单，最重要的是 —— 安全可靠！再也不用提心吊胆了！” —— 来自某位忠实用户的心声💖 🎯 适合谁？ 想尝试数字资产但担心安全问题的宝子们 追求高效交易体验的资深玩家 企业用户，需要合规的数字资产配置方案 💬 互动时间 宝子们，你们对数字资产交易最关心的是什么？是安全性、便捷性，还是收益？快来评论区聊聊吧！👇",
	//	"timestamp": 1234567890,
	//	"user_id":   1174016,
	//}

	// 加密
	encrypted := getParam(ids, checkText)
	fmt.Printf("Encrypted: %s\n", encrypted["access"])

	// 解密
	decrypted := getData(ids, encrypted["access"])
	fmt.Printf("Decrypted: %+v\n", decrypted)

	// 验证加密解密的一致性
	originalJson, _ := json.Marshal(checkText)
	decryptedJson, _ := json.Marshal(decrypted)
	fmt.Printf("Original:  %s\n", string(originalJson))
	fmt.Printf("Decrypted: %s\n", string(decryptedJson))

	//请求接口
	reqBody, errjson := json.Marshal(encrypted)
	if errjson != nil {
		fmt.Printf("序列化失败 %v", errjson)
		return apiResp, errjson
	}

	req, err := http.NewRequest("POST", "https://www.lingkechaci.com/index/index/check_text.html", bytes.NewReader(reqBody))
	if err != nil {
		fmt.Printf("构建请求失败 %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("user-agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/140.0.0.0 Safari/537.36 Edg/140.0.0.0")
	//req.Header.Set("cookie", "PHPSESSID=chp4dvdm40idt54p79udvb9da4; Hm_lvt_8b547c9f15a7c8c628ad7b9f236921ec=1757901508; HMACCOUNT=A819273EF298FB39; Hm_lpvt_8b547c9f15a7c8c628ad7b9f236921ec=1757904509")
	for _, c := range cookies {
		req.AddCookie(c)
	}
	client := &http.Client{
		Timeout: 30 * time.Second,
	}

	resp, reError := client.Do(req)

	if reError != nil {
		return apiResp, reError
	}
	defer func(Body io.ReadCloser) {
		err := Body.Close()
		if err != nil {

		}
	}(resp.Body)
	var respMap map[string]interface{}
	body, _ := io.ReadAll(resp.Body)
	fmt.Printf("通用函数请求结果: %s\n", string(body))
	erru := json.Unmarshal(body, &respMap)
	if erru != nil {
		fmt.Printf("出错啦：----%v", erru)
		return apiResp, erru
	}
	if msg, ok := respMap["msg"].(string); ok && (strings.Contains(msg, "需要登录") || strings.Contains(msg, "过期")) {
		//登录
		bol, newCookies, errLogin := SensWordApplicationSVC.loginingKeChaCi(ctx)
		if errLogin != nil {
			return apiResp, errLogin
		}
		if bol && len(newCookies) > 0 && *loop < 10 {
			*loop++
			words, err := sensDeteWords(ctx, checkText, loop, newCookies)
			if err != nil {
				return vPlugin.ApiResponse{}, err
			}
			return words, nil
		}
	}
	data := getData(ids, respMap["token"].(string))
	fmt.Printf("打他的值：-----%v \n", data)
	if respMap["token"] == "" {
		fmt.Printf("token不存在啦")
		return apiResp, fmt.Errorf("token不存在啦")
	}

	//var rs1 map[string]interface{}
	//m, nok := data.(map[string]interface{}) //local_check,baidu_status,baidu_check

	//s := fmt.Sprint(m["local_check"])
	// 假设 data 是 map[string]interface{}，先序列化成 JSON 再反序列化
	b, _ := json.Marshal(data)

	json.Unmarshal(b, &apiResp)

	fmt.Println("LocalCheck:", apiResp.Data.LocalCheck)
	fmt.Println("BaiduStatus:", apiResp.Data.BaiduStatus)
	*loop++
	return apiResp, nil

}

/*
*
领克查词登录
*/
func (s *SensWordApplicationService) loginingKeChaCi(ctx context.Context) (bool, []*http.Cookie, error) {
	body := map[string]interface{}{
		"access": "c18161d77dd4de664cbe1b10ec5610fcq8aPDlBneFBZjaf48gYRjU4CKohBVmK/EE/D+M1C6WuKQxXkHek1UG3+5CCXcYdd5Yt8+1zzHnGNqpnUEFzv3GLHb+qxdDm8E/fhNZ2/x4M=",
	}
	var c []*http.Cookie

	//jar, _ := cookiejar.New(nil)
	reqBody, err1 := json.Marshal(body)
	if err1 != nil {
		return false, c, err1
	}

	request, err := http.NewRequest("POST", "https://www.lingkechaci.com/index/index/user_login.html", bytes.NewReader(reqBody))
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("user-agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/140.0.0.0 Safari/537.36 Edg/140.0.0.0")

	if err != nil {
		return false, c, err
	}
	client := &http.Client{
		Timeout: 30 * time.Second,
		//Jar:     jar,
	}
	// 定义一个 map
	var result map[string]interface{}
	do, err2 := client.Do(request)
	if err2 != nil {
		return false, c, err2
	}
	body1, _ := io.ReadAll(do.Body)

	if errr := json.Unmarshal(body1, &result); err != nil {
		panic(errr)
		return false, c, errr
	}
	if rer, ok := result["error"].(int); ok && rer != 0 {
		return false, c, fmt.Errorf("登录失败：%s", result["error"])
	}
	//保存cookie
	cookies := do.Cookies()

	ckJson, err1 := json.Marshal(cookies)

	s.baseService.Cache.Set(ctx, vPlugin.LingKeCookie, ckJson, 168*time.Hour) //7天
	return true, cookies, nil
}

// *************************untils***************************//
// getAesKey 获取32位密钥 (对应JavaScript的getAesKey函数)
func getAesKey(userId string) string {
	// 转换为字符串
	userIdStr := userId + ""
	ret := ""

	// 重复userId直到接近32位
	for i := 0; i < int(math.Floor(32.0/float64(len(userIdStr)))); i++ {
		ret += userIdStr
	}

	// 补齐剩余位数
	remainder := 32 % len(userIdStr)
	if remainder > 0 {
		ret += userIdStr[:remainder]
	}

	return ret
}

// AES_Encrypt AES加密函数 (CBC模式，PKCS7填充)
func AES_Encrypt(plaintext, key string, iv []byte) string {
	plaintextBytes := []byte(plaintext)
	keyBytes := []byte(key)

	// 创建cipher
	block, err := aes.NewCipher(keyBytes)
	if err != nil {
		log.Panicf("aes error: %v", err)
		return ""
	}

	// PKCS7填充
	plaintextBytes = pkcs7Padding(plaintextBytes, block.BlockSize())

	// CBC模式加密
	mode := cipher.NewCBCEncrypter(block, iv)
	ciphertext := make([]byte, len(plaintextBytes))
	mode.CryptBlocks(ciphertext, plaintextBytes)

	// 返回base64编码的结果 (对应CryptoJS.AES.encrypt().toString())
	return base64.StdEncoding.EncodeToString(ciphertext)
}

// AES_Decrypt AES解密函数 (CBC模式，PKCS7填充)
func AES_Decrypt(cipherStr, key, ivHex string) string {
	keyBytes := []byte(key)

	// 解析IV (从hex字符串)
	iv, err := hex.DecodeString(ivHex)
	if err != nil {
		log.Fatal(err)
	}

	// 解析密文 (从base64字符串)
	ciphertext, err := base64.StdEncoding.DecodeString(cipherStr)
	if err != nil {
		log.Fatal(err)
	}

	// 创建cipher
	block, err := aes.NewCipher(keyBytes)
	if err != nil {
		log.Fatal(err)
	}

	// CBC模式解密
	mode := cipher.NewCBCDecrypter(block, iv)
	plaintext := make([]byte, len(ciphertext))
	mode.CryptBlocks(plaintext, ciphertext)

	// 去除PKCS7填充
	plaintext = pkcs7UnPadding(plaintext)

	return string(plaintext)
}

// getParam 对应JavaScript的getParam函数
func getParam(ids string, data interface{}) map[string]string {
	jsonData, err := json.Marshal(data)
	if err != nil {
		log.Fatal(err)
	}

	return map[string]string{
		"access": getParamStr(ids, string(jsonData)),
	}
}

// getParamStr 对应JavaScript的getParamStr函数
func getParamStr(ids, data string) string {
	// 生成16字节随机IV
	iv := make([]byte, 16)
	if _, err := rand.Read(iv); err != nil {
		log.Fatal(err)
	}

	fmt.Println(getAesKey(ids)) // 对应console.log(getAesKey(ids))

	// IV转为hex字符串 + 加密结果
	return hex.EncodeToString(iv) + AES_Encrypt(data, getAesKey(ids), iv)
}

// getData 对应JavaScript的getData函数
func getData(ids string, data string) interface{} {
	dataStr := getDataStr(ids, data)
	var result interface{}
	err := json.Unmarshal([]byte(dataStr), &result)
	if err != nil {
		log.Fatal(err)
	}
	return result
}

// getDataStr 对应JavaScript的getDataStr函数
func getDataStr(ids, data string) string {
	if len(data) < 32 {
		log.Fatal("data length is less than 32")
	}

	ivHex := data[:32]     // 前32个字符是IV (hex编码)
	cipherStr := data[32:] // 后面是密文 (base64编码)

	return AES_Decrypt(cipherStr, getAesKey(ids), ivHex)
}

// PKCS7填充
func pkcs7Padding(data []byte, blockSize int) []byte {
	padding := blockSize - len(data)%blockSize
	padtext := make([]byte, padding)
	for i := range padtext {
		padtext[i] = byte(padding)
	}
	return append(data, padtext...)
}

// PKCS7去填充
func pkcs7UnPadding(data []byte) []byte {
	length := len(data)
	if length == 0 {
		return data
	}
	unpadding := int(data[length-1])
	if unpadding > length || unpadding == 0 {
		return data
	}
	return data[:(length - unpadding)]
}
