package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	"github.com/asb1302/innopolis_go_assesment_1/internal/app"
	"github.com/asb1302/innopolis_go_assesment_1/internal/config"
	"github.com/asb1302/innopolis_go_assesment_1/internal/handler"
	"github.com/asb1302/innopolis_go_assesment_1/internal/repository"
	"github.com/asb1302/innopolis_go_assesment_1/internal/types"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	cfg := config.LoadConfig()

	userRepo := repository.NewUserRepository(cfg.ValidTokens)
	writer := &types.DefaultFileWriter{}
	application := app.NewApp(cfg, writer, userRepo)
	msgHandler := handler.NewMessageHandler(userRepo, application, cfg)

	http.HandleFunc("/add-user", func(w http.ResponseWriter, r *http.Request) {
		token := r.URL.Query().Get("token")
		fileID := r.URL.Query().Get("fileID")

		if token == "" || fileID == "" {
			http.Error(w, "отсутствуют параметры", http.StatusBadRequest)
			return
		}

		log.Printf("Добавление пользователя: token=%s, fileID=%s", token, fileID)
		err := application.AddUser(types.User{
			Token:  token,
			FileID: fileID,
		})
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		w.Write([]byte("пользователь добавлен"))
	})

	http.HandleFunc("/add-message", func(w http.ResponseWriter, r *http.Request) {
		token := r.URL.Query().Get("token")
		fileID := r.URL.Query().Get("fileID")
		data := r.URL.Query().Get("data")

		if token == "" || fileID == "" || data == "" {
			http.Error(w, "отсутствуют параметры", http.StatusBadRequest)
			return
		}

		log.Printf("добавление сообщения: token=%s, fileID=%s, data=%s", token, fileID, data)

		if !userRepo.IsValidToken(token) {
			http.Error(w, "недействительный токен", http.StatusUnauthorized)
			return
		}

		user, exists := userRepo.GetUserByToken(token)
		if !exists {
			http.Error(w, "пользователь не найден", http.StatusUnauthorized)
			return
		}

		if user.FileID != fileID {
			http.Error(w, "неверный fileID файла для данного токена", http.StatusBadRequest)
			return
		}

		if _, exists := application.GetFileCh(fileID); !exists {
			http.Error(w, "канал для файла не существует", http.StatusBadRequest)
			return
		}

		msgHandler.HandleMessage(types.Message{
			Token:  token,
			FileID: fileID,
			Data:   data,
		})

		filePath := filepath.Join(cfg.FilesDir, fileID+".txt")
		absoluteFilePath, err := filepath.Abs(filePath)
		if err != nil {
			log.Fatalf("не удалось получить абсолютный путь для файла: %v", err)
		}
		log.Printf("cообщение добавлено и сохранено в файл: %s", absoluteFilePath)
		w.Write([]byte("сообщение добавлено"))
	})

	server := &http.Server{Addr: ":8080"}

	go func() {
		log.Println("сервер запущен на порту 8080")
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Не удалось запустить сервер: %s\n", err)
		}
	}()

	go application.Start(ctx)

	<-ctx.Done()

	log.Println("завершение работы сервера")
	if err := server.Shutdown(context.Background()); err != nil {
		log.Fatalf("Ошибка завершения работы сервера:%+v", err)
	}
	log.Println("сервер успешно завершил работу")
}
