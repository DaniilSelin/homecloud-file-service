package config

import (
	"fmt"
	"os"

	"go.uber.org/zap"
	"gopkg.in/yaml.v2"
)

// LoggerConfig - конфигурация логгера
type LoggerConfig struct {
	zap.Config `yaml:",inline"`
}

func (lc *LoggerConfig) Build() (*zap.Logger, error) {
	return lc.Config.Build()
}

// ServerConfig - конфигурация HTTP сервера
type ServerConfig struct {
	Host string `yaml:"host"`
	Port int    `yaml:"port"`
}

// DatabaseConfig - конфигурация базы данных
type DatabaseConfig struct {
	Host     string `yaml:"host"`
	Port     int    `yaml:"port"`
	User     string `yaml:"user"`
	Password string `yaml:"password"`
	DBName   string `yaml:"dbname"`
	SSLMode  string `yaml:"sslmode"`
}

// StorageConfig - конфигурация файлового хранилища
type StorageConfig struct {
	BasePath    string `yaml:"base_path"`     // Базовая директория для хранения файлов
	MaxSize     int64  `yaml:"max_size"`      // Максимальный размер файла в байтах
	ChunkSize   int64  `yaml:"chunk_size"`    // Размер чанка для возобновляемых загрузок
	TempPath    string `yaml:"temp_path"`     // Временная директория для загрузок
	UserDirName string `yaml:"user_dir_name"` // Имя директории для пользователей (по умолчанию "users")
}

// GrpcConfig - конфигурация gRPC клиента для БД
type GrpcConfig struct {
	Host string `yaml:"host"`
	Port int    `yaml:"port"`
}

// DbManagerConfig - конфигурация gRPC клиента для DBManager
type DbManagerConfig struct {
	Host string `yaml:"host"`
	Port int    `yaml:"port"`
}

// AuthConfig - конфигурация auth сервиса
type AuthConfig struct {
	Host string `yaml:"host"`
	Port int    `yaml:"port"`
}

// Config - основная конфигурация приложения
type Config struct {
	Server    ServerConfig    `yaml:"server"`
	Database  DatabaseConfig  `yaml:"database"`
	Storage   StorageConfig   `yaml:"storage"`
	Logger    LoggerConfig    `yaml:"logger"`
	Grpc      GrpcConfig      `yaml:"grpc"`
	DbManager DbManagerConfig `yaml:"dbmanager"`
	Auth      AuthConfig      `yaml:"auth"`
}

func LoadConfig(filename string) (*Config, error) {
	file, err := os.Open(filename)
	if err != nil {
		return nil, fmt.Errorf("could not open config file: %v", err)
	}
	defer file.Close()

	var config Config
	decoder := yaml.NewDecoder(file)
	err = decoder.Decode(&config)
	if err != nil {
		return nil, fmt.Errorf("could not decode config file: %v", err)
	}
	return &config, nil
}
