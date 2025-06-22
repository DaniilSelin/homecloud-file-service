package main

import (
	"context"
	"fmt"
	"log"
	"time"

	pb "homecloud-file-service/internal/transport/grpc/protos"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

func main() {
	// Подключаемся к gRPC серверу
	conn, err := grpc.Dial("localhost:9090", grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		log.Fatalf("Failed to connect: %v", err)
	}
	defer conn.Close()

	// Создаем клиент
	client := pb.NewFileServiceClient(conn)

	// Создаем контекст с таймаутом
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*10)
	defer cancel()

	// Тестируем создание директории для пользователя
	testUserID := "550e8400-e29b-41d4-a716-446655440000" // UUID для testuser

	fmt.Printf("Testing CreateUserDirectory for user: %s\n", testUserID)

	// Вызываем gRPC метод
	resp, err := client.CreateUserDirectory(ctx, &pb.CreateUserDirectoryRequest{
		UserId:   testUserID,
		Username: "testuser",
	})

	if err != nil {
		log.Fatalf("Failed to create user directory: %v", err)
	}

	// Выводим результат
	fmt.Printf("Success: %t\n", resp.Success)
	fmt.Printf("Message: %s\n", resp.Message)
	fmt.Printf("Directory Path: %s\n", resp.DirectoryPath)

	if resp.Success {
		fmt.Println("✅ User directory created successfully!")
	} else {
		fmt.Println("❌ Failed to create user directory")
	}
}
