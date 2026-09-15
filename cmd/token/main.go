package main

import (
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/avtofast/avtofast-back/internal/config"
	"github.com/avtofast/avtofast-back/pkg/jwt"
)

func main() {
	subFlag := flag.String("sub", "usr_dev_test_01", "User subject ID (provider unique ID)")
	emailFlag := flag.String("email", "dev@avtofast.uz", "User email")
	roleFlag := flag.String("role", "user", "User role ('user' or 'content_publisher')")
	daysFlag := flag.Int("days", 30, "Token validity in days")
	secretFlag := flag.String("secret", "", "Custom JWT secret (defaults to .env JWT_SECRET)")
	flag.Parse()

	cfg, _ := config.Load(".env")
	secret := *secretFlag
	if secret == "" {
		if cfg != nil && cfg.JWT.SecretKey != "" {
			secret = cfg.JWT.SecretKey
		} else if envSecret := os.Getenv("JWT_SECRET"); envSecret != "" {
			secret = envSecret
		} else {
			secret = "avtofast-super-secure-production-secret-key-32b"
		}
	}

	duration := time.Duration(*daysFlag) * 24 * time.Hour
	token, err := jwt.GenerateTestToken(*subFlag, *roleFlag, secret, duration, *emailFlag)
	if err != nil {
		fmt.Printf("Error generating token: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("==================================================================")
	fmt.Println("             AvtoFast JWT Test Token Generator                    ")
	fmt.Println("==================================================================")
	fmt.Printf("Subject (sub) : %s\n", *subFlag)
	fmt.Printf("Email         : %s\n", *emailFlag)
	fmt.Printf("Role          : %s\n", *roleFlag)
	fmt.Printf("Expires In    : %d kun (%s)\n", *daysFlag, time.Now().Add(duration).Format("2006-01-02 15:04:05"))
	fmt.Println("------------------------------------------------------------------")
	fmt.Println("YOUR TOKEN:")
	fmt.Println(token)
	fmt.Println("------------------------------------------------------------------")
	fmt.Println("\nQuyidagi curl buyruqlari orqali darhol test qilib ko'rishingiz mumkin:")
	fmt.Println("\n1. Profilni olish (GET /v1/me):")
	fmt.Printf("curl -s -H \"Authorization: Bearer %s\" https://api.avtofast.uz/v1/me | jq\n", token)
	fmt.Println("\n2. Savollar to'plamlarini ko'rish (GET /v1/content/packs):")
	fmt.Printf("curl -s -H \"Authorization: Bearer %s\" https://api.avtofast.uz/v1/content/packs | jq\n", token)
	fmt.Println("\n3. Savollar ro'yxatini olish (GET /v1/content/packs/uz-theory-2026-09/questions):")
	fmt.Printf("curl -s -H \"Authorization: Bearer %s\" \"https://api.avtofast.uz/v1/content/packs/uz-theory-2026-09/questions?limit=5\" | jq\n", token)
	fmt.Println("\n4. Swagger UI orqali test qilish:")
	fmt.Println("Browserda https://api.avtofast.uz/docs sahifasini oching, 'Authorize' tugmasini bosing va ushbu tokenni qo'ying.")
	fmt.Println("==================================================================")
}
