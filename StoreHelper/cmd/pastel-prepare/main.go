package main

import (
	"context"
	"fmt"
	"github.com/majd/ipatool/v2/pkg/appstore"
	"os"
	"time"
)

func main() {
	os.Setenv("PASTEL_PROGRESS", "1")
	fmt.Println("正在准备并校验 Apple 系统文件；此过程不需要 Apple 账户或密码。")
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
	defer cancel()
	if err := appstore.Prepare(ctx); err != nil {
		fmt.Fprintln(os.Stderr, "准备失败:", err)
		os.Exit(1)
	}
	fmt.Println("Apple 系统文件已准备并校验完成。现在可以回到 Pastel 登录。")
}
