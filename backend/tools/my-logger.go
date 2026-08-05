package tools

import (
	"encoding/json"
	"fmt"

	"github.com/gin-gonic/gin"
)

// PrettyPrint 美化打印任意结构体
func PrettyPrint(v any) {
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		fmt.Printf("print err: %v, raw: %+v\n", err, v)
		return
	}
	fmt.Println("==================== DEBUG ====================")
	fmt.Println(string(b))
	fmt.Println("===============================================")
}

// 使用
func TestHandler(c *gin.Context) {
	type Req struct {
		Name string `json:"name"`
		Age  int    `json:"age"`
	}
	var req Req
	_ = c.ShouldBindJSON(&req)
	PrettyPrint(req)              // 直接打印请求体
	PrettyPrint(c.Request.Header) // 打印请求头
}
