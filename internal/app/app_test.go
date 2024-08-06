package app

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/asb1302/innopolis_go_assesment_1/internal/config"
	"github.com/asb1302/innopolis_go_assesment_1/internal/handler"
	"github.com/asb1302/innopolis_go_assesment_1/internal/repository"
	"github.com/asb1302/innopolis_go_assesment_1/internal/types"
)

func setupConfig(filesDir string) *config.Config {
	return &config.Config{
		ValidTokens:    []string{"valid_token_1", "valid_token_2"},
		WorkerInterval: 1 * time.Second,
		FilesDir:       filesDir,
		NumWorkers:     1,
		MaxRetries:     3,
		RetryInterval:  2 * time.Second,
	}
}

func setup(filesDir string) (*App, *handler.MessageHandler) {
	cfg := setupConfig(filesDir)
	writer := &types.DefaultFileWriter{}
	userRepo := repository.NewUserRepository(cfg.ValidTokens)
	application := NewApp(cfg, writer, userRepo)
	msgHandler := handler.NewMessageHandler(userRepo, application, cfg)
	return application, msgHandler
}

func checkFile(t *testing.T, path string, expected []string) {
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		if len(expected) == 0 {
			return
		}
		t.Fatalf("Ожидалось, что файл %s существует", path)
	} else if err != nil {
		t.Fatalf("не удалось прочитать файл %s: %v", path, err)
	}

	lines := string(data)
	for _, exp := range expected {
		if !strings.Contains(lines, exp) {
			t.Fatalf("Ожидалось, что файл %s содержит %s", path, exp)
		}
	}
}

func generateExpectedData(n int) []string {
	var data []string
	for i := 0; i < n; i++ {
		data = append(data, fmt.Sprintf("data%d", i))
	}
	return data
}

func generateExpectedConcurrentData(numMessages int) []string {
	var expectedData []string
	for i := 0; i < numMessages; i++ {
		expectedData = append(expectedData, fmt.Sprintf("data1_%d", i))
		expectedData = append(expectedData, fmt.Sprintf("data2_%d", i))
	}
	return expectedData
}

