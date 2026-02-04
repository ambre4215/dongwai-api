package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"text/tabwriter"
	"time"

	"dongwai_backend/internal/config"
	"dongwai_backend/internal/model"
	"dongwai_backend/internal/pkg/utils"

	"github.com/google/uuid"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

// 定义全局数据库变量
var db *gorm.DB

func initDB() {
	// 加载配置 (自动读取 .env)
	config.LoadConfig()

	var err error
	dsn := config.AppConfig.DB_DSN
	db, err = gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		log.Fatalf("❌ 无法连接数据库: %v\n请检查 .env 文件配置是否正确", err)
	}
}

func main() {
	// 定义子命令
	addCmd := flag.NewFlagSet("add", flag.ExitOnError)
	listCmd := flag.NewFlagSet("list", flag.ExitOnError)
	pwdCmd := flag.NewFlagSet("pwd", flag.ExitOnError)
	delCmd := flag.NewFlagSet("del", flag.ExitOnError)

	// add 子命令参数
	addName := addCmd.String("u", "", "用户名 (必须)")
	addPass := addCmd.String("p", "", "密码 (必须)")
	addRole := addCmd.String("r", "admin", "角色 (可选: admin/editor)")

	// pwd 子命令参数
	pwdName := pwdCmd.String("u", "", "用户名 (必须)")
	pwdPass := pwdCmd.String("p", "", "新密码 (必须)")

	// del 子命令参数
	delName := delCmd.String("u", "", "要删除的用户名 (必须)")

	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	initDB()

	switch os.Args[1] {
	case "add":
		addCmd.Parse(os.Args[2:])
		if *addName == "" || *addPass == "" {
			fmt.Println("❌ 错误: 必须提供用户名 (-u) 和密码 (-p)")
			addCmd.PrintDefaults()
			os.Exit(1)
		}
		handleAdd(*addName, *addPass, *addRole)

	case "list":
		listCmd.Parse(os.Args[2:])
		handleList()

	case "pwd":
		pwdCmd.Parse(os.Args[2:])
		if *pwdName == "" || *pwdPass == "" {
			fmt.Println("❌ 错误: 必须提供用户名 (-u) 和新密码 (-p)")
			pwdCmd.PrintDefaults()
			os.Exit(1)
		}
		handleResetPwd(*pwdName, *pwdPass)

	case "del":
		delCmd.Parse(os.Args[2:])
		if *delName == "" {
			fmt.Println("❌ 错误: 必须提供用户名 (-u)")
			delCmd.PrintDefaults()
			os.Exit(1)
		}
		handleDelete(*delName)

	default:
		printUsage()
		os.Exit(1)
	}
}

func printUsage() {
	fmt.Println("🛠️  账号管理工具使用说明:")
	fmt.Println("  add   - 添加新用户 (例如: user-cli add -u admin -p 123456)")
	fmt.Println("  list  - 列出所有用户")
	fmt.Println("  pwd   - 重置用户密码 (例如: user-cli pwd -u admin -p newpass)")
	fmt.Println("  del   - 删除用户 (例如: user-cli del -u admin)")
}

// --- 处理函数 ---

func handleAdd(username, password, role string) {
	var count int64
	db.Model(&model.UserRole{}).Where("username = ?", username).Count(&count)
	if count > 0 {
		fmt.Printf("❌ 用户 '%s' 已存在\n", username)
		return
	}

	hashedPwd, _ := utils.HashPassword(password)
	newUser := model.UserRole{
		ID:        uuid.New().String(),
		Username:  username,
		Password:  hashedPwd,
		Role:      role,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	if err := db.Create(&newUser).Error; err != nil {
		log.Fatalf("创建失败: %v", err)
	}
	fmt.Printf("✅ 用户 '%s' 创建成功 (角色: %s)\n", username, role)
}

func handleList() {
	var users []model.UserRole
	db.Order("created_at desc").Find(&users)

	fmt.Println("\n📋 用户列表:")
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 3, ' ', 0)
	fmt.Fprintln(w, "ID\t用户名\t角色\t创建时间")
	fmt.Fprintln(w, "--\t---\t--\t----")
	for _, u := range users {
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\n", u.ID[:8]+"...", u.Username, u.Role, u.CreatedAt.Format("2006-01-02 15:04"))
	}
	w.Flush()
	fmt.Println("")
}

func handleResetPwd(username, newPass string) {
	hashedPwd, _ := utils.HashPassword(newPass)
	res := db.Model(&model.UserRole{}).Where("username = ?", username).Update("password", hashedPwd)
	if res.Error != nil {
		log.Fatalf("更新失败: %v", res.Error)
	}
	if res.RowsAffected == 0 {
		fmt.Printf("❌ 未找到用户 '%s'\n", username)
		return
	}
	fmt.Printf("✅ 用户 '%s' 密码已重置\n", username)
}

func handleDelete(username string) {
	res := db.Where("username = ?", username).Delete(&model.UserRole{})
	if res.Error != nil {
		log.Fatalf("删除失败: %v", res.Error)
	}
	if res.RowsAffected == 0 {
		fmt.Printf("❌ 未找到用户 '%s'\n", username)
		return
	}
	fmt.Printf("🗑️  用户 '%s' 已删除\n", username)
}
