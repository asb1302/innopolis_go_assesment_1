package types

import (
	"log"
	"os"
)

type Message struct {
	Token  string
	FileID string
	Data   string
}

type User struct {
	Token  string
	FileID string
}

type AppInterface interface {
	AddUser(User) error
	SendMsg(Message)
}

type FileWriter interface {
	WriteToFile(filePath string, messages []Message) error
}

type DefaultFileWriter struct{}

func (w *DefaultFileWriter) WriteToFile(filePath string, messages []Message) error {
	log.Printf("запись в файл: %s", filePath)
	file, err := os.OpenFile(filePath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	defer file.Close()

	for _, msg := range messages {
		if _, err := file.WriteString(msg.Data + "\n"); err != nil {
			return err
		}
	}

	return nil
}