// Проверяет корректную запись данных в файл.
func TestSuccessfulWrite(t *testing.T) {
	filesDir := filepath.Join("..", "..", "files", "TestSuccessfulWrite")
	if err := os.MkdirAll(filesDir, 0755); err != nil {
		t.Fatalf("не удалось создать папку для файлов: %v", err)
	}
	defer os.RemoveAll(filesDir)

	application, msgHandler := setup(filesDir)
	if err := application.AddUser(types.User{Token: "valid_token_1", FileID: "file1"}); err != nil {
		t.Fatalf("не удалось добавить пользователя: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go application.Start(ctx)

	for i := 0; i < 100; i++ {
		msgHandler.HandleMessage(types.Message{
			Token:  "valid_token_1",
			FileID: "file1",
			Data:   fmt.Sprintf("data%d", i),
		})
	}

	time.Sleep(3 * time.Second)

	application.Shutdown()

	checkFile(t, filepath.Join(filesDir, "file1.txt"), generateExpectedData(100))
}

// Проверяет обработку недействительных токенов.
func TestInvalidToken(t *testing.T) {
	filesDir := filepath.Join("..", "..", "files", "TestInvalidToken")
	if err := os.MkdirAll(filesDir, 0755); err != nil {
		t.Fatalf("не удалось создать папку для файлов: %v", err)
	}
	defer os.RemoveAll(filesDir)

	application, msgHandler := setup(filesDir)
	if err := application.AddUser(types.User{Token: "valid_token_1", FileID: "file1"}); err != nil {
		t.Fatalf("не удалось добавить пользователя: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go application.Start(ctx)

	msgHandler.HandleMessage(types.Message{
		Token:  "invalid_token",
		FileID: "file1",
		Data:   "data1",
	})

	time.Sleep(1 * time.Second)

	cancel()
	application.Shutdown()

	checkFile(t, filepath.Join(filesDir, "file1.txt"), []string{})
}

// Проверяет плавное завершение работы и сохранение данных перед завершением.
func TestGracefulShutdown(t *testing.T) {
	filesDir := filepath.Join("..", "..", "files", "TestGracefulShutdown")
	if err := os.MkdirAll(filesDir, 0755); err != nil {
		t.Fatalf("не удалось создать папку для файлов: %v", err)
	}
	defer os.RemoveAll(filesDir)

	application, msgHandler := setup(filesDir)
	if err := application.AddUser(types.User{Token: "valid_token_1", FileID: "file1"}); err != nil {
		t.Fatalf("не удалось добавить пользователя: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go application.Start(ctx)

	for i := 0; i < 50; i++ {
		msgHandler.HandleMessage(types.Message{
			Token:  "valid_token_1",
			FileID: "file1",
			Data:   fmt.Sprintf("data%d", i),
		})
	}

	time.Sleep(1 * time.Second)

	application.Shutdown()

	checkFile(t, filepath.Join(filesDir, "file1.txt"), generateExpectedData(50))
}

// Проверяет конкурентную запись данных в один и тот же файл несколькими пользователями.
func TestConcurrentFileWrites(t *testing.T) {
	filesDir := filepath.Join("..", "..", "files", "TestConcurrentFileWrites")
	if err := os.MkdirAll(filesDir, 0755); err != nil {
		t.Fatalf("не удалось создать папку для файлов: %v", err)
	}
	defer os.RemoveAll(filesDir)

	cfg := &config.Config{
		ValidTokens:    []string{"valid_token_1", "valid_token_2"},
		WorkerInterval: 1 * time.Second,
		FilesDir:       filesDir,
		NumWorkers:     10,
		MaxRetries:     3,
		RetryInterval:  2 * time.Second,
	}

	writer := &types.DefaultFileWriter{}
	userRepo := repository.NewUserRepository(cfg.ValidTokens)
	application := NewApp(cfg, writer, userRepo)

	// Добавляем двух пользователей, которые будут писать в один и тот же файл
	if err := application.AddUser(types.User{Token: "valid_token_1", FileID: "file1"}); err != nil {
		t.Fatalf("не удалось добавить пользователя: %v", err)
	}
	if err := application.AddUser(types.User{Token: "valid_token_2", FileID: "file1"}); err != nil {
		t.Fatalf("не удалось добавить пользователя: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go application.Start(ctx)

	var wg sync.WaitGroup
	numMessages := 5000

	// Запускаем несколько горутин для отправки сообщений
	for i := 0; i < numMessages; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()

			// Отправляем сообщение от первого пользователя
			msg1 := types.Message{
				Token:  "valid_token_1",
				FileID: "file1",
				Data:   fmt.Sprintf("data1_%d", i),
			}
			application.SendMsg(msg1)

			// Отправляем сообщение от второго пользователя
			msg2 := types.Message{
				Token:  "valid_token_2",
				FileID: "file1",
				Data:   fmt.Sprintf("data2_%d", i),
			}
			application.SendMsg(msg2)
		}(i)
	}
	wg.Wait()

	time.Sleep(5 * time.Second)

	application.Shutdown()

	checkFile(t, filepath.Join(filesDir, "file1.txt"), generateExpectedConcurrentData(numMessages))
}

// Проверяет работу приложения при высокой нагрузке.
func TestHighLoad(t *testing.T) {
	filesDir := filepath.Join("..", "..", "files", "TestHighLoad")
	if err := os.MkdirAll(filesDir, 0755); err != nil {
		t.Fatalf("не удалось создать папку для файлов: %v", err)
	}
	defer os.RemoveAll(filesDir)

	cfg := setupConfig(filesDir)
	writer := &types.DefaultFileWriter{}
	userRepo := repository.NewUserRepository(cfg.ValidTokens)
	application := NewApp(cfg, writer, userRepo)
	user := types.User{
		Token:  "valid_token_1",
		FileID: "file1",
	}
	if err := application.AddUser(user); err != nil {
		t.Fatalf("не удалось добавить пользователя: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go application.Start(ctx)

	var wg sync.WaitGroup
	for i := 0; i < 10000; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			application.SendMsg(types.Message{
				Token:  "valid_token_1",
				FileID: "file1",
				Data:   fmt.Sprintf("data%d", i),
			})
		}(i)
	}
	wg.Wait()

	time.Sleep(5 * time.Second)

	application.Shutdown()

	// Проверка увеличения количества воркеров
	if application.GetWorkerCount("file1") <= 10 {
		t.Errorf("Ожидалось увеличение количества воркеров, но их количество осталось: %d", application.GetWorkerCount("file1"))
	}

	checkFile(t, filepath.Join(filesDir, "file1.txt"), generateExpectedData(10000))
}

type MockFileWriter struct {
	failCount int
	maxFails  int
}

func (m *MockFileWriter) WriteToFile(filePath string, messages []types.Message) error {
	// Как только количество ошибок достигает значения maxFails, метод начинает успешно записывать данные, используя DefaultFileWriter.
	if m.failCount < m.maxFails {
		m.failCount++
		return fmt.Errorf("симулированная ошибка записи")
	}
	return (&types.DefaultFileWriter{}).WriteToFile(filePath, messages)
}

// Проверяет повторные попытки записи воркером.
func TestRetriesWithPartialFailures(t *testing.T) {
	filesDir := filepath.Join("..", "..", "files", "TestRetriesWithPartialFailures")
	if err := os.MkdirAll(filesDir, 0755); err != nil {
		t.Fatalf("не удалось создать папку для файлов: %v", err)
	}
	defer os.RemoveAll(filesDir)

	// Создаем MockFileWriter, который будет симулировать две неудачные попытки записи
	writer := &MockFileWriter{maxFails: 2}

	cfg := setupConfig(filesDir)
	userRepo := repository.NewUserRepository(cfg.ValidTokens)
	application := NewApp(cfg, writer, userRepo)
	if err := application.AddUser(types.User{Token: "valid_token_1", FileID: "file1"}); err != nil {
		t.Fatalf("не удалось добавить пользователя: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go application.Start(ctx)

	msgHandler := handler.NewMessageHandler(userRepo, application, cfg)
	msgHandler.HandleMessage(types.Message{
		Token:  "valid_token_1",
		FileID: "file1",
		Data:   "data1",
	})

	// Ждем, чтобы дать время на ретраи
	time.Sleep(3 * time.Second)

	application.Shutdown()

	checkFile(t, filepath.Join(filesDir, "file1.txt"), []string{"data1"})
}

// Проверяет, что запись прерывается после достижения MaxRetries.
func TestExceedingMaxRetriesStopsRetrying(t *testing.T) {
	filesDir := filepath.Join("..", "..", "files", "TestExceedingMaxRetriesStopsRetrying")
	if err := os.MkdirAll(filesDir, 0755); err != nil {
		t.Fatalf("не удалось создать папку для файлов: %v", err)
	}
	defer os.RemoveAll(filesDir)

	// MockFileWriter всегда возвращает ошибку
	writer := &MockFileWriter{maxFails: 100}

	cfg := setupConfig(filesDir)
	cfg.MaxRetries = 3 // Устанавливаем MaxRetries на 3
	userRepo := repository.NewUserRepository(cfg.ValidTokens)
	application := NewApp(cfg, writer, userRepo)
	if err := application.AddUser(types.User{Token: "valid_token_1", FileID: "file1"}); err != nil {
		t.Fatalf("не удалось добавить пользователя: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go application.Start(ctx)

	msgHandler := handler.NewMessageHandler(userRepo, application, cfg)
	msgHandler.HandleMessage(types.Message{
		Token:  "valid_token_1",
		FileID: "file1",
		Data:   "data1",
	})

	time.Sleep(5 * time.Second) // Ждем, чтобы дать время на все ретраи

	application.Shutdown()

	// Проверяем, что файл не был создан, так как все попытки записи завершились неудачно
	if _, err := os.Stat(filepath.Join(filesDir, "file1.txt")); !os.IsNotExist(err) {
		t.Fatalf("Файл не должен существовать, так как все попытки записи завершились неудачно")
	}
}

type SlowFileWriter struct {
	Delay time.Duration
}

func (s *SlowFileWriter) WriteToFile(filePath string, messages []types.Message) error {
	time.Sleep(s.Delay)

	return (&types.DefaultFileWriter{}).WriteToFile(filePath, messages)
}

// Проверяет, что новые сообщения, добавленные во время записи, не теряются.
func TestNewMessagesDuringWrite(t *testing.T) {
	filesDir := filepath.Join("..", "..", "files", "TestNewMessagesDuringWrite")
	if err := os.MkdirAll(filesDir, 0755); err != nil {
		t.Fatalf("Не удалось создать директорию для файлов: %v", err)
	}
	defer os.RemoveAll(filesDir)

	// Используем SlowFileWriter для имитирования задержки записи
	writer := &SlowFileWriter{Delay: 2 * time.Second}
	cfg := setupConfig(filesDir)
	userRepo := repository.NewUserRepository(cfg.ValidTokens)
	application := NewApp(cfg, writer, userRepo)
	if err := application.AddUser(types.User{Token: "valid_token_1", FileID: "file1"}); err != nil {
		t.Fatalf("Не удалось добавить пользователя: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go application.Start(ctx)

	msgHandler := handler.NewMessageHandler(userRepo, application, cfg)

	// Добавляем начальные сообщения
	for i := 0; i < 5; i++ {
		msgHandler.HandleMessage(types.Message{
			Token:  "valid_token_1",
			FileID: "file1",
			Data:   fmt.Sprintf("data%d", i),
		})
	}

	// Запуск процессора кеша в отдельной горутине
	go func() {
		time.Sleep(1 * time.Second)

		application.processCache() // чтобы симулировать сценарий, когда кэш обрабатывается и записывается в файл в то время, как новые сообщения продолжают поступать
	}()

	// Добавляем новые сообщения после начала записи
	time.Sleep(1 * time.Second)
	for i := 5; i < 10; i++ {
		msgHandler.HandleMessage(types.Message{
			Token:  "valid_token_1",
			FileID: "file1",
			Data:   fmt.Sprintf("data%d", i),
		})
	}

	time.Sleep(3 * time.Second)
	application.Shutdown()

	checkFile(t, filepath.Join(filesDir, "file1.txt"), generateExpectedData(10))
}
