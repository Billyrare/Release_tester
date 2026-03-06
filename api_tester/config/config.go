package config

import (
	"log"
	"os"

	"github.com/spf13/viper"
)

type Config struct {
	AppEnv      string `mapstructure:"APP_ENV"`
	ServerPort  string `mapstructure:"SERVER_PORT"`
	AslApiURL   string `mapstructure:"ASL_API_URL"`
	AslApiToken string `mapstructure:"ASL_API_TOKEN"`
}

func LoadConfig() (*Config, error) {
	viper.AddConfigPath(".")
	viper.SetConfigName(".env")
	viper.SetConfigType("env")

	viper.AutomaticEnv()
	log.Println("--- DEBUG: Попытка прочитать .env файл ---") // Временный лог
	// if err := viper.ReadInConfig(); err != nil {
	// 	if _, ok := err.(viper.ConfigFileNotFoundError); ok {
	// 		log.Println("Файл конфигурации не найнед, используем переменные окружения")
	// 	} else {
	// 		return nil, err
	// 	}

	// }
	if err := viper.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); ok {
			log.Println("--- DEBUG: .env файл не найден (как ConfigFileNotFoundError), используем переменные окружения. ---") // Временный лог
		} else {
			log.Printf("--- DEBUG: Другая ошибка при чтении .env файла: %v ---", err) // Временный лог
			return nil, err                                                           // Другая ошибка при чтении .env файла
		}
	} else {
		log.Println("--- DEBUG: .env файл успешно прочитан. ---") // Временный лог
		// Дополнительный лог для проверки значений, прочитанных viper-ом
		log.Printf("--- DEBUG: SERVER_PORT из Viper: %s ---", viper.GetString("SERVER_PORT"))
		log.Printf("--- DEBUG: ASL_API_URL из Viper: %s ---", viper.GetString("ASL_API_URL"))
		log.Printf("--- DEBUG: ASL_API_TOKEN из Viper: %s ---", viper.GetString("ASL_API_TOKEN"))
	}
	log.Println("--- DEBUG: Попытка Unmarshal в структуру Config ---") // Временный лог

	var cfg Config
	if err := viper.Unmarshal(&cfg); err != nil {
		log.Printf("--- DEBUG: Ошибка при Unmarshal: %v ---", err) // Временный лог
		return nil, err
	}
	log.Printf("--- DEBUG: Unmarshal успешен. Значения в cfg: %+v ---", cfg) // Временный лог (этот был до)
	log.Printf("--- DEBUG: ASL_API_TOKEN из конфига (с кавычками): \"%s\" ---", cfg.AslApiToken)

	if cfg.ServerPort == "" {
		cfg.ServerPort = "8080"
		log.Println(("SERVER_PORT не указан, используем порт по умолчанию: 8080"))
		log.Printf("DEBUG: ASL_API_TOKEN из конфига (с кавычками): \"%s\"", cfg.AslApiToken)
	}

	if cfg.AslApiURL == "" {
		return nil, os.ErrNotExist
	}

	if cfg.AslApiToken == "" {
		return nil, os.ErrNotExist
	}

	if cfg.AppEnv == "" {
		cfg.AppEnv = "development"
		log.Println("APP_ENV не указан, используем режим разработки")
	}

	return &cfg, nil
}
